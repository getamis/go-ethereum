// Copyright 2017 AMIS Technologies
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/istanbul"
)

// Start implements core.Engine.Start
func (c *core) Start(lastSequence *big.Int, lastProposer common.Address) error {
	// initial last proposer
	c.lastProposer = lastProposer

	// Start a new round from last sequence + 1
	c.startNewRound(&istanbul.View{
		Sequence: new(big.Int).Add(lastSequence, common.Big1),
		Round:    common.Big0,
	}, false)

	// Tests will handle events itself, so we have to make subscribeEvents()
	// be able to call in test.
	c.subscribeEvents()
	go c.handleEvents()

	return nil
}

// Stop implements core.Engine.Stop
func (c *core) Stop() error {
	c.unsubscribeEvents()
	return nil
}

// ----------------------------------------------------------------------------

// Subscribe both internal and external events
func (c *core) subscribeEvents() {
	c.events = c.backend.EventMux().Subscribe(
		// external events
		istanbul.RequestEvent{},
		istanbul.MessageEvent{},
		istanbul.FinalCommittedEvent{},
		// internal events
		backlogEvent{},
	)
}

// Unsubscribe all events
func (c *core) unsubscribeEvents() {
	c.events.Unsubscribe()
}

func (c *core) handleEvents() {
	for event := range c.events.Chan() {
		// A real event arrived, process interesting content
		switch ev := event.Data.(type) {
		case istanbul.RequestEvent:
			r := &istanbul.Request{
				Proposal: ev.Proposal,
			}
			err := c.handleRequest(r)
			if err == errFutureMessage {
				c.storeRequestMsg(r)
			}
		case istanbul.MessageEvent:
			c.handleMsg(ev.Payload)
		case istanbul.FinalCommittedEvent:
			c.handleFinalCommitted(ev.Proposal, ev.Proposer)
		case backlogEvent:
			// No need to check signature for internal messages
			c.handleCheckedMsg(ev.msg, ev.src)
		}
	}
}

// sendEvent sends internal events
func (c *core) sendEvent(ev interface{}) {
	c.backend.EventMux().Post(ev)
}

func (c *core) handleMsg(payload []byte) error {
	logger := c.logger.New("address", c.address)

	// Decode message and check its signature
	msg := new(message)
	if err := msg.FromPayload(payload, c.backend.CheckValidatorSignature); err != nil {
		logger.Error("Failed to decode message from payload", "err", err)
		return err
	}

	// Only accept message if address is valid
	_, src := c.backend.Validators().GetByAddress(msg.Address)
	if src == nil {
		logger.Error("Invalid address in message", "msg", msg)
		return istanbul.ErrUnauthorizedAddress
	}

	return c.handleCheckedMsg(msg, src)
}

func (c *core) handleCheckedMsg(msg *message, src istanbul.Validator) error {
	logger := c.logger.New("address", c.address, "from", src)

	// Store message if its a future message
	testBacklog := func(err error) error {
		if err == errFutureMessage {
			c.storeBacklog(msg, src)
			return nil
		}

		return err
	}

	switch msg.Code {
	case msgPreprepare:
		return testBacklog(c.handlePreprepare(msg, src))
	case msgPrepare:
		return testBacklog(c.handlePrepare(msg, src))
	case msgCommit:
		return testBacklog(c.handleCommit(msg, src))
	case msgRoundChange:
		return c.handleRoundChange(msg, src)
	default:
		logger.Error("Invalid message", "msg", msg)
	}

	return errInvalidMessage
}

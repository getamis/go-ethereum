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

	"github.com/ethereum/go-ethereum/consensus/pbft"
)

// Start implements core.Engine.Start
func (c *core) Start() error {
	// Tests will handle events itself, so we have to make subscribeEvents()
	// be able to call in test.
	c.subscribeEvents()

	go c.handleExternalEvent()
	go c.handleInternalEvent()

	return nil
}

// Stop implements core.Engine.Stop
func (c *core) Stop() error {
	c.unsubscribeEvents()
	return nil
}

// ----------------------------------------------------------------------------

func (c *core) subscribeEvents() {
	c.events = c.backend.EventMux().Subscribe(
		pbft.RequestEvent{},
		pbft.ConnectionEvent{},
		pbft.MessageEvent{},
		pbft.CheckpointEvent{},
	)

	c.internalEvents = c.internalMux.Subscribe(
		backlogEvent{},
		buildCheckpointEvent{},
	)
}

func (c *core) unsubscribeEvents() {
	c.events.Unsubscribe()
	c.internalEvents.Unsubscribe()
}

func (c *core) handleExternalEvent() {
	for event := range c.events.Chan() {
		// A real event arrived, process interesting content
		switch ev := event.Data.(type) {
		case pbft.CheckpointEvent:
			// TODO: we only implement sequence and digest now
			// TODO: might have to handle error
			c.sendCheckpoint(&pbft.Subject{
				View: &pbft.View{
					Sequence:   ev.BlockNumber,
					ViewNumber: new(big.Int),
				},
				Digest: ev.BlockHash.Bytes(),
			})
		case pbft.ConnectionEvent:

		case pbft.RequestEvent:
			c.handleRequest(&pbft.Request{
				BlockContext: ev.BlockContext,
			}, c.backend.Validators().GetByAddress(c.address))
		case pbft.MessageEvent:
			c.handleMsg(ev.Payload, c.backend.Validators().GetByAddress(ev.Address))
		}
	}
}

func (c *core) sendInternalEvent(ev interface{}) {
	c.internalMux.Post(ev)
}

func (c *core) handleInternalEvent() {
	for event := range c.internalEvents.Chan() {
		// A real event arrived, process interesting content
		switch ev := event.Data.(type) {
		case backlogEvent:
			c.handle(ev.msg, ev.src)
		case buildCheckpointEvent:
			go c.buildStableCheckpoint()
		}
	}
}

func (c *core) handleMsg(payload []byte, src pbft.Validator) error {
	logger := c.logger.New("address", c.address.Hex(), "from", src.Address().Hex())

	// Decode message
	msg := new(pbft.Message)
	if err := msg.FromPayload(payload, c.backend.CheckValidatorSignature); err != nil {
		logger.Error("Failed to decode message from payload", "error", err)
		return err
	}

	return c.handle(msg, src)
}

func (c *core) handle(msg *pbft.Message, src pbft.Validator) error {
	logger := c.logger.New("address", c.address.Hex(), "from", src.Address().Hex())

	testBacklog := func(err error) error {
		if err == errFutureMessage {
			c.storeBacklog(msg, src)
			return nil
		}

		return err
	}

	switch msg.Code {
	case pbft.MsgPreprepare:
		return testBacklog(c.handlePreprepare(msg, src))
	case pbft.MsgPrepare:
		return testBacklog(c.handlePrepare(msg, src))
	case pbft.MsgCommit:
		return testBacklog(c.handleCommit(msg, src))
	case pbft.MsgCheckpoint:
		return c.handleCheckpoint(msg, src)
	case pbft.MsgViewChange:
	case pbft.MsgNewView:
	default:
		logger.Error("Invalid message", "msg", msg)
	}

	return nil
}

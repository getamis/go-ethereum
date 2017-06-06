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
	"math"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/istanbul"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"gopkg.in/karalabe/cookiejar.v2/collections/prque"
)

const (
	keyStableCheckpoint = "StableCheckpoint"
)

func New(backend istanbul.Backend, config *istanbul.Config) Engine {
	// update n and f
	n := int64(backend.Validators().Size())
	f := int64(math.Ceil(float64(n)/3) - 1)

	return &core{
		config:            config,
		address:           backend.Address(),
		N:                 n,
		F:                 f,
		state:             StateAcceptRequest,
		logger:            log.New("address", backend.Address()),
		backend:           backend,
		backlogs:          make(map[istanbul.Validator]*prque.Prque),
		backlogsMu:        new(sync.Mutex),
		roundChangeSet:    newRoundChangeSet(backend.Validators()),
		pendingRequests:   prque.New(),
		pendingRequestsMu: new(sync.Mutex),
	}
}

// ----------------------------------------------------------------------------

type core struct {
	config  *istanbul.Config
	address common.Address
	N       int64
	F       int64
	state   State
	logger  log.Logger

	backend istanbul.Backend
	events  *event.TypeMuxSubscription

	lastProposer          common.Address
	waitingForRoundChange bool

	backlogs   map[istanbul.Validator]*prque.Prque
	backlogsMu *sync.Mutex

	current *snapshot

	roundChangeSet   *roundChangeSet
	roundChangeTimer *time.Timer

	pendingRequests   *prque.Prque
	pendingRequestsMu *sync.Mutex
}

func (c *core) finalizeMessage(msg *message) ([]byte, error) {
	// Add sender address
	msg.Address = c.Address()

	// Sign message
	data, err := msg.PayloadNoSig()
	if err != nil {
		return nil, err
	}
	msg.Signature, err = c.backend.Sign(data)
	if err != nil {
		return nil, err
	}

	// Convert to payload
	payload, err := msg.Payload()
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func (c *core) send(msg *message, target common.Address) {
	logger := c.logger.New("state", c.state)

	payload, err := c.finalizeMessage(msg)
	if err != nil {
		logger.Error("Failed to finalize message", "msg", msg, "err", err)
		return
	}

	// send payload
	if err = c.backend.Send(payload, target); err != nil {
		logger.Error("Failed to send message", "msg", msg, "err", err)
		return
	}
}

func (c *core) broadcast(msg *message) {
	logger := c.logger.New("state", c.state)

	payload, err := c.finalizeMessage(msg)
	if err != nil {
		logger.Error("Failed to finalize message", "msg", msg, "err", err)
		return
	}

	// Broadcast payload
	if err = c.backend.Broadcast(payload); err != nil {
		logger.Error("Failed to broadcast message", "msg", msg, "err", err)
		return
	}
}

func (c *core) currentView() *istanbul.View {
	return &istanbul.View{
		Sequence: new(big.Int).Set(c.current.Sequence()),
		Round:    new(big.Int).Set(c.current.Round()),
	}
}

func (c *core) nextRound() *istanbul.View {
	return &istanbul.View{
		Sequence: new(big.Int).Set(c.current.Sequence()),
		Round:    new(big.Int).Add(c.current.Round(), common.Big1),
	}
}

func (c *core) isPrimary() bool {
	v := c.backend.Validators()
	if v == nil {
		return false
	}
	return v.IsProposer(c.backend.Address())
}

func (c *core) commit() {
	c.setState(StateCommitted)

	proposal := c.current.Proposal()
	if proposal != nil {
		if err := c.backend.Commit(proposal); err != nil {
			c.sendRoundChange()
			return
		}
	}
}

func (c *core) startNewRound(newView *istanbul.View, roundChange bool) {
	var logger log.Logger
	if c.current == nil {
		logger = c.logger.New("old_round", -1, "old_seq", 0, "old_proposer", c.backend.Validators().GetProposer())
	} else {
		logger = c.logger.New("old_round", c.current.Round(), "old_seq", c.current.Sequence(), "old_proposer", c.backend.Validators().GetProposer())
	}

	// Clear invalid RoundChange messages
	c.roundChangeSet.Clear(newView)
	// New snapshot for new round
	c.current = newSnapshot(newView, c.backend.Validators())
	// Calculate new proposer
	c.backend.Validators().CalcProposer(c.proposerSeed())
	c.waitingForRoundChange = false
	c.setState(StateAcceptRequest)
	if roundChange && c.isPrimary() {
		c.backend.NextRound()
	}
	c.newRoundChangeTimer()

	logger.Debug("New round", "new_round", newView.Round, "new_seq", newView.Sequence, "new_proposer", c.backend.Validators().GetProposer())
}

func (c *core) catchUpRound(view *istanbul.View) {
	logger := c.logger.New("old_round", c.current.Round(), "old_seq", c.current.Sequence(), "old_proposer", c.backend.Validators().GetProposer())
	c.waitingForRoundChange = true
	c.current = newSnapshot(view, c.backend.Validators())
	c.newRoundChangeTimer()

	logger.Trace("Catch up round", "new_round", view.Round, "new_seq", view.Sequence, "new_proposer", c.backend.Validators().GetProposer())
}

func (c *core) proposerSeed() uint64 {
	emptyAddr := common.Address{}
	if c.lastProposer == emptyAddr {
		return c.current.Round().Uint64()
	}
	offset := 0
	if idx, val := c.backend.Validators().GetByAddress(c.lastProposer); val != nil {
		offset = idx
	}
	return uint64(offset) + c.current.Round().Uint64() + 1
}

func (c *core) setState(state State) {
	if c.state != state {
		c.state = state
	}
	if state == StateAcceptRequest {
		c.processPendingRequests()
	}
	c.processBacklog()
}

func (c *core) Address() common.Address {
	return c.address
}

func (c *core) Snapshot() (State, *snapshot) {
	return c.state, c.current
}

func (c *core) Backlog() map[istanbul.Validator]*prque.Prque {
	return c.backlogs
}

func (c *core) newRoundChangeTimer() {
	if c.roundChangeTimer != nil {
		c.roundChangeTimer.Stop()
	}

	timeout := time.Duration(c.config.RequestTimeout) * time.Millisecond
	c.roundChangeTimer = time.AfterFunc(timeout, func() {
		c.sendRoundChange()
	})
}

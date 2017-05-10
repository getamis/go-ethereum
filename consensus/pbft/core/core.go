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

	"gopkg.in/karalabe/cookiejar.v2/collections/prque"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/pbft"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
)

const (
	StateAcceptRequest State = iota
	StatePreprepared
	StatePrepared
	StateCommitted
	StateCheckpointReady
)

const (
	keyStableCheckpoint = "StableCheckpoint"
)

type State uint64

type Engine interface {
	Start() error
	Stop() error
}

func New(backend pbft.Backend) Engine {
	// update n and f
	n := int64(backend.Validators().Size())
	f := int64(math.Ceil(float64(n)/3) - 1)
	return &core{
		address:     backend.Address(),
		N:           n,
		F:           f,
		state:       StateAcceptRequest,
		logger:      log.New("address", backend.Address().Hex()),
		backend:     backend,
		sequence:    new(big.Int),
		viewNumber:  new(big.Int),
		internalMux: new(event.TypeMux),
		backlogs:    make(map[pbft.Validator]*prque.Prque),
		backlogsMu:  new(sync.Mutex),
		snapshotsMu: new(sync.RWMutex),
	}
}

// ----------------------------------------------------------------------------

type core struct {
	address common.Address
	N       int64
	F       int64
	state   State
	logger  log.Logger

	backend pbft.Backend
	events  *event.TypeMuxSubscription

	internalMux    *event.TypeMux
	internalEvents *event.TypeMuxSubscription

	sequence   *big.Int
	viewNumber *big.Int
	completed  bool

	subject *pbft.Subject

	backlogs   map[pbft.Validator]*prque.Prque
	backlogsMu *sync.Mutex

	current     *snapshot
	snapshots   []*snapshot
	snapshotsMu *sync.RWMutex
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

func (c *core) broadcast(msg *message) {
	logger := c.logger.New("state", c.state)

	payload, err := c.finalizeMessage(msg)
	if err != nil {
		logger.Error("Failed to finalize message", "msg", msg, "error", err)
		return
	}

	// Broadcast payload
	if err = c.backend.Broadcast(payload); err != nil {
		logger.Error("Failed to broadcast message", "msg", msg, "error", err)
		return
	}
}

func (c *core) nextSequence() *pbft.View {
	return &pbft.View{
		ViewNumber: c.viewNumber,
		Sequence:   new(big.Int).Add(c.sequence, common.Big1),
	}
}

func (c *core) nextViewNumber() *pbft.View {
	return &pbft.View{
		ViewNumber: new(big.Int).Add(c.viewNumber, common.Big1),
		Sequence:   c.sequence,
	}
}

func (c *core) isPrimary() bool {
	return c.backend.IsProposer()
}

func (c *core) makeProposal(seq *big.Int, request *pbft.Request) *pbft.Proposal {
	header := &pbft.ProposalHeader{
		Sequence: seq,
		// FIXME: use actual parent hash
		ParentHash: c.backend.Hash(request.BlockContext.Payload()),
		DataHash:   c.backend.Hash(request.BlockContext.Payload()),
	}

	return &pbft.Proposal{
		Header:       header,
		BlockContext: request.BlockContext,
	}
}

func (c *core) commit() {
	c.setState(StateCommitted)
	logger := c.logger.New("state", c.state)
	logger.Debug("Ready to commit", "view", c.current.Preprepare.View)
	c.backend.Commit(c.current.Preprepare.Proposal)

	c.snapshotsMu.Lock()
	c.snapshots = append(c.snapshots, c.current)
	c.snapshotsMu.Unlock()

	c.viewNumber = c.current.ViewNumber
	c.sequence = c.current.Sequence
	c.completed = true
	c.setState(StateAcceptRequest)

	// We build stable checkpoint every 100 requests
	// FIXME: this should be passed by configuration
	if new(big.Int).Mod(c.sequence, big.NewInt(100)).Int64() == 0 {
		go c.sendInternalEvent(buildCheckpointEvent{})
	}
}

func (c *core) setState(state State) {
	if c.state != state {
		c.state = state
		c.processBacklog()
	}
}

func (c *core) Address() common.Address {
	return c.address
}

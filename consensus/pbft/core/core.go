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
	"bytes"
	"math"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/pbft"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"gopkg.in/karalabe/cookiejar.v2/collections/prque"
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

func (s State) String() string {
	if s == StateAcceptRequest {
		return "Accept request"
	} else if s == StatePreprepared {
		return "Preprepared"
	} else if s == StatePrepared {
		return "Prepared"
	} else if s == StateCommitted {
		return "Committed"
	} else {
		return "Unknown"
	}
}

type Engine interface {
	Start(lastSequence *big.Int, lastProposer common.Address) error
	Stop() error
}

func New(backend pbft.Backend, config *pbft.Config) Engine {
	// update n and f
	n := int64(backend.Validators().Size())
	f := int64(math.Ceil(float64(n)/3) - 1)

	return &core{
		config:      config,
		address:     backend.Address(),
		N:           n,
		F:           f,
		state:       StateAcceptRequest,
		logger:      log.New("address", backend.Address().Hex()),
		backend:     backend,
		sequence:    common.Big0,
		round:       common.Big0,
		internalMux: new(event.TypeMux),
		backlogs:    make(map[pbft.Validator]*prque.Prque),
		backlogsMu:  new(sync.Mutex),
		snapshotsMu: new(sync.RWMutex),
	}
}

// ----------------------------------------------------------------------------

type core struct {
	config  *pbft.Config
	address common.Address
	N       int64
	F       int64
	state   State
	logger  log.Logger

	backend pbft.Backend
	events  *event.TypeMuxSubscription

	internalMux    *event.TypeMux
	internalEvents *event.TypeMuxSubscription

	sequence     *big.Int
	round        *big.Int
	lastProposer common.Address

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

func (c *core) currentView() *pbft.View {
	return &pbft.View{
		Sequence: new(big.Int).Set(c.sequence),
		Round:    new(big.Int).Set(c.round),
	}
}

func (c *core) nextRound() *pbft.View {
	return &pbft.View{
		Sequence: new(big.Int).Set(c.sequence),
		Round:    new(big.Int).Add(c.round, common.Big1),
	}
}

func (c *core) isPrimary() bool {
	return c.backend.IsProposer()
}

func (c *core) makeProposal(seq *big.Int, request *pbft.Request) *pbft.Proposal {
	header := &pbft.ProposalHeader{
		Sequence: seq,
		// FIXME: use actual parent hash
		ParentHash: c.backend.Hash(request.BlockContext.Number()),
		DataHash:   c.backend.Hash(request.BlockContext.Number()),
	}

	return &pbft.Proposal{
		Header:         header,
		RequestContext: request.BlockContext,
	}
}

func (c *core) commit() {
	c.setState(StateCommitted)
	logger := c.logger.New("state", c.state)
	logger.Debug("Ready to commit", "view", c.current.Preprepare.View)
	if err := c.backend.Commit(c.current.Preprepare.Proposal); err != nil {
		// TODO: fire a view change immediately
	}
}

func (c *core) proposerSeed() uint64 {
	if bytes.Compare(c.lastProposer.Bytes(), []byte{}) == 0 {
		return c.round.Uint64()
	}
	offset := 0
	if idx, val := c.backend.Validators().GetByAddress(c.lastProposer); val != nil {
		offset = idx
	}
	return uint64(offset) + c.round.Uint64() + 1
}

func (c *core) setState(state State) {
	if c.state != state {
		c.state = state
	}
	c.processBacklog()
}

func (c *core) Address() common.Address {
	return c.address
}

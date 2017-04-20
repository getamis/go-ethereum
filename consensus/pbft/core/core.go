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
	"sync"

	"gopkg.in/karalabe/cookiejar.v2/collections/prque"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/pbft"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
)

const (
	StateAcceptRequest int = iota
	StatePreprepared
	StatePrepared
	StateCommitted
	StateCheckpointReady
)

type Engine interface {
	Start()
	Stop()
}

func New(backend pbft.Backend) Engine {
	return &core{
		id:             backend.ID(),
		N:              4,
		F:              1,
		state:          StateAcceptRequest,
		logger:         log.New("backend", "simulation", "id", backend.ID()),
		backend:        backend,
		checkpointMsgs: make(map[uint64]*pbft.Checkpoint),
		sequence:       new(big.Int),
		viewNumber:     new(big.Int),
		events: backend.EventMux().Subscribe(
			pbft.RequestEvent{},
			pbft.ConnectionEvent{},
			pbft.MessageEvent{},
		),
		backlogs:        make(map[*pbft.Validator]*prque.Prque),
		backlogsMu:      new(sync.Mutex),
		consensusLogsMu: new(sync.RWMutex),
	}
}

// ----------------------------------------------------------------------------

type core struct {
	id     uint64
	N      int64
	F      int64
	state  int
	logger log.Logger

	backend pbft.Backend
	events  *event.TypeMuxSubscription

	sequence   *big.Int
	viewNumber *big.Int

	subject *pbft.Subject

	checkpointMsgs map[uint64]*pbft.Checkpoint

	backlogs   map[*pbft.Validator]*prque.Prque
	backlogsMu *sync.Mutex

	current         *pbft.Log
	consensusLogs   []*pbft.Log
	consensusLogsMu *sync.RWMutex
}

func (c *core) broadcast(code uint64, msg interface{}) {
	m, err := pbft.Encode(code, msg)
	if err != nil {
		log.Error("failed to encode message", "msg", msg, "error", err)
		return
	}

	payload, err := m.ToPayload()
	if err != nil {
		log.Error("failed to marshal message", "msg", msg, "error", err)
		return
	}

	c.backend.Send(payload)
}

func (c *core) nextSequence() *pbft.View {
	return &pbft.View{
		ViewNumber: c.viewNumber,
		Sequence:   new(big.Int).Add(c.sequence, common.Big1),
	}
}

func (c *core) primaryIDView() *big.Int {
	return new(big.Int).Mod(c.viewNumber, big.NewInt(c.N))
}

func (c *core) primaryID() *big.Int {
	return c.primaryIDView()
}

func (c *core) isPrimary() bool {
	return c.primaryID().Uint64() == c.ID()
}

func (c *core) makeProposal(seq *big.Int, request *pbft.Request) *pbft.Proposal {
	header := &pbft.ProposalHeader{
		Sequence:   seq,
		ParentHash: c.backend.Hash(request.Payload),
		DataHash:   c.backend.Hash(request.Payload),
	}

	rawHeader, _ := c.backend.Encode(header)

	return &pbft.Proposal{
		Header:  rawHeader,
		Payload: request.Payload,
	}
}

func (c *core) commit() {
	c.state = StateCommitted
	logger := c.logger.New("state", c.state)
	logger.Debug("Ready to commit", "view", c.current.Preprepare.View)
	c.backend.Commit(c.current.Preprepare.Proposal)
	c.processBacklog()

	c.consensusLogsMu.Lock()
	c.consensusLogs = append(c.consensusLogs, c.current)
	c.consensusLogsMu.Unlock()

	c.viewNumber = c.current.ViewNumber
	c.sequence = c.current.Sequence
	c.state = StateAcceptRequest
}

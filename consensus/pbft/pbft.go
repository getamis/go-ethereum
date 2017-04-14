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

package pbft

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

const (
	StateAcceptRequest int = iota
	StatePreprepared
	StatePrepared
	StateCommitted
	StateCheckpointReady
)

type Algorithm interface {
	Handler

	NewRequest(payload []byte)
}

func New(backend Backend) Algorithm {
	pbft := &pbft{
		id:             backend.ID(),
		N:              4,
		F:              1,
		state:          StateAcceptRequest,
		backend:        backend,
		prepareMsgs:    make(map[uint64]*Subject),
		commitMsgs:     make(map[uint64]*Subject),
		checkpointMsgs: make(map[uint64]*Checkpoint),
		sequence:       new(big.Int),
		viewNumber:     new(big.Int),
	}

	return pbft
}

// ----------------------------------------------------------------------------

type pbft struct {
	id    uint64
	N     int64
	F     int64
	state int

	backend Backend

	sequence   *big.Int
	viewNumber *big.Int

	subject        *Subject
	preprepareMsg  *Preprepare
	prepareMsgs    map[uint64]*Subject
	commitMsgs     map[uint64]*Subject
	checkpointMsgs map[uint64]*Checkpoint
}

func (pbft *pbft) NewRequest(payload []byte) {
	// Lazy preprepare
	pbft.sendPreprepare(&Request{
		Payload: payload,
	})
}

func (pbft *pbft) broadcast(code uint64, msg interface{}) {
	payload, err := Encode(code, msg)
	if err != nil {
		log.Error("failed to encode message", "msg", msg, "error", err)
		return
	}

	pbft.backend.Send(payload)
}

func (pbft *pbft) nextSequence() *View {
	return &View{
		ViewNumber: pbft.viewNumber,
		Sequence:   new(big.Int).Add(pbft.sequence, common.Big1),
	}
}

func (pbft *pbft) primaryIDView() *big.Int {
	return new(big.Int).Mod(pbft.viewNumber, big.NewInt(pbft.N))
}

func (pbft *pbft) primaryID() *big.Int {
	return pbft.primaryIDView()
}

func (pbft *pbft) isPrimary() bool {
	return pbft.primaryID().Uint64() == pbft.ID()
}

func (pbft *pbft) makeProposal(seq *big.Int, request *Request) *Proposal {
	header := &ProposalHeader{
		Sequence:   seq,
		ParentHash: pbft.backend.Hash(request.Payload),
		DataHash:   pbft.backend.Hash(request.Payload),
	}

	rawHeader, _ := pbft.backend.Encode(header)

	return &Proposal{
		Header:  rawHeader,
		Payload: request.Payload,
	}
}

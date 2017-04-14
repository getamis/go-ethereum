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
	MsgRequest uint64 = iota
	MsgPreprepare
	MsgPrepare
	MsgCommit
	MsgCheckpoint
	MsgViewChange
	MsgNewView
)

// TODO: under cooking
type State struct {
	View     *View
	Proposal *Proposal

	PrepareMsgs map[uint64]*Subject
	CommitMsgs  map[uint64]*Subject
}

type Request struct {
	Payload []byte
}

type View struct {
	ViewNumber *big.Int
	Sequence   *big.Int
}

type ProposalHeader struct {
	Sequence   *big.Int
	ParentHash common.Hash
	DataHash   common.Hash
}

type Proposal struct {
	Header     []byte
	Payload    []byte
	Signatures [][]byte
}

type Preprepare struct {
	View     *View
	Proposal *Proposal
}

type Subject struct {
	View   *View
	Digest []byte
}

type ViewChange struct {
	ViewNumber *big.Int
	PSet       []*Subject
	QSet       []*Subject
	Proposal   *Proposal
}

type SignedViewChange struct {
	Data      []byte
	Signature []byte
}

type NewView struct {
	ViewNumber *big.Int
	VSet       *SignedViewChange
	XSet       *Subject
	Proposal   *Proposal
}

type Checkpoint struct {
	Sequence  *big.Int
	Digest    []byte
	Signature []byte
}

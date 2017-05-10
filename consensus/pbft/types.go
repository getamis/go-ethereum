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
	"encoding/gob"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// TODO: under cooking
type State struct {
	View     *View
	Proposal *Proposal

	PrepareMsgs map[uint64]*Subject
	CommitMsgs  map[uint64]*Subject
}

// BlockContexter supports retrieving height and serialized block to be used during PBFT consensus.
type BlockContexter interface {
	// Number retrieves block height.
	Height() *big.Int

	// Payload returns a serialized block
	Payload() []byte
}

// NewBlockContext returns a BlockContext using the given payload and number.
// payload is serialized block, number is block height.
//
// TODO: Wrapper is not needed.
// We wrap the block by BlockContext struct since the gob cannot encode/decode private fields (like types.Block.header).
// So a custom encoder/decoder is needed. The go-ethereum rlp encoder/decoder is recommended here.
func NewBlockContext(payload []byte, number *big.Int) *BlockContext {
	return &BlockContext{
		RawData: payload,
		Number:  number,
	}
}

// BlockContext expose their members which allow the struct can be marshal/unmarshal by gob.
type BlockContext struct {
	RawData []byte
	Number  *big.Int
}

func (b *BlockContext) Height() *big.Int {
	return b.Number
}

func (b *BlockContext) Payload() []byte {
	return b.RawData
}

type Request struct {
	BlockContext BlockContexter
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
	Header       *ProposalHeader
	BlockContext BlockContexter
	Signatures   [][]byte
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

func init() {
	gob.Register(&Preprepare{})
	gob.Register(&Subject{})
	gob.Register(&BlockContext{})
}

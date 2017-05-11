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
	"io"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
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
	Number() *big.Int

	// Payload returns a serialized block
	Payload() []byte

	EncodeRLP(w io.Writer) error

	DecodeRLP(s *rlp.Stream) error
}

// NewBlockContext returns a BlockContext using the given payload and number.
// payload is serialized block, number is block height.
//
// TODO: Wrapper is not needed.
// We wrap the block by BlockContext struct since the gob cannot encode/decode private fields (like types.Block.header).
// So a custom encoder/decoder is needed. The go-ethereum rlp encoder/decoder is recommended here.
func NewBlockContext(payload []byte, height *big.Int) *BlockContext {
	return &BlockContext{
		RawData: payload,
		Height:  height,
	}
}

// BlockContext expose their members which allow the struct can be marshal/unmarshal by gob.
type BlockContext struct {
	RawData []byte
	Height  *big.Int
}

func (b *BlockContext) Number() *big.Int {
	return b.Height
}

func (b *BlockContext) Payload() []byte {
	return b.RawData
}

func (b *BlockContext) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{b.Height,b.RawData})
}

func (b *BlockContext) DecodeRLP(s *rlp.Stream) error {
	var proposal struct {
		RawData []byte
		Height *big.Int
	}

	if err := s.Decode(&proposal); err != nil {
		return err
	}
	b.RawData, b.Height = b.RawData, proposal.Height
	return nil
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

// EncodeRLP serializes b into the Ethereum RLP View format.
func (b *Proposal) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{b.Header, b.BlockContext, b.Signatures})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (b *Proposal) DecodeRLP(s *rlp.Stream) error {
	var proposal struct {
		Header       *ProposalHeader
		BlockContext *BlockContext
		Signatures   [][]byte
	}

	if err := s.Decode(&proposal); err != nil {
		return err
	}
	b.Header, b.BlockContext, b.Signatures = proposal.Header, proposal.BlockContext, proposal.Signatures
	return nil
}

type Preprepare struct {
	View     *View
	Proposal *Proposal
}

// EncodeRLP serializes b into the Ethereum RLP View format.
func (b *Preprepare) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{b.View, b.Proposal})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (b *Preprepare) DecodeRLP(s *rlp.Stream) error {
	var preprepare struct {
		View     *View
		Proposal *Proposal
	}

	if err := s.Decode(&preprepare); err != nil {
		return err
	}
	b.View, b.Proposal = preprepare.View, preprepare.Proposal

	return nil
}

type Subject struct {
	View   *View
	Digest []byte
}

// EncodeRLP serializes b into the Ethereum RLP View format.
func (b *Subject) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{b.View, b.Digest})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (b *Subject) DecodeRLP(s *rlp.Stream) error {
	var subject struct {
		View   *View
		Digest []byte
	}

	if err := s.Decode(&subject); err != nil {
		return err
	}
	b.View, b.Digest = subject.View, subject.Digest
	return nil
}

type ViewChange struct {
	ViewNumber *big.Int
	PSet       []*Subject
	QSet       []*Subject
	Proposal   *Proposal
}

// EncodeRLP serializes b into the Ethereum RLP View format.
func (b *ViewChange) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{b.ViewNumber, b.PSet, b.QSet, b.Proposal})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (b *ViewChange) DecodeRLP(s *rlp.Stream) error {
	var viewChange struct {
		ViewNumber *big.Int
		PSet       []*Subject
		QSet       []*Subject
		Proposal   *Proposal
	}

	if err := s.Decode(&viewChange); err != nil {
		return err
	}
	b.ViewNumber, b.PSet, b.QSet, b.Proposal = viewChange.ViewNumber, viewChange.PSet, viewChange.QSet, viewChange.Proposal
	return nil
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

// EncodeRLP serializes b into the Ethereum RLP View format.
func (b *NewView) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{b.ViewNumber, b.VSet, b.XSet, b.Proposal})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (b *NewView) DecodeRLP(s *rlp.Stream) error {
	var newView struct {
		ViewNumber *big.Int
		VSet       *SignedViewChange
		XSet       *Subject
		Proposal   *Proposal
	}

	if err := s.Decode(&newView); err != nil {
		return err
	}
	b.ViewNumber, b.VSet, b.XSet, b.Proposal = newView.ViewNumber, newView.VSet, newView.XSet, newView.Proposal
	return nil
}

type Checkpoint struct {
	Sequence  *big.Int
	Digest    []byte
	Signature []byte
}

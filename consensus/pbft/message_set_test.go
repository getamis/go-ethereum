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
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	set "gopkg.in/fatih/set.v0"
)

func makeBlock(number int64) *types.Block {
	header := &types.Header{
		Difficulty: big.NewInt(0),
		Number:     big.NewInt(number),
		GasLimit:   big.NewInt(0),
		GasUsed:    big.NewInt(0),
		Time:       big.NewInt(0),
	}
	block := &types.Block{}
	return block.WithSeal(header)
}

func TestMessageSetWithPreprepare(t *testing.T) {
	view := &View{
		ViewNumber: new(big.Int),
		Sequence:   new(big.Int),
	}
	ms := &messageSet{
		view:     view,
		msgType:  reflect.TypeOf(&Preprepare{}),
		messages: set.New(),
	}

	pp := &Preprepare{
		View: view,
		Proposal: &Proposal{
			Header: &ProposalHeader{
				Sequence:   big.NewInt(10),
				ParentHash: common.HexToHash("0x1234567890"),
				DataHash:   common.HexToHash("0x9876543210"),
			},
			RequestContext: makeBlock(1),
			Signatures: [][]byte{
				[]byte{0x01, 0x03},
				[]byte{0x02, 0x04},
			},
		},
	}

	added, err := ms.Add(pp, nil)
	if !added {
		t.Errorf("Failed to add message %v", pp)
	}
	if err != nil {
		t.Errorf("Failed to add message %v, err: %v", pp, err)
	}

	added, err = ms.Add(pp, nil)
	if added {
		t.Error("Same message should not be added again")
	}

	added, err = ms.Add(&Subject{
		View:   view,
		Digest: []byte{0x01},
	}, nil)
	if added || err == nil {
		t.Error("Different messages should not be added")
	}

	if ms.Size() != 1 {
		t.Error("There should be exactly one message in set")
	}

	if !reflect.DeepEqual(ms.view, view) {
		t.Error("View should be equal")
	}
}

func TestMessageSetWithSubject(t *testing.T) {
	view := &View{
		ViewNumber: new(big.Int),
		Sequence:   new(big.Int),
	}
	ms := &messageSet{
		view:     view,
		msgType:  reflect.TypeOf(&Subject{}),
		messages: set.New(),
	}

	s := &Subject{
		View:   view,
		Digest: []byte{0x01, 0x02},
	}

	added, err := ms.Add(s, nil)
	if !added {
		t.Errorf("Failed to add message %v", s)
	}
	if err != nil {
		t.Errorf("Failed to add message %v, err: %v", s, err)
	}

	added, err = ms.Add(s, nil)
	if added {
		t.Error("Same message should not be added again")
	}

	added, err = ms.Add(&Preprepare{
		View: view,
		Proposal: &Proposal{
			Header: &ProposalHeader{
				Sequence:   big.NewInt(10),
				ParentHash: common.HexToHash("0x1234567890"),
				DataHash:   common.HexToHash("0x9876543210"),
			},
			RequestContext: makeBlock(1),
			Signatures: [][]byte{
				[]byte{0x01, 0x03},
				[]byte{0x02, 0x04},
			},
		},
	}, nil)
	if added || err == nil {
		t.Error("Different messages should not be added")
	}

	if ms.Size() != 1 {
		t.Error("There should be exactly one message in set")
	}

	if !reflect.DeepEqual(ms.view, view) {
		t.Error("View should be equal")
	}
}

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
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/consensus/pbft"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/common"
)

func TestMessageSetWithPreprepare(t *testing.T) {
	valSet := newTestValidatorSet(4)
	valSet.CalcProposer(0)

	ms := newMessageSet(valSet)

	view := &pbft.View{
		Round:    new(big.Int),
		Sequence: new(big.Int),
	}
	pp := &pbft.Preprepare{
		View:     view,
		Proposal: makeBlock(1),
	}

	rawPP, err := rlp.EncodeToBytes(pp)
	if err != nil {
		t.Errorf("Failed to encode preprepare %v, err: %v", pp, err)
	}
	msg := &message{
		Code:    msgPreprepare,
		Msg:     rawPP,
		Address: valSet.GetProposer().Address(),
	}

	err = ms.Add(msg)
	if err != nil {
		t.Errorf("Failed to add message %v, err: %v", msg, err)
	}

	err = ms.Add(msg)
	if err != nil {
		t.Errorf("Failed to add message %v, err: %v", msg, err)
	}

	if ms.Size() != 1 {
		t.Error("There should be exactly one message in set")
	}
}

func TestMessageSetWithSubject(t *testing.T) {
	valSet := newTestValidatorSet(4)
	valSet.CalcProposer(0)

	ms := newMessageSet(valSet)

	view := &pbft.View{
		Round:    new(big.Int),
		Sequence: new(big.Int),
	}

	sub := &pbft.Subject{
		View:   view,
		Digest: common.StringToHash("1234567890"),
	}

	rawSub, err := rlp.EncodeToBytes(sub)
	if err != nil {
		t.Errorf("Failed to encode subject %v, err: %v", sub, err)
	}

	msg := &message{
		Code:    msgPrepare,
		Msg:     rawSub,
		Address: valSet.GetProposer().Address(),
	}

	err = ms.Add(msg)
	if err != nil {
		t.Errorf("Failed to add message %v, err: %v", msg, err)
	}

	err = ms.Add(msg)
	if err != nil {
		t.Errorf("Failed to add message %v, err: %v", msg, err)
	}

	if ms.Size() != 1 {
		t.Error("There should be exactly one message in set")
	}
}

func TestMessageSetEncodeDecode(t *testing.T) {
	valSet := newTestValidatorSet(10)
	valSet.CalcProposer(0)

	ms := newMessageSet(valSet)

	for i := 0; i < 10; i++ {
		view := &pbft.View{
			Round:    new(big.Int),
			Sequence: new(big.Int),
		}
		pp := &pbft.Preprepare{
			View:     view,
			Proposal: makeBlock(1),
		}

		rawPP, err := rlp.EncodeToBytes(pp)
		if err != nil {
			t.Errorf("Failed to encode preprepare %v, err: %v", pp, err)
		}
		msg := &message{
			Code:      msgPreprepare,
			Msg:       rawPP,
			Address:   valSet.GetProposer().Address(),
			Signature: []byte{0x01, 0x02},
		}

		err = ms.Add(msg)
		if err != nil {
			t.Errorf("Failed to add message %v, err: %v", msg, err)
		}
	}

	rawMessageSet, err := rlp.EncodeToBytes(ms)
	if err != nil {
		t.Errorf("Failed to encode message set %v, err: %v", ms, err)
	}

	var decodedMsgSet *messageSet
	err = rlp.DecodeBytes(rawMessageSet, &decodedMsgSet)
	if err != nil {
		t.Errorf("Failed to decode message set, err: %v", err)
	}

	if !reflect.DeepEqual(decodedMsgSet, ms) {
		t.Errorf("Decoded message set must equal to the original one\nexpected: %+v\ngot: %+v\n", ms, decodedMsgSet)
	}
}

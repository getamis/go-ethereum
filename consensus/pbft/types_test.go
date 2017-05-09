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
	"bytes"
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func testPreprepare(t *testing.T) {
	pp := &Preprepare{
		View: &View{
			ViewNumber: big.NewInt(1),
			Sequence:   big.NewInt(2),
		},
		Proposal: &Proposal{
			Header: &ProposalHeader{
				Sequence:   big.NewInt(10),
				ParentHash: common.HexToHash("0x1234567890"),
				DataHash:   common.HexToHash("0x9876543210"),
			},
			BlockContext: NewBlockContext([]byte{0x02}, big.NewInt(2)),
			Signatures: [][]byte{
				[]byte{0x01},
				[]byte{0x02},
			},
		},
	}

	m, err := Encode(MsgPreprepare, pp, nil)
	if err != nil {
		t.Error(err)
	}

	msgPayload, err := m.ToPayload()
	if err != nil {
		t.Error(err)
	}

	decodedMsg, err := Decode(msgPayload, nil)
	if err != nil {
		t.Error(err)
	}

	var decodedPP *Preprepare
	decodedPP = decodedMsg.Msg.(*Preprepare)
	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(pp, decodedPP) {
		t.Errorf("messages are different, expected '%+v', got '%+v'", pp, decodedPP)
	}
}

func testSubject(t *testing.T) {
	s := &Subject{
		View: &View{
			ViewNumber: big.NewInt(1),
			Sequence:   big.NewInt(2),
		},
		Digest: []byte{0x01, 0x02},
	}

	m, err := Encode(MsgPrepare, s, nil)
	if err != nil {
		t.Error(err)
	}

	msgPayload, err := m.ToPayload()
	if err != nil {
		t.Error(err)
	}

	decodedMsg, err := Decode(msgPayload, nil)
	if err != nil {
		t.Error(err)
	}

	var decodedSub *Subject
	decodedSub = decodedMsg.Msg.(*Subject)
	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(s, decodedSub) {
		t.Errorf("messages are different, expected '%+v', got '%+v'", s, decodedSub)
	}
}

func testSignature(t *testing.T) {
	s := &Subject{Digest: []byte{0x01, 0x02}}

	// 1. Encode test
	// 1.1. Test nil sign func
	m, err := Encode(MsgPrepare, s, nil)
	if err != nil {
		t.Error(err)
	}
	if m.Signature != nil {
		t.Errorf("Signature should be nil, but got :%v", m.Signature)
	}

	// 1.2. Test sign fun
	expectedSig := []byte{0x01}
	m, err = Encode(MsgPrepare, s, func(data []byte) ([]byte, error) {
		return expectedSig, nil
	})
	if bytes.Compare(m.Signature, expectedSig) != 0 {
		t.Errorf("Signature should be %v, but got: %v", expectedSig, m.Signature)
	}

	// 2. Decode test
	msgPayload, err := m.ToPayload()
	if err != nil {
		t.Error(err)
	}

	// 2.1 Test nil validate func
	_, err = Decode(msgPayload, nil)
	if err != nil {
		t.Errorf("Decode should succeed, but got error: %v", err)
	}

	// 2.2 Test validate func
	_, err = Decode(msgPayload, func(data []byte, sig []byte) (common.Address, error) {
		return common.Address{}, ErrNoMatchingValidator
	})
	if err != ErrNoMatchingValidator {
		t.Errorf("Expect ErrNoMatchingValidator error, but got: %v", err)
	}
}

func TestMessageEncodeDecode(t *testing.T) {
	testPreprepare(t)
	testSubject(t)
	testSignature(t)
}

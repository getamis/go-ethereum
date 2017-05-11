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

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/pbft"
)

func testPreprepare(t *testing.T) {
	pp := &pbft.Preprepare{
		View: &pbft.View{
			ViewNumber: big.NewInt(1),
			Sequence:   big.NewInt(2),
		},
		Proposal: &pbft.Proposal{
			Header: &pbft.ProposalHeader{
				Sequence:   big.NewInt(10),
				ParentHash: common.HexToHash("0x1234567890"),
				DataHash:   common.HexToHash("0x9876543210"),
			},
			BlockContext: pbft.NewBlockContext([]byte{0x02}, big.NewInt(2)),
			Signatures: [][]byte{
				[]byte{0x01},
				[]byte{0x02},
			},
		},
	}
	prepreparePayload, _ := Encode(pp)

	m := &message{
		Code:    msgPreprepare,
		Msg:     prepreparePayload,
		Address: common.HexToAddress("0x1234567890"),
	}

	msgPayload, err := m.Payload()
	if err != nil {
		t.Error(err)
	}

	decodedMsg := new(message)
	err = decodedMsg.FromPayload(msgPayload, nil)
	if err != nil {
		t.Error(err)
	}

	var decodedPP *pbft.Preprepare
	err = decodedMsg.Decode(&decodedPP)
	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(pp, decodedPP) {
		t.Errorf("messages are different, expected '%+v', got '%+v'", pp, decodedPP)
	}
}

func testSubject(t *testing.T) {
	s := &pbft.Subject{
		View: &pbft.View{
			ViewNumber: big.NewInt(1),
			Sequence:   big.NewInt(2),
		},
		Digest: []byte{0x01, 0x02},
	}

	subjectPayload, _ := Encode(s)

	m := &message{
		Code:    msgPreprepare,
		Msg:     subjectPayload,
		Address: common.HexToAddress("0x1234567890"),
	}

	msgPayload, err := m.Payload()
	if err != nil {
		t.Error(err)
	}

	decodedMsg := new(message)
	err = decodedMsg.FromPayload(msgPayload, nil)
	if err != nil {
		t.Error(err)
	}

	var decodedSub *pbft.Subject
	err = decodedMsg.Decode(&decodedSub)
	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(s, decodedSub) {
		t.Errorf("messages are different, expected '%+v', got '%+v'", s, decodedSub)
	}
}

func testWithSignature(t *testing.T) {
	s := &pbft.Subject{
		View: &pbft.View{
			ViewNumber: big.NewInt(1),
			Sequence:   big.NewInt(2),
		},
		Digest: []byte{0x01, 0x02},
	}
	expectedSig := []byte{0x01}

	subjectPayload, _ := Encode(s)
	// 1. Encode test
	m := &message{
		Code:      msgPreprepare,
		Msg:       subjectPayload,
		Address:   common.HexToAddress("0x1234567890"),
		Signature: expectedSig,
	}

	msgPayload, err := m.Payload()
	if err != nil {
		t.Error(err)
	}

	// 2. Decode test
	// 2.1 Test normal validate func
	decodedMsg := new(message)
	err = decodedMsg.FromPayload(msgPayload, func(data []byte, sig []byte) (common.Address, error) {
		return common.Address{}, nil
	})
	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(decodedMsg, m) {
		t.Errorf("Messages are different, expected '%+v', got '%+v'", m, decodedMsg)
	}

	// 2.2 Test nil validate func
	decodedMsg = new(message)
	err = decodedMsg.FromPayload(msgPayload, nil)
	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(decodedMsg, m) {
		t.Errorf("Messages are different, expected '%+v', got '%+v'", m, decodedMsg)
	}

	// 2.3 Test failed validate func
	decodedMsg = new(message)
	err = decodedMsg.FromPayload(msgPayload, func(data []byte, sig []byte) (common.Address, error) {
		return common.Address{}, pbft.ErrNoMatchingValidator
	})
	if err != pbft.ErrNoMatchingValidator {
		t.Errorf("Expect ErrNoMatchingValidator error, but got: %v", err)
	}
}

func TestMessageEncodeDecode(t *testing.T) {
	testPreprepare(t)
	testSubject(t)
	testWithSignature(t)
}

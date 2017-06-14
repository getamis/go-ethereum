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
	"github.com/ethereum/go-ethereum/consensus/istanbul"
)

func testPreprepare(t *testing.T) {
	pp := &istanbul.Preprepare{
		View: &istanbul.View{
			Round:    big.NewInt(1),
			Sequence: big.NewInt(2),
		},
		Proposal: makeBlock(1),
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

	var decodedPP *istanbul.Preprepare
	err = decodedMsg.Decode(&decodedPP)
	if err != nil {
		t.Error(err)
	}

	// if block is encoded/decoded by rlp, we cannot to compare interface data type using reflect.DeepEqual. (like istanbul.Proposal)
	// so individual comparison here.
	if !reflect.DeepEqual(pp.Proposal.Hash(), decodedPP.Proposal.Hash()) {
		t.Errorf("Header are different, expected '%+v', got '%+v'", pp.Proposal, decodedPP.Proposal)
	}

	if !reflect.DeepEqual(pp.View, decodedPP.View) {
		t.Errorf("View are different, expected '%+v', got '%+v'", pp.View, decodedPP.View)
	}

	if !reflect.DeepEqual(pp.Proposal.Number(), decodedPP.Proposal.Number()) {
		t.Errorf("Block number are different, expected '%+v', got '%+v'", pp, decodedPP)
	}
}

func testSubject(t *testing.T) {
	s := &istanbul.Subject{
		View: &istanbul.View{
			Round:    big.NewInt(1),
			Sequence: big.NewInt(2),
		},
		Digest: common.StringToHash("1234567890"),
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

	var decodedSub *istanbul.Subject
	err = decodedMsg.Decode(&decodedSub)
	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(s, decodedSub) {
		t.Errorf("messages are different, expected '%+v', got '%+v'", s, decodedSub)
	}
}

func testSubjectWithSignature(t *testing.T) {
	s := &istanbul.Subject{
		View: &istanbul.View{
			Round:    big.NewInt(1),
			Sequence: big.NewInt(2),
		},
		Digest: common.StringToHash("1234567890"),
	}
	expectedSig := []byte{0x01}

	subjectPayload, _ := Encode(s)
	// 1. Encode test
	m := &message{
		Code:          msgPreprepare,
		Msg:           subjectPayload,
		Address:       common.HexToAddress("0x1234567890"),
		Signature:     expectedSig,
		CommittedSeal: []byte{},
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
		return common.Address{}, istanbul.ErrUnauthorizedAddress
	})
	if err != istanbul.ErrUnauthorizedAddress {
		t.Errorf("Expect ErrUnauthorizedAddress error, but got: %v", err)
	}
}

func testRoundChange(t *testing.T) {
	rc := &roundChange{
		Round:    big.NewInt(1),
		Sequence: big.NewInt(2),
		Digest:   common.StringToHash("1234567890"),
	}
	RoundChangePayload, _ := Encode(rc)

	m := &message{
		Code:    msgRoundChange,
		Msg:     RoundChangePayload,
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

	var decodedRC *roundChange
	err = decodedMsg.Decode(&decodedRC)
	if err != nil {
		t.Error(err)
	}

	// if block is encoded/decoded by rlp, we cannot to compare interface data type using reflect.DeepEqual. (like istanbul.Proposal)
	// so individual comparison here.
	if !reflect.DeepEqual(rc, decodedRC) {
		t.Errorf("messages are different, expected '%+v', got '%+v'", rc, decodedRC)
	}
}

func TestMessageEncodeDecode(t *testing.T) {
	testPreprepare(t)
	testSubject(t)
	testSubjectWithSignature(t)
	testRoundChange(t)
}

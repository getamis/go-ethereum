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
)

func testPreprepare(t *testing.T) {
	pp := &Preprepare{
		View: &View{
			ViewNumber: big.NewInt(1),
			Sequence:   big.NewInt(2),
		},
		Proposal: &Proposal{
			Header:  []byte{0x01},
			Payload: []byte{0x02},
			Signatures: [][]byte{
				[]byte{0x01},
				[]byte{0x02},
			},
		},
	}

	m, err := Encode(MsgPreprepare, pp)
	if err != nil {
		t.Error(err)
	}

	var decodedPP *Preprepare
	err = Decode(m, &decodedPP)
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

	m, err := Encode(MsgPrepare, s)
	if err != nil {
		t.Error(err)
	}

	var decodedSub *Subject
	err = Decode(m, &decodedSub)
	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(s, decodedSub) {
		t.Errorf("messages are different, expected '%+v', got '%+v'", s, decodedSub)
	}
}

func TestMessageEncodeDecode(t *testing.T) {
	testPreprepare(t)
	testSubject(t)
}

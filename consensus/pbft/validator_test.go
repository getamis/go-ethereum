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
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

var (
	testAddress  = "0x70524d664ffe731100208a0154e556f9bb679ae6"
	testAddress2 = "0xb37866a925bccd69cfa98d43b510f1d23d78a851"
)

func TestValidatorSet(t *testing.T) {
	testNormalValSet(t)
	testEmptyValSet(t)
}

func testNormalValSet(t *testing.T) {
	valSet := NewValidatorSet()
	add1 := common.HexToAddress(testAddress)
	val1 := NewValidator(uint(0), addr1)
	add2 := common.HexToAddress(testAddress2)
	val2 := NewValidator(uint(1), addr2)
	vals := []*Validator{val1, val2}

	valSet := NewValidatorSet(vals)

	// check size
	if size := valSet.Size(); size != 2 {
		t.Errorf("wrong peer set size, got: %v, expected: 2", size)

	}
	// test get by index
	if val := valSet.GetByIndex(uint(0)); val != val1 {
		t.Errorf("get wrong validator, got: %v, expected: %v", val, val1)
	}
	// test get by invalid index
	if val := valSet.GetByIndex(uint(2)); val != nil {
		t.Errorf("get wrong validator, got: %v, expected: nil", val)
	}
	// test get by address
	if val := valSet.GetByAddress(add2); val != val2 {
		t.Errorf("get wrong validator, got: %v, expected: %v", val, val2)
	}
	// test get by invalid address
	invalidAddr := common.HexToAddress("0x9535b2e7faaba5288511d89341d94a38063a349b")
	if val := valSet.GetByAddress(invalidAddr); val != nil {
		t.Errorf("get wrong validator, got: %v, expected: nil", val)
	}
	// test get proposer
	if val := valSet.GetProposer(); val != val1 {
		t.Errorf("get wrong proposer, got: %v, expected: %v", val, val1)
	}
	// test calculate proposer
	valSet.CalcProposer(uint64(3))
	if val := valSet.GetProposer(); val != val2 {
		t.Errorf("get wrong proposer, got: %v, expected: %v", val, val2)
	}
}

func testEmptyValSet(t *testing.T) {
	vals := []*Validator{}
	valSet := NewValidatorSet(vals)

	// check size
	if size := valSet.Size(); size != 0 {
		t.Errorf("wrong peer set size, got: %v, expected: 0", size)

	}
	// test get by index
	if val := valSet.GetByIndex(uint(0)); val != nil {
		t.Errorf("get wrong validator, got: %v, expected: nil", val)
	}
	// test get by invalid address
	invalidAddr := common.HexToAddress("0x9535b2e7faaba5288511d89341d94a38063a349b")
	if val := valSet.GetByAddress(invalidAddr); val != nil {
		t.Errorf("get wrong validator, got: %v, expected: nil", val)
	}
	// test get proposer
	if val := valSet.GetProposer(); val != nil {
		t.Errorf("get wrong proposer, got: %v, expected: nil", val)
	}
	// test calculate proposer
	valSet.CalcProposer(uint64(3))
	if val := valSet.GetProposer(); val != nil {
		t.Errorf("get wrong proposer, got: %v, expected: nil", val)
	}
}

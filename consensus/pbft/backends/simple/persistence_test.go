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

package simple

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
)

type Foo struct {
	Name   string
	Number *big.Int
	Hash   common.Hash
}

func TestSaveAndRestore(t *testing.T) {
	ethDB, err := ethdb.NewMemDatabase()
	if err != nil {
		t.Error("New eth memory database failed")
	}
	b := &simpleBackend{
		db: ethDB,
	}
	key := "key"
	foo := Foo{
		Name:   "name",
		Number: big.NewInt(9),
		Hash:   common.HexToHash("1234567890"),
	}
	err = b.Save(key, foo)
	if err != nil {
		t.Error("Should save the object correctly")
	}

	result := Foo{}
	err = b.Restore(key, &result)
	if err != nil {
		t.Error("Should restore the object correctly")
	}
	if !reflect.DeepEqual(foo, result) {
		t.Errorf("Should have the same data, result = %v, expected = %v", result, foo)
	}
}

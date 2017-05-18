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

package validator

import (
	"bytes"
	"reflect"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/pbft"
)

type Validators []pbft.Validator

func (slice Validators) Len() int {
	return len(slice)
}

func (slice Validators) Less(i, j int) bool {
	return strings.Compare(slice[i].Address().Hex(), slice[j].Address().Hex()) < 0
}

func (slice Validators) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

type defaultValidator struct {
	address common.Address
}

func (val *defaultValidator) Address() common.Address { return val.address }

type defaultSet struct {
	validators Validators
	proposer   pbft.Validator
}

func newDefaultSet(addrs []common.Address) *defaultSet {
	valSet := &defaultSet{}

	// init validators
	valSet.validators = make([]pbft.Validator, len(addrs))
	for i, addr := range addrs {
		valSet.validators[i] = New(addr)
	}
	// sort validator
	sort.Sort(valSet.validators)
	// init proposer
	valSet.CalcProposer(0)

	return valSet
}

func (valSet *defaultSet) Size() int              { return len(valSet.validators) }
func (valSet *defaultSet) List() []pbft.Validator { return valSet.validators }

func (valSet *defaultSet) GetByIndex(i uint64) pbft.Validator {
	if i < uint64(valSet.Size()) {
		return valSet.validators[i]
	}
	return nil
}

func (valSet *defaultSet) GetByAddress(addr common.Address) (int, pbft.Validator) {
	for i, val := range valSet.List() {
		if bytes.Compare(addr.Bytes(), val.Address().Bytes()) == 0 {
			return i, val
		}
	}
	return -1, nil
}

func (valSet *defaultSet) GetProposer() pbft.Validator {
	return valSet.proposer
}

func (valSet *defaultSet) IsProposer(address common.Address) bool {
	_, val := valSet.GetByAddress(address)
	return reflect.DeepEqual(valSet.GetProposer(), val)
}

func (valSet *defaultSet) CalcProposer(seed uint64) {
	if valSet.Size() != 0 {
		pick := seed % uint64(valSet.Size())
		valSet.proposer = valSet.validators[pick]
	}
}

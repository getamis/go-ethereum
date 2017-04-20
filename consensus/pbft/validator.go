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

	"github.com/ethereum/go-ethereum/common"
)

type Validator struct {
	id      uint64
	address common.Address
}

func NewValidator(id uint64, addr common.Address) *Validator {
	return &Validator{
		id:      id,
		address: addr,
	}
}

func (val *Validator) ID() uint64              { return val.id }
func (val *Validator) Address() common.Address { return val.address }

//------------------------------------------------------------------------

type ValidatorSet struct {
	validators []*Validator
	proposer   *Validator
}

func NewValidatorSet(vals []*Validator) *ValidatorSet {
	vs := &ValidatorSet{
		validators: vals,
	}
	vs.CalcProposer(0)
	return vs
}

func (valSet *ValidatorSet) Size() int          { return len(valSet.validators) }
func (valSet *ValidatorSet) List() []*Validator { return valSet.validators }

func (valSet *ValidatorSet) GetByIndex(i uint64) *Validator {
	if i < uint64(valSet.Size()) {
		return valSet.validators[i]
	}
	return nil
}

func (valSet *ValidatorSet) GetByAddress(addr common.Address) *Validator {
	for _, val := range valSet.List() {
		if bytes.Compare(addr.Bytes(), val.Address().Bytes()) == 0 {
			return val
		}
	}
	return nil
}

func (valSet *ValidatorSet) GetProposer() *Validator {
	if valSet.proposer == nil {
		valSet.CalcProposer(0)
	}
	return valSet.proposer
}

func (valSet *ValidatorSet) CalcProposer(seed uint64) {
	if valSet.Size() != 0 {
		pick := seed % uint64(valSet.Size())
		valSet.proposer = valSet.validators[pick]
	}
}

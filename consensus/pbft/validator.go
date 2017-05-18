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

import "github.com/ethereum/go-ethereum/common"

type Validator interface {
	// Return address
	Address() common.Address
}

type ValidatorSet interface {
	// Calculate the proposer
	CalcProposer(seed uint64)
	// Return the validator size
	Size() int
	// Return the validator array
	List() []Validator
	// Get validator by index
	GetByIndex(i uint64) Validator
	// Get validator by address
	GetByAddress(addr common.Address) (int, Validator)
	// Get current proposer
	GetProposer() Validator
	// Check whether the validator with address is a proposer
	IsProposer(address common.Address) bool
}

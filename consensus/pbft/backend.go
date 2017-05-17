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

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
)

// Backend provides callbacks for PBFT consensus core
type Backend interface {
	// Address returns self address
	Address() common.Address

	// Validators returns validator set
	Validators() ValidatorSet

	// EventMux is defined to handle request event and pbft message event
	EventMux() *event.TypeMux

	// Send is to send pbft message to specific peer
	Send(payload []byte, target common.Address) error

	// Broadcast is to send pbft message to all peers
	Broadcast(payload []byte) error

	// UpdateState is to update the current pbft state to backend
	UpdateState(*State) error

	// Commit is to deliver a final result to write into blockchain
	Commit(*Proposal) error

	// ViewChanged is called when view change occurred
	ViewChanged(needNewProposal bool) error

	// Verify is to verify the proposal request
	Verify(*Proposal) error

	// Sign is to sign the data
	Sign([]byte) ([]byte, error)

	// Check wether I'm a proposer
	IsProposer() bool

	// CheckSignature is to verify the signature is signed from given peer
	CheckSignature(data []byte, addr common.Address, sig []byte) error

	// CheckValidatorSignature verifies if the data is signed by one of the validators
	CheckValidatorSignature(data []byte, sig []byte) (common.Address, error)

	// LastCommitSequence returns latest block number
	LastCommitSequence() *big.Int

	// FIXME: Hash, Encode, Decode and SetHandler are workaround functions for developing
	Hash(b interface{}) common.Hash

	Persistence
}

type Persistence interface {
	// Save an object into db
	Save(key string, val interface{}) error
	// Restore an object to val from db
	Restore(key string, val interface{}) error
}

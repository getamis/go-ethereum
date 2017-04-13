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

// Backend provides callbacks for PBFT consensus core
type Backend interface {
	ID() uint64
	Peer() Peer
	Peers() []Peer
	AddPeer(Peer) error

	Send(uint64, interface{}, Peer)

	Hash(b interface{}) common.Hash
	Encode(b interface{}) ([]byte, error)
	Decode([]byte, interface{}) error

	Commit(*Proposal)

	SetHandler(h Handler)
}

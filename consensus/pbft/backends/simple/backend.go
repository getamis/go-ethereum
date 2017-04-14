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
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/pbft"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

func NewBackend(id uint64, n uint64, f uint64, eventMux *event.TypeMux) consensus.PBFT {
	backend := &simpleBackend{
		id:       id,
		n:        n,
		f:        f,
		peers:    make([]pbft.Peer, n),
		eventMux: eventMux,
		logger:   log.New("backend", "simple"),
	}

	return backend
}

// ----------------------------------------------------------------------------

type simpleBackend struct {
	id             uint64
	n              uint64
	f              uint64
	peers          []pbft.Peer
	eventMux       *event.TypeMux
	consensusState *pbft.State
	logger         log.Logger
	quitSync       chan struct{}
}

func (sb *simpleBackend) ID() uint64 {
	return sb.id
}

func (sb *simpleBackend) Peer(id uint64) pbft.Peer {
	return sb.peers[id]
}

func (sb *simpleBackend) Peers() []pbft.Peer {
	return sb.peers
}

func (sb *simpleBackend) Send(code uint64, msg interface{}, peer pbft.Peer) {
}

func (sb *simpleBackend) Commit(proposal *pbft.Proposal) {
	log.Info("Committed", "id", sb.ID(), "proposal", proposal)
}

func (sb *simpleBackend) Hash(x interface{}) (h common.Hash) {
	hw := sha3.NewKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

func (sb *simpleBackend) Encode(v interface{}) ([]byte, error) {
	return rlp.EncodeToBytes(v)
}

func (sb *simpleBackend) Decode(b []byte, v interface{}) error {
	return rlp.DecodeBytes(b, v)
}

func (sb *simpleBackend) EventMux() *event.TypeMux {
	// not implemented
	return nil
}

func (sb *simpleBackend) Verify(proposal *pbft.Proposal) (bool, error) {
	// not implemented
	return true, nil
}

func (sb *simpleBackend) Sign(data []byte) []byte {
	// not implemented
	return data
}

func (sb *simpleBackend) CheckSignature(data []byte, Peer, sig []byte) error {
	// not implemented
	return nil
}

func (sb *simpleBackend) UpdateState(state *pbft.State) {
	sb.consensusState = state
}

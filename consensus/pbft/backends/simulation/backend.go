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

package simulation

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/pbft"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rlp"
)

func NewSimulationBackend(id uint64, n uint64, f uint64) pbft.Backend {
	backend := &simulationBackend{
		id:     id,
		n:      n,
		f:      f,
		me:     newPeer(id),
		peers:  make([]pbft.Peer, n),
		logger: log.New("backend", "simulated"),
	}

	return backend
}

// ----------------------------------------------------------------------------

type simulationBackend struct {
	id          uint64
	n           uint64
	f           uint64
	me          *peer
	peers       []pbft.Peer
	logger      log.Logger
	newPeerCh   chan *peer
	quitSync    chan struct{}
	peerHandler pbft.Handler
}

func (sb *simulationBackend) SetHandler(h pbft.Handler) {
	sb.peerHandler = h
}

func (sb *simulationBackend) ID() uint64 {
	return sb.id
}

func (sb *simulationBackend) Peer() pbft.Peer {
	return sb.me
}

func (sb *simulationBackend) AddPeer(p pbft.Peer) error {

	if sb.ID() == p.ID() {
		return fmt.Errorf("Don't add myself, %v = %v", sb.ID(), p.ID())
	}

	go func() {
		for {
			if sb.peerHandler != nil {
				sb.peerHandler.Handle(p)
			}
		}
	}()

	sb.peers[p.ID()] = p
	return nil
}

func (sb *simulationBackend) Peers() []pbft.Peer {
	return sb.peers
}

func (sb *simulationBackend) Send(code uint64, msg interface{}, peer pbft.Peer) {
	if err := p2p.Send(sb.me.Writer(), code, msg); err != nil {
		log.Error("Failed to send message", "msg", msg, "error", err)
	}
}

func (sb *simulationBackend) Commit(proposal *pbft.Proposal) {
	log.Info("Committed", "id", sb.ID(), "proposal", proposal)
}

func (sb *simulationBackend) Hash(x interface{}) (h common.Hash) {
	hw := sha3.NewKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

func (sb *simulationBackend) Encode(v interface{}) ([]byte, error) {
	return rlp.EncodeToBytes(v)
}

func (sb *simulationBackend) Decode(b []byte, v interface{}) error {
	return rlp.DecodeBytes(b, v)
}

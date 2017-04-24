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
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/pbft"
	"github.com/ethereum/go-ethereum/consensus/pbft/backends"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rlp"
)

type CommitEvent struct {
	Payload []byte
}

var peers []*peer = []*peer{
	newPeer(uint64(0)),
	newPeer(uint64(1)),
	newPeer(uint64(2)),
	newPeer(uint64(3)),
}

func NewBackend(id uint64) *Backend {
	backend := &Backend{
		id:     id,
		me:     peers[id],
		peers:  make([]pbft.Peer, len(peers)),
		logger: log.New("backend", "simulated", "id", id),
		mux:    new(event.TypeMux),
	}

	backend.peers[id] = peers[id]

	go func() {
		for {
			m, err := backend.me.ReadMsg()
			if err != nil {
				backend.logger.Error("Failed to ReadMsg", "error", err)
				continue
			}

			defer m.Discard()

			// log.Debug("New message", "peer", peer, "msg", m)

			var payload []byte
			err = m.Decode(&payload)
			if err != nil {
				backend.logger.Error("Failed to read payload", "error", err, "msg", m)
				continue
			}

			go backend.mux.Post(pbft.MessageEvent{
				ID:      m.Code,
				Payload: payload,
			})
		}
	}()

	return backend
}

// ----------------------------------------------------------------------------

type Backend struct {
	id     uint64
	mux    *event.TypeMux
	appMux *event.TypeMux
	me     *peer
	peers  []pbft.Peer
	logger log.Logger
}

func (sb *Backend) ID() uint64 {
	return sb.id
}

func (sb *Backend) Peers() pbft.PeerSet {
	return backends.NewPeerSet(sb.peers)
}

func (sb *Backend) Send(payload []byte) {
	go func() {
		for _, p := range peers {
			if p.ID() != sb.me.ID() {
				p2p.Send(p, sb.ID(), payload)
			} else {
				go sb.mux.Post(pbft.MessageEvent{
					ID:      sb.ID(),
					Payload: payload,
				})
			}
		}
	}()
}

func (sb *Backend) Commit(proposal *pbft.Proposal) {
	go sb.mux.Post(CommitEvent{
		Payload: proposal.Payload,
	})
}

func (sb *Backend) Hash(x interface{}) (h common.Hash) {
	hw := sha3.NewKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

func (sb *Backend) Encode(v interface{}) ([]byte, error) {
	return rlp.EncodeToBytes(v)
}

func (sb *Backend) Decode(b []byte, v interface{}) error {
	return rlp.DecodeBytes(b, v)
}

func (sb *Backend) EventMux() *event.TypeMux {
	return sb.mux
}

func (sb *Backend) Verify(proposal *pbft.Proposal) (bool, error) {
	// not implemented
	return true, nil
}

func (sb *Backend) Sign(data []byte) ([]byte, error) {
	// not implemented
	return data, nil
}

func (sb *Backend) CheckSignature(data []byte, addr common.Address, sig []byte) error {
	// not implemented
	return nil
}

func (sb *Backend) UpdateState(*pbft.State) {
	// not implemented
}

func (sb *Backend) AddPeer(id string) {
	numID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		sb.logger.Error("Invalid peer ID", "id", id)
		return
	}
	if sb.ID() == uint64(numID) {
		sb.logger.Error("Don't add myself", sb.ID(), numID)
		return
	}

	sb.peers[numID] = peers[numID]
}

func (sb *Backend) RemovePeer(id string) {
}

func (sb *Backend) HandleMsg(id string, data []byte) {
	// TODO: forward pbft message to pbft engine
}

func (sb *Backend) Start() {
}

func (sb *Backend) Stop() {
}

func (sb *Backend) NewRequest(payload []byte) {
	go sb.mux.Post(pbft.RequestEvent{
		Payload: payload,
	})
}

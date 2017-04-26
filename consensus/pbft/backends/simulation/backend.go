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
	"crypto/ecdsa"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/pbft"
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
		peers:  make([]*peer, len(peers)),
		logger: log.New("backend", "simulated", "id", id),
		mux:    new(event.TypeMux),
	}
	var vals []*pbft.Validator
	for i, peer := range peers {
		vals = append(vals, pbft.NewValidator(uint64(i), peer.Address()))
	}
	backend.valSet = pbft.NewValidatorSet(vals)

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
	valSet *pbft.ValidatorSet
	peers  []*peer
	logger log.Logger
}

// ID implements pbft.Backend.ID
func (sb *Backend) ID() uint64 {
	return sb.id
}

// Validators implements pbft.Backend.Validators
func (sb *Backend) Validators() *pbft.ValidatorSet {
	return sb.valSet
}

// Send implements pbft.Backend.Send
func (sb *Backend) Send(payload []byte) error {
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
	return nil
}

// Commit implements pbft.Backend.Commit
func (sb *Backend) Commit(proposal *pbft.Proposal) error {
	go sb.mux.Post(CommitEvent{
		Payload: proposal.Payload,
	})
	return nil
}

// Hash implements pbft.Backend.Hash
func (sb *Backend) Hash(x interface{}) (h common.Hash) {
	hw := sha3.NewKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

// Encode implements pbft.Backend.Encode
func (sb *Backend) Encode(v interface{}) ([]byte, error) {
	return rlp.EncodeToBytes(v)
}

// Decode implements pbft.Backend.Decode
func (sb *Backend) Decode(b []byte, v interface{}) error {
	return rlp.DecodeBytes(b, v)
}

// EventMux implements pbft.Backend.EventMux
func (sb *Backend) EventMux() *event.TypeMux {
	return sb.mux
}

// Verify implements pbft.Backend.Verify
func (sb *Backend) Verify(proposal *pbft.Proposal) (bool, error) {
	// not implemented
	return true, nil
}

// Sign implements pbft.Backend.Sign
func (sb *Backend) Sign(data []byte) ([]byte, error) {
	// not implemented
	return data, nil
}

// CheckSignature implements pbft.Backend.CheckSignature
func (sb *Backend) CheckSignature(data []byte, addr common.Address, sig []byte) error {
	// not implemented
	return nil
}

// UpdateState implements pbft.Backend.UpdateState
func (sb *Backend) UpdateState(*pbft.State) error {
	// not implemented
	return nil
}

// ViewChanged implements pbft.Backend.ViewChanged
func (sb *Backend) ViewChanged(needNewProposal bool) error {
	// not implemented
	return nil
}

// AddPeer implements consensus.PBFT.AddPeer
func (sb *Backend) AddPeer(peerID string, publicKey *ecdsa.PublicKey) error {
	numID, err := strconv.ParseInt(peerID, 10, 64)
	if err != nil {
		sb.logger.Error("Invalid peer ID", "id", peerID)
		return pbft.ErrInvalidPeerId
	}
	if sb.ID() == uint64(numID) {
		sb.logger.Error("Don't add myself", sb.ID(), numID)
		return pbft.ErrInvalidPeerId
	}

	sb.peers[numID] = peers[numID]
	return nil
}

// RemovePeer implements consensus.PBFT.RemovePeer
func (sb *Backend) RemovePeer(peerID string) error {
	return nil
}

// HandleMsg implements consensus.PBFT.HandleMsg
func (sb *Backend) HandleMsg(peerID string, data []byte) error {
	// TODO: forward pbft message to pbft engine
	return nil
}

// Start implements consensus.PBFT.Start
func (sb *Backend) Start(chain consensus.ChainReader) error {
	return nil
}

// Stop implements consensus.PBFT.Stop
func (sb *Backend) Stop() error {
	return nil
}

func (sb *Backend) NewRequest(payload []byte) {
	go sb.mux.Post(pbft.RequestEvent{
		Payload: payload,
	})
}

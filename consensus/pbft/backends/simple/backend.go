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
	"bytes"
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/pbft"
	"github.com/ethereum/go-ethereum/consensus/pbft/backends"
	pbftCore "github.com/ethereum/go-ethereum/consensus/pbft/core"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	extraVanity = 32 // Fixed number of extra-data prefix bytes reserved for signer vanity
	extraSeal   = 65 // Fixed number of extra-data suffix bytes reserved for signer seal
)

func NewBackend(n uint64, f uint64, eventMux *event.TypeMux, privateKey *ecdsa.PrivateKey, blockchain *core.BlockChain) consensus.PBFT {
	backend := &simpleBackend{
		n:            n,
		f:            f,
		blockchain:   blockchain,
		eventMux:     eventMux,
		pbftEventMux: new(event.TypeMux),
		privateKey:   privateKey,
		logger:       log.New("backend", "simple"),
	}

	backend.core = pbftCore.New(backend)
	backend.initPeerSet()

	return backend
}

// ----------------------------------------------------------------------------

type simpleBackend struct {
	id             uint64
	n              uint64
	f              uint64
	blockchain     *core.BlockChain
	peerSet        pbft.PeerSet
	eventMux       *event.TypeMux
	pbftEventMux   *event.TypeMux
	privateKey     *ecdsa.PrivateKey
	consensusState *pbft.State
	core           pbftCore.Engine
	logger         log.Logger
	quitSync       chan struct{}
}

func (sb *simpleBackend) ID() uint64 {
	return sb.id
}

func (sb *simpleBackend) Peers() pbft.PeerSet {
	return sb.peerSet
}

func (sb *simpleBackend) Send(data []byte) {
	peers := sb.peerSet.Peers()
	for _, peer := range peers {
		// just post pbft event to pbft core engine if peer is eaqual to self
		if peer.ID() == sb.ID() {
			go sb.pbftEventMux.Post(pbft.MessageEvent{
				ID:      peer.ID(),
				Payload: data,
			})
		} else {
			if peer.IsConnected() {
				go sb.eventMux.Post(eth.PBFTEvent{
					PeerPublicKey: peer.PublicKey(),
					Data:          data,
				})
			}
		}
	}
}

func (sb *simpleBackend) Commit(proposal *pbft.Proposal) {
	log.Info("Committed", "id", sb.ID(), "proposal", proposal)
	// step1: update validator set from extra data of block
	// step2: insert chain

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
	return sb.pbftEventMux
}

func (sb *simpleBackend) Verify(proposal *pbft.Proposal) (bool, error) {
	// not implemented
	return true, nil
}

func (sb *simpleBackend) Sign(data []byte) ([]byte, error) {
	hashData := crypto.Keccak256([]byte(data))
	return crypto.Sign(hashData, sb.privateKey)
}

func (sb *simpleBackend) CheckSignature(data []byte, address common.Address, sig []byte) error {
	//1. Keccak data
	hashData := crypto.Keccak256([]byte(data))
	//2. Recover public key
	pubkey, err := crypto.Ecrecover(hashData, sig)
	if err != nil {
		log.Error("CheckSignature", "error", err)
		return err
	}
	//3. Compare derived addresses
	signer := publicKey2Addr(string(pubkey))
	if bytes.Compare(signer.Bytes(), address.Bytes()) != 0 {
		return pbft.ErrInvalidSignature
	}
	return nil
}

func (sb *simpleBackend) UpdateState(state *pbft.State) {
	sb.consensusState = state
}

func (sb *simpleBackend) initPeerSet() {
	currentBlock := sb.blockchain.CurrentBlock()
	addrs := getValidatorSet(currentBlock.Header())
	vals := make([]pbft.Peer, len(addrs))
	for i, addr := range addrs {
		vals[i] = &peer{
			id:      uint64(i),
			address: addr,
		}
	}
	sb.peerSet = backends.NewPeerSet(vals)

	// update self public key
	pub := string(crypto.FromECDSAPub(&sb.privateKey.PublicKey))
	if peer := sb.updatePeerPublicKey(pub); peer != nil {
		sb.id = peer.ID()
	}
}

func (sb *simpleBackend) updatePeerPublicKey(pubKey string) pbft.Peer {
	// get peer by address
	addr := publicKey2Addr(pubKey)
	peer := sb.peerSet.GetByAddress(addr)
	// update public key
	if peer != nil {
		peer.SetPublicKey(pubKey)
		return peer
	}
	return nil
}

func getValidatorSet(block *types.Header) []common.Address {
	// get validator address from block header
	addrs := make([]common.Address, (len(block.Extra)-extraVanity-extraSeal)/common.AddressLength)
	for i := 0; i < len(addrs); i++ {
		copy(addrs[i][:], block.Extra[extraVanity+i*common.AddressLength:])
	}
	return addrs
}

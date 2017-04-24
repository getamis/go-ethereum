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
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/pbft"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
)

// VerifyHeader checks whether a header conforms to the consensus rules of a
// given engine. Verifying the seal may be done optionally here, or explicitly
// via the VerifySeal method.
func (sb *simpleBackend) VerifyHeader(chain consensus.ChainReader, header *types.Header, seal bool) error {
	return nil
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications (the order is that of
// the input slice).
func (sb *simpleBackend) VerifyHeaders(chain consensus.ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	results := make(chan error, len(headers))
	go func() {
		for i := range headers {
			results <- sb.VerifyHeader(chain, headers[i], seals[i])
		}
	}()
	return make(chan struct{}), make(chan error, len(headers))
}

// VerifyUncles verifies that the given block's uncles conform to the consensus
// rules of a given engine.
func (sb *simpleBackend) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	return nil
}

// VerifySeal checks whether the crypto seal on a header is valid according to
// the consensus rules of the given engine.
func (sb *simpleBackend) VerifySeal(chain consensus.ChainReader, header *types.Header) error {
	return nil
}

// Prepare initializes the consensus fields of a block header according to the
// rules of a particular engine. The changes are executed inline.
func (sb *simpleBackend) Prepare(chain consensus.ChainReader, header *types.Header) error {
	return nil
}

// Finalize runs any post-transaction state modifications (e.g. block rewards)
// and assembles the final block.
//
// Note, the block header and state database might be updated to reflect any
// consensus rules that happen at finalization (e.g. block rewards).
func (sb *simpleBackend) Finalize(chain consensus.ChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction,
	uncles []*types.Header, receipts []*types.Receipt) (*types.Block, error) {
	return types.NewBlockWithHeader(header), nil
}

// Seal generates a new block for the given input block with the local miner's
// seal place on top.
func (sb *simpleBackend) Seal(chain consensus.ChainReader, block *types.Block, stop <-chan struct{}) (*types.Block, error) {
	return block, nil
}

// APIs returns the RPC APIs this consensus engine provides.
func (sb *simpleBackend) APIs(chain consensus.ChainReader) []rpc.API {
	return []rpc.API{{
		Namespace: "pbft",
		Version:   "1.0",
		Service:   &API{chain: chain, backend: sb},
		Public:    true,
	}}
}

func (sb *simpleBackend) AddPeer(peerID string, publicKey *ecdsa.PublicKey) {
	peer := newPeer(peerID, publicKey)
	// check is validator
	if val := sb.valSet.GetByAddress(peer.Address()); val != nil {
		// add to peer set
		sb.peerSet.Add(peer)
		// post connection event to pbft core
		go sb.pbftEventMux.Post(pbft.ConnectionEvent{
			ID: val.ID(),
		})
	}
}

func (sb *simpleBackend) RemovePeer(peerID string) {
	sb.peerSet.Remove(peerID)
}

func (sb *simpleBackend) HandleMsg(peerID string, data []byte) {
	peer := sb.peerSet.Get(peerID)
	if peer == nil {
		return
	}

	msgEvent, err := Decode(data)
	if err != nil {
		return
	}

	if val := sb.valSet.GetByAddress(peer.Address()); val != nil {
		go sb.pbftEventMux.Post(*msgEvent)
	}
}

func (sb *simpleBackend) Start(chain consensus.ChainReader) {
	sb.initValidatorSet(chain)
	sb.core.Start()
}

func (sb *simpleBackend) Stop() {
	sb.core.Stop()
}

func (sb *simpleBackend) initValidatorSet(chain consensus.ChainReader) {
	currentHeader := chain.CurrentHeader()
	addrs := getValidatorSet(currentHeader)
	vals := make([]*pbft.Validator, len(addrs))
	for i, addr := range addrs {
		vals[i] = pbft.NewValidator(uint64(i), addr)
	}
	sb.valSet = pbft.NewValidatorSet(vals)

	// FIXME: self should be the one of valifators
	// update self id
	// get peer by address
	addr := crypto.PubkeyToAddress(sb.privateKey.PublicKey)
	privVal := sb.valSet.GetByAddress(addr)
	// update validator id
	if privVal != nil {
		sb.id = privVal.ID()
	}
}

func getValidatorSet(block *types.Header) []common.Address {
	// get validator address from block header
	addrs := make([]common.Address, (len(block.Extra)-extraVanity-extraSeal)/common.AddressLength)
	for i := 0; i < len(addrs); i++ {
		copy(addrs[i][:], block.Extra[extraVanity+i*common.AddressLength:])
	}
	return addrs
}

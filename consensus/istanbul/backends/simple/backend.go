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
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/istanbul"
	istanbulCore "github.com/ethereum/go-ethereum/consensus/istanbul/core"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
)

// New creates an Ethereum backend for Istanbul core engine.
func New(config *istanbul.Config, eventMux *event.TypeMux, privateKey *ecdsa.PrivateKey, db ethdb.Database) consensus.Istanbul {
	backend := &simpleBackend{
		config:           config,
		eventMux:         eventMux,
		istanbulEventMux: new(event.TypeMux),
		privateKey:       privateKey,
		address:          crypto.PubkeyToAddress(privateKey.PublicKey),
		logger:           log.New("backend", "simple"),
		db:               db,
		commitCh:         make(chan common.Hash, 1),
	}
	return backend
}

// ----------------------------------------------------------------------------

type simpleBackend struct {
	config           *istanbul.Config
	valSet           istanbul.ValidatorSet
	eventMux         *event.TypeMux
	istanbulEventMux *event.TypeMux
	privateKey       *ecdsa.PrivateKey
	address          common.Address
	core             istanbulCore.Engine
	logger           log.Logger
	quitSync         chan struct{}
	db               ethdb.Database
	timeout          uint64
	chain            consensus.ChainReader
	inserter         func(block *types.Block) error

	// the channels for istanbul engine notifications
	commitCh          chan common.Hash
	proposedBlockHash common.Hash
	sealMu            sync.Mutex

	// Current list of candidates we are pushing
	candidates map[common.Address]bool
	// Protects the signer fields
	candidatesLock sync.RWMutex
}

// Address implements istanbul.Backend.Address
func (sb *simpleBackend) Address() common.Address {
	return sb.address
}

// Validators implements istanbul.Backend.Validators
func (sb *simpleBackend) Validators() istanbul.ValidatorSet {
	return sb.valSet
}

func (sb *simpleBackend) Send(payload []byte, target common.Address) error {
	go sb.eventMux.Post(istanbul.ConsensusDataEvent{
		Target: target,
		Data:   payload,
	})
	return nil
}

// Broadcast implements istanbul.Backend.Send
func (sb *simpleBackend) Broadcast(payload []byte) error {
	for _, val := range sb.valSet.List() {
		if val.Address() == sb.Address() {
			// send to self
			pbftMsg := istanbul.MessageEvent{
				Payload: payload,
			}
			go sb.istanbulEventMux.Post(pbftMsg)

		} else {
			// send to other peers
			sb.Send(payload, val.Address())
		}
	}
	return nil
}

// Commit implements istanbul.Backend.Commit
func (sb *simpleBackend) Commit(proposal istanbul.Proposal) error {
	// Check if the proposal is a valid block
	block := &types.Block{}
	block, ok := proposal.(*types.Block)
	if !ok {
		sb.logger.Error("Invalid proposal, %v", proposal)
		return errInvalidProposal
	}

	sb.logger.Info("Committed", "address", sb.Address(), "hash", proposal.Hash(), "number", proposal.Number().Uint64())
	// - if the proposed and committed blocks are the same, send the proposed hash
	//   to commit channel, which is being watched inside the engine.Seal() function.
	// - otherwise, we try to insert the block.
	// -- if success, the ChainHeadEvent event will be broadcasted, try to build
	//    the next block and the previous Seal() will be stopped.
	// -- otherwise, a error will be returned and a round change event will be fired.
	if sb.proposedBlockHash == block.Hash() {
		// feed block hash to Seal() and wait the Seal() result
		sb.commitCh <- block.Hash()
		// TODO: how do we check the block is inserted correctly?
		return nil
	}
	return sb.inserter(block)
}

// NextRound will broadcast ChainHeadEvent to trigger next seal()
func (sb *simpleBackend) NextRound() error {
	header := sb.chain.CurrentHeader()
	sb.logger.Debug("NextRound", "address", sb.Address(), "current_hash", header.Hash(), "current_number", header.Number)
	go sb.eventMux.Post(core.ChainHeadEvent{})
	return nil
}

// EventMux implements istanbul.Backend.EventMux
func (sb *simpleBackend) EventMux() *event.TypeMux {
	return sb.istanbulEventMux
}

// Verify implements istanbul.Backend.Verify
func (sb *simpleBackend) Verify(proposal istanbul.Proposal) error {
	// Check if the proposal is a valid block
	block := &types.Block{}
	block, ok := proposal.(*types.Block)
	if !ok {
		sb.logger.Error("Invalid proposal, %v", proposal)
		return errInvalidProposal
	}
	// verify the header of proposed block
	return sb.VerifyHeader(sb.chain, block.Header(), false)
}

// Sign implements istanbul.Backend.Sign
func (sb *simpleBackend) Sign(data []byte) ([]byte, error) {
	hashData := crypto.Keccak256([]byte(data))
	return crypto.Sign(hashData, sb.privateKey)
}

// CheckSignature implements istanbul.Backend.CheckSignature
func (sb *simpleBackend) CheckSignature(data []byte, address common.Address, sig []byte) error {
	signer, err := getSignatureAddress(data, sig)
	if err != nil {
		log.Error("Failed to get signer address", "err", err)
		return err
	}
	// Compare derived addresses
	if signer != address {
		return errInvalidSignature
	}
	return nil
}

// CheckValidatorSignature implements istanbul.Backend.CheckValidatorSignature
func (sb *simpleBackend) CheckValidatorSignature(data []byte, sig []byte) (common.Address, error) {
	// 1. Get signature address
	signer, err := getSignatureAddress(data, sig)
	if err != nil {
		log.Error("Failed to get signer address", "err", err)
		return common.Address{}, err
	}

	// 2. Check validator
	if _, val := sb.valSet.GetByAddress(signer); val != nil {
		return val.Address(), nil
	}

	return common.Address{}, istanbul.ErrUnauthorizedAddress
}

// get the signer address from the signature
func getSignatureAddress(data []byte, sig []byte) (common.Address, error) {
	// 1. Keccak data
	hashData := crypto.Keccak256([]byte(data))
	// 2. Recover public key
	pubkey, err := crypto.SigToPub(hashData, sig)
	if err != nil {
		return common.Address{}, err
	}
	return crypto.PubkeyToAddress(*pubkey), nil
}

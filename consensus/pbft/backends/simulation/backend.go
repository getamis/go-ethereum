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
	"bytes"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/pbft"
	"github.com/ethereum/go-ethereum/consensus/pbft/backends/simple"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
)

const (
	extraVanity = 32 // Fixed number of extra-data prefix bytes reserved for signer vanity
	extraSeal   = 65 // Fixed number of extra-data suffix bytes reserved for signer seal

	PBFTMsg = 0x11
)

func New(nodeKey *NodeKey, genesis *core.Genesis) *ProtocolManager {
	eventMux := new(event.TypeMux)
	memDB, _ := ethdb.NewMemDatabase()
	backend := simple.New(3000, eventMux, nodeKey.PrivateKey(), memDB)
	p := newPeer(nodeKey)
	pm := &ProtocolManager{
		rwMutex:  new(sync.RWMutex),
		backend:  backend,
		db:       memDB,
		chain:    newTestBlockChain(genesis, eventMux, backend, memDB),
		eventMux: eventMux,
		me:       p,
		peers:    make(map[string]*peer),
		logger:   log.New("backend", "simulated", "id", p.ID()),
	}

	return pm
}

// ----------------------------------------------------------------------------

type ProtocolManager struct {
	rwMutex *sync.RWMutex

	backend  consensus.PBFT
	db       ethdb.Database
	chain    *core.BlockChain
	eventMux *event.TypeMux
	eventSub *event.TypeMuxSubscription
	me       *peer
	peers    map[string]*peer
	logger   log.Logger
}

func (pm *ProtocolManager) Start() {
	pm.eventSub = pm.eventMux.Subscribe(
		pbft.ConsensusDataEvent{},
		// XXX: Which event should we listen? core.ChainHeadEvent or core.ChainEvent?
		core.ChainHeadEvent{})
	go pm.consensusEventLoop()

	pm.backend.Start(pm.chain, pm.commitBlock)
	go pm.handlePeerMessage()
}

func (pm *ProtocolManager) Stop() {
	pm.me.Close()
	pm.backend.Stop()
	pm.eventSub.Unsubscribe()
}

func (pm *ProtocolManager) TryNewRequest() {
	// try to make next block
	pm.newRequest(pm.chain.Genesis())
}

func (pm *ProtocolManager) SelfPeer() *peer {
	return pm.me
}

func (pm *ProtocolManager) AddPeer(p *peer) {
	pm.rwMutex.Lock()
	defer pm.rwMutex.Unlock()
	if p.ID() != pm.me.ID() {
		pm.peers[p.ID()] = p
		pm.backend.AddPeer(p.ID(), p.PublicKey())
	}
}

func (pm *ProtocolManager) handlePeerMessage() {
	for {
		payload, err := pm.readPeerMessage()
		if err != nil {
			return
		}

		// FIXME: pass first peer id for test, the real source is hidden in payload
		pm.rwMutex.RLock()
		var randP *peer
		for _, p := range pm.peers {
			randP = p
			break
		}
		pm.rwMutex.RUnlock()
		pm.backend.HandleMsg(randP.ID(), payload)
	}
}

func (pm *ProtocolManager) readPeerMessage() ([]byte, error) {
	m, err := pm.me.ReadMsg()
	if err != nil {
		pm.logger.Error("Failed to ReadMsg", "error", err)
		return nil, err
	}
	defer m.Discard()

	var payload []byte
	err = m.Decode(&payload)
	if err != nil {
		pm.logger.Error("Failed to read payload", "error", err, "msg", m)
		return nil, err
	}
	return payload, nil
}

func (pm *ProtocolManager) consensusEventLoop() {
	for obj := range pm.eventSub.Chan() {
		switch ev := obj.Data.(type) {
		case pbft.ConsensusDataEvent:
			pm.sendEvent(ev)
		// After block insertion, we post a CheckpointEvent and make a new request
		case core.ChainHeadEvent:
			go pm.backend.NewChainHead(ev.Block)
			go pm.newRequest(pm.chain.CurrentBlock())
		}
	}
}

func (pm *ProtocolManager) sendEvent(event pbft.ConsensusDataEvent) {
	pm.rwMutex.RLock()
	defer pm.rwMutex.RUnlock()

	p := pm.peers[event.PeerID]
	if p == nil {
		return
	}
	p2p.Send(p, PBFTMsg, event.Data)
}

func (pm *ProtocolManager) commitBlock(block *types.Block) error {
	_, err := pm.chain.InsertChain([]*types.Block{block})
	return err
}

func (pm *ProtocolManager) newRequest(lastBlock *types.Block) {
	block := pm.makeBlock(lastBlock)
	// only proposer can make block; otherwise, will get nil
	if block == nil {
		return
	}

	// proposer gets new block after Seal and commit it directly
	// non-proposer validators get new block by receiving ConsensusCommitBlockEvent
	newBlock, err := pm.backend.Seal(pm.chain, block, nil)
	if newBlock != nil && err == nil {
		pm.commitBlock(newBlock)
	}
}

func (pm *ProtocolManager) makeBlock(parent *types.Block) *types.Block {
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     parent.Number().Add(parent.Number(), common.Big1),
		GasLimit:   core.CalcGasLimit(parent),
		GasUsed:    new(big.Int),
		Extra:      parent.Extra(),
		Time:       big.NewInt(time.Now().Unix()),
	}

	if err := pm.backend.Prepare(pm.chain, header); err != nil {
		return nil
	}

	state, err := pm.chain.StateAt(parent.Root())
	if err != nil {
		return nil
	}

	block, err := pm.backend.Finalize(pm.chain, header, state, nil, nil, nil)
	if err != nil {
		return nil
	}

	return block
}

func newTestBlockChain(genesis *core.Genesis, eventMux *event.TypeMux, engine consensus.Engine, db ethdb.Database) *core.BlockChain {
	genesis.MustCommit(db)
	blockchain, err := core.NewBlockChain(db, genesis.Config, engine, eventMux, vm.Config{})
	if err != nil {
		panic(err)
	}
	return blockchain
}

func AppendValidators(genesis *core.Genesis, addrs []common.Address) {
	if len(genesis.ExtraData) < extraVanity {
		genesis.ExtraData = append(genesis.ExtraData, bytes.Repeat([]byte{0x00}, extraVanity)...)
	}
	genesis.ExtraData = genesis.ExtraData[:extraVanity]

	for _, addr := range addrs {
		genesis.ExtraData = append(genesis.ExtraData, addr[:]...)
	}
	genesis.ExtraData = append(genesis.ExtraData, make([]byte, extraSeal)...)
}

// Copyright 2017 The go-ethereum Authors
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

package eth

import (
	"fmt"

	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/istanbul"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/params"
)

const (
	// istanbul is compatible with eth63 protocol
	istanbulName           = "istanbul"
	IstanbulVersion        = 64
	istanbulProtocolLength = 18

	IstanbulMsg = 0x11
)

type istanbulProtocolManager struct {
	*protocolManager

	engine   consensus.Istanbul
	eventSub *event.TypeMuxSubscription
}

func newIstanbulProtocolManager(config *params.ChainConfig, mode downloader.SyncMode, networkId uint64, maxPeers int, mux *event.TypeMux, txpool txPool, engine consensus.Istanbul, blockchain *core.BlockChain, chaindb ethdb.Database) (*istanbulProtocolManager, error) {
	// Create eth63 protocol manager
	defaultManager, err := newProtocolManager(config, mode, networkId, maxPeers, mux, txpool, engine, blockchain, chaindb)
	if err != nil {
		return nil, err
	}

	// Create the istanbul protocol manager
	manager := &istanbulProtocolManager{
		protocolManager: defaultManager,
		engine:          engine,
	}

	// Support only Istanbul protocol
	manager.SubProtocols = []p2p.Protocol{
		p2p.Protocol{
			Name:    istanbulName,
			Version: IstanbulVersion,
			Length:  istanbulProtocolLength,
			Run: func(p *p2p.Peer, rw p2p.MsgReadWriter) error {
				peer := manager.newPeer(int(IstanbulVersion), p, rw)
				select {
				case manager.newPeerCh <- peer:
					manager.wg.Add(1)
					defer manager.wg.Done()
					return manager.handle(peer, manager.handleMsg)
				case <-manager.quitSync:
					return p2p.DiscQuitting
				}
			},
			NodeInfo: func() interface{} {
				return manager.NodeInfo()
			},
			PeerInfo: func(id discover.NodeID) interface{} {
				if p := manager.peers.Peer(fmt.Sprintf("%x", id[:8])); p != nil {
					return p.Info()
				}
				return nil
			},
		},
	}

	return manager, nil
}

func (pm *istanbulProtocolManager) Start() {
	// Subscribe required events
	pm.eventSub = pm.eventMux.Subscribe(istanbul.ConsensusDataEvent{}, core.ChainHeadEvent{}, istanbul.NewCommittedEvent{})
	go pm.eventLoop()
	pm.protocolManager.Start()
}

func (pm *istanbulProtocolManager) Stop() {
	log.Info("Stopping Ethereum protocol")
	pm.protocolManager.Stop()
	pm.eventSub.Unsubscribe() // quits eventLoop
}

// handleMsg handles Istanbul related consensus messages or
// fallback to default procotol manager's handler
func (pm *istanbulProtocolManager) handleMsg(p *peer, msg p2p.Msg) error {
	// Handle Istanbul messages
	switch {
	case msg.Code == IstanbulMsg:
		pubKey, err := p.ID().Pubkey()
		if err != nil {
			return err
		}
		var data []byte
		if err := msg.Decode(&data); err != nil {
			return errResp(ErrDecode, "msg %v: %v", msg, err)
		}
		return pm.engine.HandleMsg(pubKey, data)
	default:
		// Invoke default protocol manager's message handler
		return pm.protocolManager.handleMsg(p, msg)
	}
}

// event loop for Istanbul
func (pm *istanbulProtocolManager) eventLoop() {
	for obj := range pm.eventSub.Chan() {
		switch ev := obj.Data.(type) {
		case istanbul.ConsensusDataEvent:
			pm.sendEvent(ev)
		case istanbul.NewCommittedEvent:
			pm.BroadcastBlock(ev.Block, false)
		case core.ChainHeadEvent:
			pm.newHead(ev)
		}
	}
}

// sendEvent sends a p2p message with given data to a peer
func (pm *istanbulProtocolManager) sendEvent(event istanbul.ConsensusDataEvent) {
	for _, p := range pm.peers.Peers() {
		pubKey, err := p.ID().Pubkey()
		if err != nil {
			continue
		}
		addr := crypto.PubkeyToAddress(*pubKey)
		if event.Targets[addr] {
			p2p.Send(p.rw, IstanbulMsg, event.Data)
			delete(event.Targets, addr)
			if len(event.Targets) == 0 {
				return
			}
		}
	}
}

func (pm *istanbulProtocolManager) newHead(event core.ChainHeadEvent) {
	block := event.Block
	if block != nil {
		pm.engine.NewChainHead(block)
	}
}

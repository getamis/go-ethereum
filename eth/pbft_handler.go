// Copyright 2015 The go-ethereum Authors
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
	"encoding/json"
	"fmt"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/misc"
	"github.com/ethereum/go-ethereum/consensus/pbft"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/eth/fetcher"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	// support eth/63 protocol
	pbftName           = "pbft"
	pbftVersion        = 64
	pbftProtocolLength = 18

	PBFTMsg = 0x11
)

type pbftProtocolManager struct {
	*protocolManager

	engine   consensus.PBFT
	eventSub *event.TypeMuxSubscription
}

// PBFTEvent is posted
type PBFTEvent struct {
	// peer public key
	PeerPublicKey string
	// PBFT message data
	Data []byte
}

func newPBFTProtocolManager(config *params.ChainConfig, mode downloader.SyncMode, networkId uint64, maxPeers int, mux *event.TypeMux, txpool txPool, engine consensus.PBFT, blockchain *core.BlockChain, chaindb ethdb.Database) (*pbftProtocolManager, error) {
	// Create the protocol manager with the base fields
	m := &protocolManager{
		networkId:   networkId,
		eventMux:    mux,
		txpool:      txpool,
		blockchain:  blockchain,
		chaindb:     chaindb,
		chainconfig: config,
		maxPeers:    maxPeers,
		peers:       newPeerSet(),
		newPeerCh:   make(chan *peer),
		noMorePeers: make(chan struct{}),
		txsyncCh:    make(chan *txsync),
		quitSync:    make(chan struct{}),
	}

	// Create the pbft protocol manager
	manager := &pbftProtocolManager{
		protocolManager: m,
		engine:          engine,
	}

	// Figure out whether to allow fast sync or not
	if mode == downloader.FastSync && blockchain.CurrentBlock().NumberU64() > 0 {
		log.Warn("Blockchain not empty, fast sync disabled")
		mode = downloader.FullSync
	}
	if mode == downloader.FastSync {
		manager.fastSync = uint32(1)
	}

	// Only support 1 pbft protocol
	manager.SubProtocols = []p2p.Protocol{
		p2p.Protocol{
			Name:    pbftName,
			Version: pbftVersion,
			Length:  pbftProtocolLength,
			Run: func(p *p2p.Peer, rw p2p.MsgReadWriter) error {
				peer := manager.newPeer(int(pbftVersion), p, rw)
				select {
				case manager.newPeerCh <- peer:
					manager.wg.Add(1)
					defer manager.wg.Done()
					return manager.handle(peer)
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

	if len(manager.SubProtocols) == 0 {
		return nil, errIncompatibleConfig
	}
	// Construct the different synchronisation mechanisms
	manager.downloader = downloader.New(downloader.FullSync, chaindb, manager.eventMux, blockchain.HasHeader, blockchain.HasBlockAndState, blockchain.GetHeaderByHash,
		blockchain.GetBlockByHash, blockchain.CurrentHeader, blockchain.CurrentBlock, blockchain.CurrentFastBlock, blockchain.FastSyncCommitHead,
		blockchain.GetTdByHash, blockchain.InsertHeaderChain, manager.blockchain.InsertChain, blockchain.InsertReceiptChain, blockchain.Rollback,
		manager.removePeer)

	validator := func(header *types.Header) error {
		return engine.VerifyHeader(blockchain, header, true)
	}
	heighter := func() uint64 {
		return blockchain.CurrentBlock().NumberU64()
	}
	inserter := func(blocks types.Blocks) (int, error) {
		atomic.StoreUint32(&manager.acceptTxs, 1) // Mark initial sync done on any fetcher import
		return manager.blockchain.InsertChain(blocks)
	}
	manager.fetcher = fetcher.New(blockchain.GetBlockByHash, validator, manager.BroadcastBlock, heighter, inserter, manager.removePeer)

	return manager, nil
}

func (pm *pbftProtocolManager) Start() {
	// receive the PBFT event
	pm.eventSub = pm.eventMux.Subscribe(pbft.ConsensusDataEvent{}, core.ChainHeadEvent{})
	go pm.eventLoop()
	pm.protocolManager.Start()
	pm.engine.Start(pm.protocolManager.blockchain, pm.commitBlock)
}

func (pm *pbftProtocolManager) Stop() {
	log.Info("Stopping Ethereum protocol")
	pm.engine.Stop()
	pm.protocolManager.Stop()
	pm.eventSub.Unsubscribe() // quits eventLoop
}

// handle is the callback invoked to manage the life cycle of an eth peer. When
// this function terminates, the peer is disconnected.
func (pm *pbftProtocolManager) handle(p *peer) error {
	if pm.peers.Len() >= pm.maxPeers {
		return p2p.DiscTooManyPeers
	}
	p.Log().Debug("Ethereum peer connected", "name", p.Name())

	// Execute the Ethereum handshake
	td, head, genesis := pm.blockchain.Status()
	if err := p.Handshake(pm.networkId, td, head, genesis); err != nil {
		p.Log().Debug("Ethereum handshake failed", "err", err)
		return err
	}
	if rw, ok := p.rw.(*meteredMsgReadWriter); ok {
		rw.Init(p.version)
	}
	// Register the peer locally
	if err := pm.peers.Register(p); err != nil {
		p.Log().Error("Ethereum peer registration failed", "err", err)
		return err
	}
	pubKey, err := p.ID().Pubkey()
	if err != nil {
		p.Log().Error("Ethereum peer get public key failed", "err", err)
		return err
	}
	pm.engine.AddPeer(p.id, pubKey)
	defer pm.removePeer(p.id)

	// Register the peer in the downloader. If the downloader considers it banned, we disconnect
	if err := pm.downloader.RegisterPeer(p.id, p.version, p.Head, p.RequestHeadersByHash, p.RequestHeadersByNumber, p.RequestBodies, p.RequestReceipts, p.RequestNodeData); err != nil {
		return err
	}
	// Propagate existing transactions. new transactions appearing
	// after this will be sent via broadcasts.
	pm.syncTransactions(p)

	// If we're DAO hard-fork aware, validate any remote peer with regard to the hard-fork
	if daoBlock := pm.chainconfig.DAOForkBlock; daoBlock != nil {
		// Request the peer's DAO fork header for extra-data validation
		if err := p.RequestHeadersByNumber(daoBlock.Uint64(), 1, 0, false); err != nil {
			return err
		}
		// Start a timer to disconnect if the peer doesn't reply in time
		p.forkDrop = time.AfterFunc(daoChallengeTimeout, func() {
			p.Log().Debug("Timed out DAO fork-check, dropping")
			pm.removePeer(p.id)
		})
		// Make sure it's cleaned up if the peer dies off
		defer func() {
			if p.forkDrop != nil {
				p.forkDrop.Stop()
				p.forkDrop = nil
			}
		}()
	}
	// main loop. handle incoming messages.
	for {
		if err := pm.handleMsg(p); err != nil {
			p.Log().Debug("Ethereum message handling failed", "err", err)
			return err
		}
	}
}

// handleMsg is invoked whenever an inbound message is received from a remote
// peer. The remote connection is torn down upon returning any error.
func (pm *pbftProtocolManager) handleMsg(p *peer) error {
	// Read the next message from the remote peer, and ensure it's fully consumed
	msg, err := p.rw.ReadMsg()
	if err != nil {
		return err
	}
	if msg.Size > ProtocolMaxMsgSize {
		return errResp(ErrMsgTooLarge, "%v > %v", msg.Size, ProtocolMaxMsgSize)
	}
	defer msg.Discard()

	// Handle the message depending on its contents
	switch {
	case msg.Code == StatusMsg:
		// Status messages should never arrive after the handshake
		return errResp(ErrExtraStatusMsg, "uncontrolled status message")

	// Block header query, collect the requested headers and reply
	case msg.Code == GetBlockHeadersMsg:
		// Decode the complex header query
		var query getBlockHeadersData
		if err := msg.Decode(&query); err != nil {
			return errResp(ErrDecode, "%v: %v", msg, err)
		}
		hashMode := query.Origin.Hash != (common.Hash{})

		// Gather headers until the fetch or network limits is reached
		var (
			bytes   common.StorageSize
			headers []*types.Header
			unknown bool
		)
		for !unknown && len(headers) < int(query.Amount) && bytes < softResponseLimit && len(headers) < downloader.MaxHeaderFetch {
			// Retrieve the next header satisfying the query
			var origin *types.Header
			if hashMode {
				origin = pm.blockchain.GetHeaderByHash(query.Origin.Hash)
			} else {
				origin = pm.blockchain.GetHeaderByNumber(query.Origin.Number)
			}
			if origin == nil {
				break
			}
			number := origin.Number.Uint64()
			headers = append(headers, origin)
			bytes += estHeaderRlpSize

			// Advance to the next header of the query
			switch {
			case query.Origin.Hash != (common.Hash{}) && query.Reverse:
				// Hash based traversal towards the genesis block
				for i := 0; i < int(query.Skip)+1; i++ {
					if header := pm.blockchain.GetHeader(query.Origin.Hash, number); header != nil {
						query.Origin.Hash = header.ParentHash
						number--
					} else {
						unknown = true
						break
					}
				}
			case query.Origin.Hash != (common.Hash{}) && !query.Reverse:
				// Hash based traversal towards the leaf block
				var (
					current = origin.Number.Uint64()
					next    = current + query.Skip + 1
				)
				if next <= current {
					infos, _ := json.MarshalIndent(p.Peer.Info(), "", "  ")
					p.Log().Warn("GetBlockHeaders skip overflow attack", "current", current, "skip", query.Skip, "next", next, "attacker", infos)
					unknown = true
				} else {
					if header := pm.blockchain.GetHeaderByNumber(next); header != nil {
						if pm.blockchain.GetBlockHashesFromHash(header.Hash(), query.Skip+1)[query.Skip] == query.Origin.Hash {
							query.Origin.Hash = header.Hash()
						} else {
							unknown = true
						}
					} else {
						unknown = true
					}
				}
			case query.Reverse:
				// Number based traversal towards the genesis block
				if query.Origin.Number >= query.Skip+1 {
					query.Origin.Number -= (query.Skip + 1)
				} else {
					unknown = true
				}

			case !query.Reverse:
				// Number based traversal towards the leaf block
				query.Origin.Number += (query.Skip + 1)
			}
		}
		return p.SendBlockHeaders(headers)

	case msg.Code == BlockHeadersMsg:
		// A batch of headers arrived to one of our previous requests
		var headers []*types.Header
		if err := msg.Decode(&headers); err != nil {
			return errResp(ErrDecode, "msg %v: %v", msg, err)
		}
		// If no headers were received, but we're expending a DAO fork check, maybe it's that
		if len(headers) == 0 && p.forkDrop != nil {
			// Possibly an empty reply to the fork header checks, sanity check TDs
			verifyDAO := true

			// If we already have a DAO header, we can check the peer's TD against it. If
			// the peer's ahead of this, it too must have a reply to the DAO check
			if daoHeader := pm.blockchain.GetHeaderByNumber(pm.chainconfig.DAOForkBlock.Uint64()); daoHeader != nil {
				if _, td := p.Head(); td.Cmp(pm.blockchain.GetTd(daoHeader.Hash(), daoHeader.Number.Uint64())) >= 0 {
					verifyDAO = false
				}
			}
			// If we're seemingly on the same chain, disable the drop timer
			if verifyDAO {
				p.Log().Debug("Seems to be on the same side of the DAO fork")
				p.forkDrop.Stop()
				p.forkDrop = nil
				return nil
			}
		}
		// Filter out any explicitly requested headers, deliver the rest to the downloader
		filter := len(headers) == 1
		if filter {
			// If it's a potential DAO fork check, validate against the rules
			if p.forkDrop != nil && pm.chainconfig.DAOForkBlock.Cmp(headers[0].Number) == 0 {
				// Disable the fork drop timer
				p.forkDrop.Stop()
				p.forkDrop = nil

				// Validate the header and either drop the peer or continue
				if err := misc.VerifyDAOHeaderExtraData(pm.chainconfig, headers[0]); err != nil {
					p.Log().Debug("Verified to be on the other side of the DAO fork, dropping")
					return err
				}
				p.Log().Debug("Verified to be on the same side of the DAO fork")
				return nil
			}
			// Irrelevant of the fork checks, send the header to the fetcher just in case
			headers = pm.fetcher.FilterHeaders(headers, time.Now())
		}
		if len(headers) > 0 || !filter {
			err := pm.downloader.DeliverHeaders(p.id, headers)
			if err != nil {
				log.Debug("Failed to deliver headers", "err", err)
			}
		}

	case msg.Code == GetBlockBodiesMsg:
		// Decode the retrieval message
		msgStream := rlp.NewStream(msg.Payload, uint64(msg.Size))
		if _, err := msgStream.List(); err != nil {
			return err
		}
		// Gather blocks until the fetch or network limits is reached
		var (
			hash   common.Hash
			bytes  int
			bodies []rlp.RawValue
		)
		for bytes < softResponseLimit && len(bodies) < downloader.MaxBlockFetch {
			// Retrieve the hash of the next block
			if err := msgStream.Decode(&hash); err == rlp.EOL {
				break
			} else if err != nil {
				return errResp(ErrDecode, "msg %v: %v", msg, err)
			}
			// Retrieve the requested block body, stopping if enough was found
			if data := pm.blockchain.GetBodyRLP(hash); len(data) != 0 {
				bodies = append(bodies, data)
				bytes += len(data)
			}
		}
		return p.SendBlockBodiesRLP(bodies)

	case msg.Code == BlockBodiesMsg:
		// A batch of block bodies arrived to one of our previous requests
		var request blockBodiesData
		if err := msg.Decode(&request); err != nil {
			return errResp(ErrDecode, "msg %v: %v", msg, err)
		}
		// Deliver them all to the downloader for queuing
		trasactions := make([][]*types.Transaction, len(request))
		uncles := make([][]*types.Header, len(request))

		for i, body := range request {
			trasactions[i] = body.Transactions
			uncles[i] = body.Uncles
		}
		// Filter out any explicitly requested bodies, deliver the rest to the downloader
		filter := len(trasactions) > 0 || len(uncles) > 0
		if filter {
			trasactions, uncles = pm.fetcher.FilterBodies(trasactions, uncles, time.Now())
		}
		if len(trasactions) > 0 || len(uncles) > 0 || !filter {
			err := pm.downloader.DeliverBodies(p.id, trasactions, uncles)
			if err != nil {
				log.Debug("Failed to deliver bodies", "err", err)
			}
		}

	case p.version >= eth63 && msg.Code == GetNodeDataMsg:
		// Decode the retrieval message
		msgStream := rlp.NewStream(msg.Payload, uint64(msg.Size))
		if _, err := msgStream.List(); err != nil {
			return err
		}
		// Gather state data until the fetch or network limits is reached
		var (
			hash  common.Hash
			bytes int
			data  [][]byte
		)
		for bytes < softResponseLimit && len(data) < downloader.MaxStateFetch {
			// Retrieve the hash of the next state entry
			if err := msgStream.Decode(&hash); err == rlp.EOL {
				break
			} else if err != nil {
				return errResp(ErrDecode, "msg %v: %v", msg, err)
			}
			// Retrieve the requested state entry, stopping if enough was found
			if entry, err := pm.chaindb.Get(hash.Bytes()); err == nil {
				data = append(data, entry)
				bytes += len(entry)
			}
		}
		return p.SendNodeData(data)

	case p.version >= eth63 && msg.Code == NodeDataMsg:
		// A batch of node state data arrived to one of our previous requests
		var data [][]byte
		if err := msg.Decode(&data); err != nil {
			return errResp(ErrDecode, "msg %v: %v", msg, err)
		}
		// Deliver all to the downloader
		if err := pm.downloader.DeliverNodeData(p.id, data); err != nil {
			log.Debug("Failed to deliver node state data", "err", err)
		}

	case p.version >= eth63 && msg.Code == GetReceiptsMsg:
		// Decode the retrieval message
		msgStream := rlp.NewStream(msg.Payload, uint64(msg.Size))
		if _, err := msgStream.List(); err != nil {
			return err
		}
		// Gather state data until the fetch or network limits is reached
		var (
			hash     common.Hash
			bytes    int
			receipts []rlp.RawValue
		)
		for bytes < softResponseLimit && len(receipts) < downloader.MaxReceiptFetch {
			// Retrieve the hash of the next block
			if err := msgStream.Decode(&hash); err == rlp.EOL {
				break
			} else if err != nil {
				return errResp(ErrDecode, "msg %v: %v", msg, err)
			}
			// Retrieve the requested block's receipts, skipping if unknown to us
			results := core.GetBlockReceipts(pm.chaindb, hash, core.GetBlockNumber(pm.chaindb, hash))
			if results == nil {
				if header := pm.blockchain.GetHeaderByHash(hash); header == nil || header.ReceiptHash != types.EmptyRootHash {
					continue
				}
			}
			// If known, encode and queue for response packet
			if encoded, err := rlp.EncodeToBytes(results); err != nil {
				log.Error("Failed to encode receipt", "err", err)
			} else {
				receipts = append(receipts, encoded)
				bytes += len(encoded)
			}
		}
		return p.SendReceiptsRLP(receipts)

	case p.version >= eth63 && msg.Code == ReceiptsMsg:
		// A batch of receipts arrived to one of our previous requests
		var receipts [][]*types.Receipt
		if err := msg.Decode(&receipts); err != nil {
			return errResp(ErrDecode, "msg %v: %v", msg, err)
		}
		// Deliver all to the downloader
		if err := pm.downloader.DeliverReceipts(p.id, receipts); err != nil {
			log.Debug("Failed to deliver receipts", "err", err)
		}

	case msg.Code == NewBlockHashesMsg:
		var announces newBlockHashesData
		if err := msg.Decode(&announces); err != nil {
			return errResp(ErrDecode, "%v: %v", msg, err)
		}
		// Mark the hashes as present at the remote node
		for _, block := range announces {
			p.MarkBlock(block.Hash)
		}
		// Schedule all the unknown hashes for retrieval
		unknown := make(newBlockHashesData, 0, len(announces))
		for _, block := range announces {
			if !pm.blockchain.HasBlock(block.Hash) {
				unknown = append(unknown, block)
			}
		}
		for _, block := range unknown {
			pm.fetcher.Notify(p.id, block.Hash, block.Number, time.Now(), p.RequestOneHeader, p.RequestBodies)
		}

	case msg.Code == NewBlockMsg:
		// Retrieve and decode the propagated block
		var request newBlockData
		if err := msg.Decode(&request); err != nil {
			return errResp(ErrDecode, "%v: %v", msg, err)
		}
		request.Block.ReceivedAt = msg.ReceivedAt
		request.Block.ReceivedFrom = p

		// Mark the peer as owning the block and schedule it for import
		p.MarkBlock(request.Block.Hash())
		pm.fetcher.Enqueue(p.id, request.Block)

		// Assuming the block is importable by the peer, but possibly not yet done so,
		// calculate the head hash and TD that the peer truly must have.
		var (
			trueHead = request.Block.ParentHash()
			trueTD   = new(big.Int).Sub(request.TD, request.Block.Difficulty())
		)
		// Update the peers total difficulty if better than the previous
		if _, td := p.Head(); trueTD.Cmp(td) > 0 {
			p.SetHead(trueHead, trueTD)

			// Schedule a sync if above ours. Note, this will not fire a sync for a gap of
			// a singe block (as the true TD is below the propagated block), however this
			// scenario should easily be covered by the fetcher.
			currentBlock := pm.blockchain.CurrentBlock()
			if trueTD.Cmp(pm.blockchain.GetTd(currentBlock.Hash(), currentBlock.NumberU64())) > 0 {
				go pm.synchronise(p)
			}
		}

	case msg.Code == TxMsg:
		// Transactions arrived, make sure we have a valid and fresh chain to handle them
		if atomic.LoadUint32(&pm.acceptTxs) == 0 {
			break
		}
		// Transactions can be processed, parse all of them and deliver to the pool
		var txs []*types.Transaction
		if err := msg.Decode(&txs); err != nil {
			return errResp(ErrDecode, "msg %v: %v", msg, err)
		}
		for i, tx := range txs {
			// Validate and mark the remote transaction
			if tx == nil {
				return errResp(ErrDecode, "transaction %d is nil", i)
			}
			p.MarkTransaction(tx.Hash())
		}
		pm.txpool.AddBatch(txs)

	case msg.Code == PBFTMsg:
		var data []byte
		if err := msg.Decode(&data); err != nil {
			return errResp(ErrDecode, "msg %v: %v", msg, err)
		}
		pm.engine.HandleMsg(p.id, data)
	default:
		return errResp(ErrInvalidMsgCode, "%v", msg.Code)
	}
	return nil
}

// event loop for PBFT
func (pm *pbftProtocolManager) eventLoop() {
	// automatically stops if unsubscribe
	for obj := range pm.eventSub.Chan() {
		switch ev := obj.Data.(type) {
		case pbft.ConsensusDataEvent:
			pm.sendEvent(ev)
		case core.ChainHeadEvent:
			pm.newHead(ev)
		}
	}
}

// event loop for PBFT events
func (pm *pbftProtocolManager) sendEvent(event pbft.ConsensusDataEvent) {
	p := pm.peers.Peer(event.PeerID)
	if p == nil {
		log.Warn("Failed to send event to peer", "id", event.PeerID)
		return
	}
	p2p.Send(p.rw, PBFTMsg, event.Data)
}

func (pm *pbftProtocolManager) commitBlock(block *types.Block) error {
	// TODO: find a better way to handle validator insert block
	if _, err := pm.blockchain.InsertChain(types.Blocks{block}); err != nil {
		log.Debug("Block insert failed", "number", block.Number(), "hash", block.Hash(), "err", err)
		return err
	}
	// Only announce the block, not broadcast it
	go pm.BroadcastBlock(block, false)
	return nil
}

func (pm *pbftProtocolManager) newHead(event core.ChainHeadEvent) {
	block := event.Block
	if block != nil {
		pm.engine.NewChainHead(block)
	}
}

func (pm *pbftProtocolManager) removePeer(id string) {
	// peer id is equal to peer public key
	pm.engine.RemovePeer(id)
	pm.protocolManager.removePeer(id)
}

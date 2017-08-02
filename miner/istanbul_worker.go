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

package miner

import (
	"errors"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	set "gopkg.in/fatih/set.v0"
)

// worker is the main object which takes care of applying messages to the new state
type istanbulWorker struct {
	*worker
}

var (
	// errInconsistentBlocks is returned when the proposed block and current block are inconsistent
	errInconsistentBlocks = errors.New("inconsistent blocks")
)

func newIstanbulWorker(config *params.ChainConfig, engine consensus.Engine, coinbase common.Address, eth Backend, mux *event.TypeMux) *istanbulWorker {
	worker := &istanbulWorker{
		worker: &worker{
			config:         config,
			engine:         engine,
			eth:            eth,
			mux:            mux,
			chainDb:        eth.ChainDb(),
			recv:           make(chan *Result, resultQueueSize),
			chain:          eth.BlockChain(),
			proc:           eth.BlockChain().Validator(),
			possibleUncles: make(map[common.Hash]*types.Block),
			coinbase:       coinbase,
			txQueue:        make(map[common.Hash]*types.Transaction),
			agents:         make(map[Agent]struct{}),
			unconfirmed:    newUnconfirmedBlocks(eth.BlockChain(), 5),
			fullValidation: false,
		},
	}
	worker.events = worker.mux.Subscribe(NewBlockEvent{}, core.ChainSideEvent{})
	go worker.update()
	go worker.wait()

	if istanbul, ok := engine.(consensus.Istanbul); ok {
		istanbul.SetWorkerFns(worker.commitProposedWork, worker.writeProposedWork)
	}
	return worker
}

func (self *istanbulWorker) pending() (*types.Block, *state.StateDB) {
	return self.pendingBlock(), self.current.state.Copy()
}

func (self *istanbulWorker) pendingBlock() *types.Block {
	if self.current == nil {
		return nil
	}
	return self.current.Block
}

func (self *istanbulWorker) update() {
	for event := range self.events.Chan() {
		// A real event arrived, process interesting content
		switch ev := event.Data.(type) {
		case NewBlockEvent:
			self.commitNewWork()
		case core.ChainSideEvent:
			if ev.Block != nil {
				// This case should not happen
				log.Warn("Fork happens", "hash", ev.Block.Hash())
			}
		}
	}
}

func (self *istanbulWorker) commitProposedWork(block *types.Block) error {
	self.mu.Lock()
	defer self.mu.Unlock()
	self.uncleMu.Lock()
	defer self.uncleMu.Unlock()
	self.currentMu.Lock()
	defer self.currentMu.Unlock()

	tstart := time.Now()
	header := &types.Header{
		ParentHash: block.ParentHash(),
		Number:     block.Number(),
		GasLimit:   block.GasLimit(),
		GasUsed:    new(big.Int),
		Extra:      block.Extra(),
		Time:       block.Time(),
		Difficulty: block.Difficulty(),
		MixDigest:  block.MixDigest(),
		Coinbase:   block.Coinbase(),
		Nonce:      types.BlockNonce{},
		UncleHash:  block.UncleHash(),
	}
	parent := self.chain.GetBlockByHash(header.ParentHash)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	err := self.makeProposedCurrent(parent, header)
	if err != nil {
		log.Error("Failed to create mining context", "err", err)
		return err
	}
	work := self.current

	author, err := self.engine.Author(block.Header())
	if err != nil {
		return err
	}
	err = work.commitProposedTransactions(self.mux, block, self.chain, author)
	if err != nil {
		return err
	}
	work.Block, _ = self.engine.Finalize(self.chain, header, work.state, work.txs, nil, work.receipts)
	if work.Block.Hash() != block.Hash() {
		return errInconsistentBlocks
	}
	work.Block.ReceivedAt = tstart
	log.Info("Commit proposed work", "number", work.Block.Number(), "txs", work.tcount, "elapsed", common.PrettyDuration(time.Since(tstart)))
	return nil
}

func (self *istanbulWorker) writeProposedWork(block *types.Block) bool {
	if atomic.LoadInt32(&self.mining) != 1 {
		return false
	}
	self.recv <- &Result{self.current, block}
	return true
}

// makeCurrent creates a new environment for the current cycle.
func (self *worker) makeProposedCurrent(parent *types.Block, header *types.Header) error {
	state, err := self.chain.StateAt(parent.Root())
	if err != nil {
		log.Error("Failed to reset parent state", "err", err)
		return err
	}
	work := &Work{
		config:    self.config,
		signer:    types.MakeSigner(self.config, header.Number),
		state:     state,
		ancestors: set.New(),
		family:    set.New(),
		uncles:    set.New(),
		header:    header,
		createdAt: time.Now(),
	}

	// when 08 is processed ancestors contain 07 (quick block)
	for _, ancestor := range self.chain.GetBlocksFromHash(parent.Hash(), 7) {
		work.family.Add(ancestor.Hash())
		work.ancestors.Add(ancestor.Hash())
	}
	// Keep track of transactions which return errors so they can be removed
	work.tcount = 0
	self.current = work
	return nil
}

func (env *Work) commitProposedTransactions(mux *event.TypeMux, block *types.Block, bc *core.BlockChain, coinbase common.Address) error {
	gp := new(core.GasPool).AddGas(env.header.GasLimit)

	var coalescedLogs []*types.Log

	for _, tx := range block.Transactions() {
		err, logs := env.commitTransaction(tx, bc, coinbase, gp)
		if err != nil {
			return err
		}
		coalescedLogs = append(coalescedLogs, logs...)
		env.tcount++
	}

	if len(coalescedLogs) > 0 || env.tcount > 0 {
		// make a copy, the state caches the logs and these logs get "upgraded" from pending to mined
		// logs by filling in the block hash when the block was mined by the local miner. This can
		// cause a race condition if a log was "upgraded" before the PendingLogsEvent is processed.
		cpy := make([]*types.Log, len(coalescedLogs))
		for i, l := range coalescedLogs {
			cpy[i] = new(types.Log)
			*cpy[i] = *l
		}
		go func(logs []*types.Log, tcount int) {
			if len(logs) > 0 {
				mux.Post(core.PendingLogsEvent{Logs: logs})
			}
			if tcount > 0 {
				mux.Post(core.PendingStateEvent{})
			}
		}(cpy, env.tcount)
	}
	return nil
}

func (self *istanbulWorker) wait() {
	for result := range self.recv {
		atomic.AddInt32(&self.atWork, -1)

		if result == nil {
			continue
		}
		block := result.Block
		work := result.Work
		tstart := time.Now()
		work.state.CommitTo(self.chainDb, self.config.IsEIP158(block.Number()))
		stat, err := self.chain.WriteBlock(block)
		if err != nil {
			log.Error("Failed writing block to chain", "err", err)
			continue
		}
		// update block hash since it is now available and not when the receipt/log of individual transactions were created
		for _, r := range work.receipts {
			for _, l := range r.Logs {
				l.BlockHash = block.Hash()
			}
		}
		for _, log := range work.state.Logs() {
			log.BlockHash = block.Hash()
		}

		// check if canon block and write transactions
		if stat == core.CanonStatTy {
			// This puts transactions in a extra db for rpc
			core.WriteTxLookupEntries(self.chainDb, block)
			// Write map map bloom filters
			core.WriteMipmapBloom(self.chainDb, block.NumberU64(), work.receipts)
		}

		// broadcast before waiting for validation
		go func(block *types.Block, logs []*types.Log, receipts []*types.Receipt) {
			self.mux.Post(core.NewMinedBlockEvent{Block: block})
			self.mux.Post(core.ChainEvent{Block: block, Hash: block.Hash(), Logs: logs})

			if stat == core.CanonStatTy {
				self.mux.Post(core.ChainHeadEvent{Block: block})
				self.mux.Post(logs)
			}
			if err := core.WriteBlockReceipts(self.chainDb, block.Hash(), block.NumberU64(), receipts); err != nil {
				log.Warn("Failed writing block receipts", "err", err)
			}
		}(block, work.state.Logs(), work.receipts)
		log.Info("Write proposed work", "number", block.Number(), "txs", work.tcount, "elapsed", common.PrettyDuration(time.Since(tstart)))

		// Insert the block into the set of pending ones to wait for confirmations
		self.unconfirmed.Insert(block.NumberU64(), block.Hash())
	}
}

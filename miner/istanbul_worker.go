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
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

// worker is the main object which takes care of applying messages to the new state
type istanbulWorker struct {
	*worker
}

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

	return worker
}

func (self *istanbulWorker) pending() (*types.Block, *state.StateDB) {
	return nil, self.current.state.Copy()
}

func (self *istanbulWorker) pendingBlock() *types.Block {
	return nil
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

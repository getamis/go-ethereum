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
	"sync"

	"sync/atomic"

	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
)

type StartPBFTEvent struct{ *types.Block }
type EndPBFTEvent struct{ *types.Block }

type PBFTAgent struct {
	mu sync.Mutex

	workCh        chan *Work
	stop          chan struct{}
	quitCurrentOp chan struct{}
	returnCh      chan<- *Result

	chain  consensus.ChainReader
	engine consensus.Engine

	isMining int32 // isMining indicates whether the agent is currently mining

	eventMux *event.TypeMux
	pbftSub  *event.TypeMuxSubscription
	lastWrok *Work
}

func NewPBFTAgent(chain consensus.ChainReader, engine consensus.Engine, eventMux *event.TypeMux) *PBFTAgent {
	miner := &PBFTAgent{
		chain:    chain,
		engine:   engine,
		stop:     make(chan struct{}, 1),
		workCh:   make(chan *Work, 1),
		eventMux: eventMux,
	}
	return miner
}

func (self *PBFTAgent) Work() chan<- *Work            { return self.workCh }
func (self *PBFTAgent) SetReturnCh(ch chan<- *Result) { self.returnCh = ch }

func (self *PBFTAgent) Stop() {
	self.stop <- struct{}{}
	self.pbftSub.Unsubscribe()
}

func (self *PBFTAgent) Start() {
	if !atomic.CompareAndSwapInt32(&self.isMining, 0, 1) {
		return // agent already started
	}
	go self.update()
	self.pbftSub = self.eventMux.Subscribe(EndPBFTEvent{})
	go self.pbftLoop()
}

func (self *PBFTAgent) update() {
out:
	for {
		select {
		case work := <-self.workCh:
			log.Info("Agent received work")
			self.mu.Lock()
			if self.quitCurrentOp != nil {
				close(self.quitCurrentOp)
			}
			self.quitCurrentOp = make(chan struct{})
			go self.mine(work, self.quitCurrentOp)
			self.mu.Unlock()
		case <-self.stop:
			self.mu.Lock()
			if self.quitCurrentOp != nil {
				close(self.quitCurrentOp)
				self.quitCurrentOp = nil
			}
			self.mu.Unlock()
			break out
		}
	}

done:
	// Empty work channel
	for {
		select {
		case <-self.workCh:
		default:
			break done
		}
	}
	atomic.StoreInt32(&self.isMining, 0)
}

func (self *PBFTAgent) mine(work *Work, stop <-chan struct{}) {
	if result, err := self.engine.Seal(self.chain, work.Block, stop); result != nil {
		log.Info("Successfully sealed new block", "number", result.Number(), "hash", result.Hash())
		self.lastWrok = work
		// start PBFT
		self.eventMux.Post(StartPBFTEvent{result})
		//
	} else {
		if err != nil {
			log.Warn("Block sealing failed", "err", err)
		}
		self.returnCh <- nil
	}
}

func (self *PBFTAgent) GetHashRate() int64 {
	if pow, ok := self.engine.(consensus.PoW); ok {
		return int64(pow.Hashrate())
	}
	return 0
}

func (self *PBFTAgent) pbftLoop() {
	// automatically stops if unsubscribe
	for obj := range self.pbftSub.Chan() {
		switch ev := obj.Data.(type) {
		case EndPBFTEvent:
			log.Info("Try to send the result to received channel")
			self.returnCh <- &Result{self.lastWrok, ev.Block}
		}
	}
}

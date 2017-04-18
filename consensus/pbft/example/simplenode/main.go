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

package main

import (
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/pbft/backends/simulation"
	pbftCore "github.com/ethereum/go-ethereum/consensus/pbft/core"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

const (
	F = 1
	N = 3*F + 1
)

var (
	genesisBlock *types.Block
	blocks       map[common.Hash]*types.Block
)

func main() {
	glogger := log.NewGlogHandler(log.StreamHandler(os.Stderr, log.TerminalFormat(false)))
	glogger.Verbosity(log.LvlDebug)
	log.Root().SetHandler(glogger)

	var validators = make([]pbftCore.Engine, N)
	var backends = make([]*simulation.Backend, N)

	for i := 0; i < N; i++ {
		backend := simulation.NewBackend(uint64(i))
		backend.Start()
		defer backend.Stop()
		validator := pbftCore.New(backend)
		validator.Start()
		defer validator.Stop()

		validators[i] = validator
		backends[i] = backend
	}

	for i := 0; i < N; i++ {
		for j := 0; j < N; j++ {
			if i != j {
				backends[i].AddPeer(fmt.Sprintf("%v", j))
			}
		}
	}

	log.Info("Start")

	time.Sleep(3 * time.Second)

	genesisBlock, _ = core.DefaultGenesisBlock().ToBlock()
	block := genesisBlock

	blocks := make(map[common.Hash]*types.Block)
	blocks[block.Hash()] = block

	for _, backend := range backends {
		subscription := backend.EventMux().Subscribe(simulation.CommitEvent{})
		be := backend
		go func() {
			for event := range subscription.Chan() {
				switch ev := event.Data.(type) {
				case simulation.CommitEvent:
					b := blocks[common.BytesToHash(ev.Payload)]
					log.Info("Block committed", "number", b.NumberU64(), "hash", b.Hash().String(), "id", be.ID())
				}
			}
		}()
	}

	for {
		backends[0].NewRequest(block.Hash().Bytes())
		block = makeBlock(block, block.Number())
		blocks[block.Hash()] = block
		time.Sleep(1 * time.Second)
	}
}

func makeBlock(parent *types.Block, num *big.Int) *types.Block {
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     num.Add(num, common.Big1),
		GasLimit:   new(big.Int),
		GasUsed:    new(big.Int),
		Extra:      nil,
		Time:       big.NewInt(int64(time.Now().Nanosecond())),
	}

	return types.NewBlockWithHeader(header)
}

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
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

// API is a user facing RPC API to dump Istanbul state
type API struct {
	chain consensus.ChainReader
	pbft  *simpleBackend
}

// RoundState returns current state and proposer
func (api *API) RoundState() {
	state, roundState := api.pbft.core.RoundState()
	log.Info("RoundState", "sequence", roundState.Sequence, "round", roundState.Round,
		"state", state, "hash", roundState.Preprepare.Proposal.Hash(),
		"prepares", roundState.Prepares, "commits", roundState.Commits,
		"checkpoint", roundState.Checkpoints)
}

// Backlog returns backlogs
func (api *API) Backlog() {
	backlog := api.pbft.core.Backlog()
	logs := make([]string, 0, len(backlog))
	for validator, q := range backlog {
		logs = append(logs, fmt.Sprintf("{%v, %v}", validator, q.Size()))
	}
	log.Info("Backlog", "logs", fmt.Sprintf("[%v]", strings.Join(logs, ", ")))
}

// GetSnapshot retrieves the state snapshot at a given block.
func (api *API) GetSnapshot(number *rpc.BlockNumber) (*Snapshot, error) {
	// Retrieve the requested block number (or current if none requested)
	var header *types.Header
	if number == nil || *number == rpc.LatestBlockNumber {
		header = api.chain.CurrentHeader()
	} else {
		header = api.chain.GetHeaderByNumber(uint64(number.Int64()))
	}
	// Ensure we have an actually valid block and return its snapshot
	if header == nil {
		return nil, errUnknownBlock
	}
	return api.pbft.snapshot(api.chain, header.Number.Uint64(), header.Hash(), nil)
}

// GetSnapshotAtHash retrieves the state snapshot at a given block.
func (api *API) GetSnapshotAtHash(hash common.Hash) (*Snapshot, error) {
	header := api.chain.GetHeaderByHash(hash)
	if header == nil {
		return nil, errUnknownBlock
	}
	return api.pbft.snapshot(api.chain, header.Number.Uint64(), header.Hash(), nil)
}

// GetSigners retrieves the list of authorized signers at the specified block.
func (api *API) GetSigners(number *rpc.BlockNumber) ([]common.Address, error) {
	// Retrieve the requested block number (or current if none requested)
	var header *types.Header
	if number == nil || *number == rpc.LatestBlockNumber {
		header = api.chain.CurrentHeader()
	} else {
		header = api.chain.GetHeaderByNumber(uint64(number.Int64()))
	}
	// Ensure we have an actually valid block and return the signers from its snapshot
	if header == nil {
		return nil, errUnknownBlock
	}
	snap, err := api.pbft.snapshot(api.chain, header.Number.Uint64(), header.Hash(), nil)
	if err != nil {
		return nil, err
	}
	return snap.signers(), nil
}

// GetSignersAtHash retrieves the state snapshot at a given block.
func (api *API) GetSignersAtHash(hash common.Hash) ([]common.Address, error) {
	header := api.chain.GetHeaderByHash(hash)
	if header == nil {
		return nil, errUnknownBlock
	}
	snap, err := api.pbft.snapshot(api.chain, header.Number.Uint64(), header.Hash(), nil)
	if err != nil {
		return nil, err
	}
	return snap.signers(), nil
}

// Candidates returns the current candidates the node tries to uphold and vote on.
func (api *API) Candidates() map[common.Address]bool {
	api.pbft.candidatesLock.RLock()
	defer api.pbft.candidatesLock.RUnlock()

	proposals := make(map[common.Address]bool)
	for address, auth := range api.pbft.candidates {
		proposals[address] = auth
	}
	return proposals
}

// Propose injects a new authorization candidate that the signer will attempt to
// push through.
func (api *API) Propose(address common.Address, auth bool) {
	api.pbft.candidatesLock.Lock()
	defer api.pbft.candidatesLock.Unlock()

	api.pbft.candidates[address] = auth
}

// Discard drops a currently running candidate, stopping the signer from casting
// further votes (either for or against).
func (api *API) Discard(address common.Address) {
	api.pbft.candidatesLock.Lock()
	defer api.pbft.candidatesLock.Unlock()

	delete(api.pbft.candidates, address)
}

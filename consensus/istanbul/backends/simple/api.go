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

	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/log"
)

// API is a user facing RPC API to dump Istanbul state
type API struct {
	chain   consensus.ChainReader
	backend *simpleBackend
}

// RoundState returns current state and proposer
func (api *API) RoundState() {
	state, roundState := api.backend.core.RoundState()
	p := api.backend.valSet.GetProposer().Address()
	log.Info("RoundState", "sequence", roundState.Sequence, "Round", roundState.Round,
		"state", state, "proposer", p,
		"hash", roundState.Preprepare.Proposal.Hash(),
		"prepares", roundState.Prepares, "commits", roundState.Commits,
		"checkpoint", roundState.Checkpoints)
}

// Backlog returns backlogs
func (api *API) Backlog() {
	backlog := api.backend.core.Backlog()
	logs := make([]string, 0, len(backlog))
	for validator, q := range backlog {
		logs = append(logs, fmt.Sprintf("{%v, %v}", validator, q.Size()))
	}
	log.Info("Backlog", "logs", fmt.Sprintf("[%v]", strings.Join(logs, ", ")))
}

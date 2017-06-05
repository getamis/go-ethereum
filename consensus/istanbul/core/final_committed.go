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

package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/istanbul"
)

func (c *core) handleFinalCommitted(proposal istanbul.Proposal, proposer common.Address) error {
	logger := c.logger.New("state", c.state, "number", proposal.Number(), "hash", proposal.Hash())

	// this proposal comes from consensus
	sub := c.current.Subject()
	if sub != nil &&
		proposal.Hash() == sub.Digest &&
		c.state == StateCommitted {
		logger.Trace("New proposal from consensus")

		// broadcast the checkpoint
		c.sendCheckpoint(&istanbul.Subject{
			View: &istanbul.View{
				Sequence: new(big.Int).Set(proposal.Number()),
				Round:    new(big.Int).Set(c.current.Round()),
			},
			Digest: proposal.Hash(),
		})

		// store snapshot
		c.snapshotsMu.Lock()
		c.snapshots = append(c.snapshots, c.current)
		c.snapshotsMu.Unlock()
	} else { // this proposal comes from synchronization
		logger.Trace("New proposal from synchronization")
	}

	// We're late, catch up the sequence number
	if proposal.Number().Cmp(c.current.Sequence()) >= 0 {
		// We build a stable checkpoint every 'CheckPointPeriod' proposal
		if new(big.Int).Mod(c.current.Sequence(), big.NewInt(int64(c.config.CheckPointPeriod))).Int64() == 0 {
			go c.sendEvent(buildCheckpointEvent{})
		}

		// Remember to store the proposer since we've accpeted the proposal
		c.lastProposer = proposer
		c.lastProposal = proposal
		c.startNewRound(&istanbul.View{
			Sequence: new(big.Int).Add(proposal.Number(), common.Big1),
			Round:    new(big.Int).Set(common.Big0),
		}, false)
	}

	return nil
}

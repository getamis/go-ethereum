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
	"github.com/ethereum/go-ethereum/consensus/pbft"
)

func (c *core) handleFinalCommitted(ev pbft.FinalCommittedEvent, p pbft.Validator) error {
	logger := c.logger.New("state", c.state)
	// this block is from consensus
	if c.subject != nil &&
		ev.BlockHash == c.subject.Digest &&
		c.state == StateCommitted {
		logger.Debug("handleFinalCommitted from consensus", "height", ev.BlockNumber, "hash", ev.BlockHash)
		// send out the checkpoint
		c.sendCheckpoint(&pbft.Subject{
			View: &pbft.View{
				Sequence: ev.BlockNumber,
				Round:    c.round,
			},
			Digest: ev.BlockHash,
		})
		c.snapshotsMu.Lock()
		c.snapshots = append(c.snapshots, c.current)
		c.snapshotsMu.Unlock()

	} else {
		// this block is from geth sync
		logger.Debug("handleFinalCommitted from geth sync", "height", ev.BlockNumber, "hash", ev.BlockHash)
	}

	if ev.BlockNumber.Cmp(c.sequence) >= 0 {
		// We build stable checkpoint every 100 blocks
		// FIXME: this should be passed by configuration
		if new(big.Int).Mod(c.sequence, big.NewInt(int64(c.config.CheckPointPeriod))).Int64() == 0 {
			go c.sendInternalEvent(buildCheckpointEvent{})
		}

		c.sequence = new(big.Int).Add(ev.BlockNumber, common.Big1)
		c.round = common.Big0
		c.lastProposer = ev.BlockProposer
		c.setState(StateAcceptRequest)
	}

	return nil
}

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
	"bytes"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/pbft"
)

func (c *core) handleFinalCommitted(ev pbft.FinalCommittedEvent, p pbft.Validator) error {
	logger := c.logger.New("state", c.state)
	// this block is from consensus
	if c.subject != nil &&
		bytes.Compare(ev.BlockHash.Bytes(), c.subject.Digest) == 0 &&
		c.state == StateCommitted {
		logger.Debug("handleFinalCommitted from consensus", "height", ev.BlockNumber, "hash", ev.BlockHash)
		// send out the checkpoint
		c.sendCheckpoint(&pbft.Subject{
			View: &pbft.View{
				Sequence: ev.BlockNumber,
				Round:    c.round,
			},
			Digest: ev.BlockHash.Bytes(),
		})
		c.snapshotsMu.Lock()
		c.snapshots = append(c.snapshots, c.current)
		c.snapshotsMu.Unlock()

		c.round = new(big.Int).Set(c.current.Round)
		c.sequence = new(big.Int).Set(c.current.Sequence)
		c.completed = true
		c.setState(StateAcceptRequest)
		// this block is from geth sync
	} else {
		logger.Debug("handleFinalCommitted from geth sync", "height", ev.BlockNumber, "hash", ev.BlockHash)
		// reset view number to 0
		c.round = common.Big0
		c.sequence = new(big.Int).Set(ev.BlockNumber)
		c.completed = true
		c.setState(StateAcceptRequest)
	}
	// We build stable checkpoint every 100 blocks
	// FIXME: this should be passed by configuration
	if new(big.Int).Mod(c.sequence, big.NewInt(100)).Int64() == 0 {
		go c.sendInternalEvent(buildCheckpointEvent{})
	}
	return nil
}

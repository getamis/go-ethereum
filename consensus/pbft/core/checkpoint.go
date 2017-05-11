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
	"sort"

	"github.com/ethereum/go-ethereum/consensus/pbft"
)

func (c *core) sendCheckpoint(cp *pbft.Subject) {
	logger := c.logger.New("state", c.state)
	logger.Debug("sendCheckpoint")

	newCp, err := Encode(cp)
	if err != nil {
		logger.Error("Failed to encode", "subject", cp)
		return
	}

	c.broadcast(&message{
		Code: msgCheckpoint,
		Msg:  newCp,
	})
}

func (c *core) handleCheckpoint(msg *message, src pbft.Validator) error {
	logger := c.logger.New("from", src.Address().Hex(), "state", c.state)

	var cp *pbft.Subject
	err := msg.Decode(&cp)
	if err != nil {
		logger.Error("Invalid checkpoint message", "msg", msg)
		return pbft.ErrInvalidMessage
	}
	if cp == nil {
		logger.Warn("Ignore empty checkpoint messsage")
		return pbft.ErrIgnored
	}

	var snapshot *snapshot

	logger.Debug("handleCheckpoint")

	c.snapshotsMu.Lock()
	defer c.snapshotsMu.Unlock()

	// Look for matching snapshot
	if cp.View.Sequence.Cmp(c.current.Sequence) == 0 { // current
		snapshot = c.current
	} else if cp.View.Sequence.Cmp(c.current.Sequence) < 0 { // old checkpoint
		snapshotIndex := sort.Search(len(c.snapshots),
			func(i int) bool {
				return c.snapshots[i].Sequence.Cmp(cp.View.Sequence) >= 0
			},
		)

		// If there is no such index, Search returns len(c.snapshots).
		if snapshotIndex < len(c.snapshots) {
			snapshot = c.snapshots[snapshotIndex]
		} else {
			logger.Warn("Failed to find snapshot entry", "seq", cp.View.Sequence, "current", c.current.Sequence)
			return pbft.ErrInvalidMessage
		}
	} else { // future checkpoint
		// TODO: Do we have to handle this?
		return pbft.ErrInvalidMessage
	}

	// Save to snapshot
	if err := snapshot.Checkpoints.Add(msg); err != nil {
		logger.Error("Failed to add checkpoint", "error", err)
		return err
	}

	return nil
}

func (c *core) buildStableCheckpoint() {
	var stableCheckpoint *snapshot
	stableCheckpointIndex := -1
	logger := c.logger.New("seq", c.sequence)

	c.snapshotsMu.Lock()
	for i := len(c.snapshots) - 1; i >= 0; i-- {
		snapshot := c.snapshots[i]
		if snapshot.Checkpoints.Size() > int(c.F*2) {
			stableCheckpoint = snapshot
			stableCheckpointIndex = i
			break
		}
	}

	// We found a stable checkpoint
	if stableCheckpointIndex != -1 {
		// Remove old snapshots
		c.snapshots = c.snapshots[stableCheckpointIndex+1:]
	}

	// Release the lock as soon as possible
	c.snapshotsMu.Unlock()

	logger.Debug("Stable checkpoint", "checkpoint", stableCheckpoint)

	if err := c.backend.Save(keyStableCheckpoint, stableCheckpoint); err != nil {
		logger.Crit("Failed to save stable checkpoint", "error", err)
	}
}

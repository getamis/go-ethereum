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
	"reflect"

	"github.com/ethereum/go-ethereum/consensus/pbft"
)

func (c *core) sendCommit() {
	log.Info("sendCommit", "id", c.ID())
	c.broadcast(MsgCommit, c.subject)
	c.commitMsgs[c.ID()] = c.subject
	c.processBacklog()
}

func (c *core) handleCommit(commit *pbft.Subject, src pbft.Peer) error {
	log.Info("handleCommit", "id", c.ID(), "from", src.ID())

	if c.isFutureMessage(commit.View) {
		return errFutureMessage
	}

	if err := c.verifyCommit(commit, src); err != nil {
		return err
	}

	c.commitMsgs[src.ID()] = commit
	c.processBacklog()

	// log.Info("Total commit msgs", "id", pbft.ID(), "num", len(pbft.commitMsgs))

	if int64(len(c.commitMsgs)) > 2*c.F && c.state == StatePrepared {
		// TODO: Enter checkpoint stage?

		c.state = StateCommitted
		c.backend.Commit(c.preprepareMsg.Proposal)
		c.state = StateAcceptRequest
	}

	return nil
}

func (c *core) verifyCommit(commit *pbft.Subject, src pbft.Peer) error {
	logger := log.New("id", c.ID(), "from", src.ID())

	if !reflect.DeepEqual(commit, c.subject) {
		logger.Warn("Subject not match", "expected", c.subject, "got", commit)
		return pbft.ErrSubjectNotMatched
	}

	return nil
}

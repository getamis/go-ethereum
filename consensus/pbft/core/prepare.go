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

func (c *core) sendPrepare() {
	logger := c.logger.New("state", c.state)
	logger.Debug("sendPrepare")
	c.broadcast(pbft.MsgPrepare, c.subject)
}

func (c *core) handlePrepare(prepare *pbft.Subject, src pbft.Peer) error {
	logger := c.logger.New("from", src.ID(), "state", c.state)
	logger.Debug("handlePrepare")

	if c.isFutureMessage(prepare.View) {
		return errFutureMessage
	}

	if err := c.verifyPrepare(prepare, src); err != nil {
		return err
	}

	c.acceptPrepare(prepare, src)

	// If 2f+1
	if int64(c.current.Prepares.Size()) > 2*c.F && c.state == StatePreprepared {
		c.state = StatePrepared
		c.sendCommit()
		c.processBacklog()
	}

	return nil
}

func (c *core) verifyPrepare(prepare *pbft.Subject, src pbft.Peer) error {
	logger := c.logger.New("from", src.ID(), "state", c.state)

	if prepare.View.Sequence != nil &&
		c.subject != nil &&
		prepare.View.Sequence.Cmp(c.subject.View.Sequence) < 0 {
		logger.Warn("Old message", "expected", c.subject, "got", prepare)
		return pbft.ErrOldMessage
	}

	if !reflect.DeepEqual(prepare, c.subject) {
		logger.Warn("Subject not match", "expected", c.subject, "got", prepare)
		return pbft.ErrSubjectNotMatched
	}

	return nil
}

func (c *core) acceptPrepare(prepare *pbft.Subject, src pbft.Peer) {
	logger := c.logger.New("from", src.ID(), "state", c.state)

	if _, err := c.current.Prepares.Add(prepare, src); err != nil {
		logger.Error("Failed to log prepare message", "msg", prepare, "error", err)
	}
}

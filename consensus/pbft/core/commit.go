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
	logger := c.logger.New("state", c.state)
	logger.Debug("sendCommit")
	c.broadcast(&message{
		Code: msgCommit,
		Msg:  c.subject,
	})
}

func (c *core) handleCommit(msg *message, src pbft.Validator) error {
	logger := c.logger.New("from", src.Address().Hex(), "state", c.state)
	logger.Debug("handleCommit")

	commit, ok := msg.Msg.(*pbft.Subject)
	if !ok {
		return errFailedDecodeCommit
	}

	if c.isFutureMessage(msgCommit, commit.View) {
		return errFutureMessage
	}

	if err := c.verifyCommit(commit, src); err != nil {
		return err
	}

	c.acceptCommit(commit, src)

	if int64(c.current.Commits.Size()) > 2*c.F && c.state == StatePrepared {
		c.commit()
	}

	return nil
}

func (c *core) verifyCommit(commit *pbft.Subject, src pbft.Validator) error {
	logger := c.logger.New("from", src.Address().Hex(), "state", c.state)

	if !reflect.DeepEqual(commit, c.subject) {
		logger.Warn("Subjects do not match", "expected", c.subject, "got", commit)
		return pbft.ErrSubjectNotMatched
	}

	return nil
}

func (c *core) acceptCommit(commit *pbft.Subject, src pbft.Validator) {
	logger := c.logger.New("from", src.Address().Hex(), "state", c.state)

	// We check signature in Add
	if _, err := c.current.Commits.Add(commit, src); err != nil {
		logger.Error("Failed to record commit message", "msg", commit, "error", err)
	}
}

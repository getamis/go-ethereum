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
	c.broadcast(&message{
		Code: msgPrepare,
		Msg:  c.subject,
	})
}

func (c *core) handlePrepare(msg *message, src pbft.Validator) error {
	logger := c.logger.New("from", src.Address().Hex(), "state", c.state)
	logger.Debug("handlePrepare")

	prepare, ok := msg.Msg.(*pbft.Subject)
	if !ok {
		return errFailedDecodePrepare
	}

	if c.isFutureMessage(msgPrepare, prepare.View) {
		return errFutureMessage
	}

	if err := c.verifyPrepare(prepare, src); err != nil {
		return err
	}

	c.acceptPrepare(msg, src)

	// If 2f+1
	if int64(c.current.Prepares.Size()) > 2*c.F && c.state == StatePreprepared {
		c.setState(StatePrepared)
		c.sendCommit()
	}

	return nil
}

func (c *core) verifyPrepare(prepare *pbft.Subject, src pbft.Validator) error {
	logger := c.logger.New("from", src.Address().Hex(), "state", c.state)

	if prepare.View.Sequence != nil &&
		c.subject != nil &&
		prepare.View.Sequence.Cmp(c.subject.View.Sequence) < 0 {
		logger.Warn("Old message", "expected", c.subject, "got", prepare)
		return pbft.ErrOldMessage
	}

	if !reflect.DeepEqual(prepare, c.subject) {
		logger.Warn("Subjects do not match", "expected", c.subject, "got", prepare)
		return pbft.ErrSubjectNotMatched
	}

	return nil
}

func (c *core) acceptPrepare(msg *message, src pbft.Validator) {
	logger := c.logger.New("from", src.Address().Hex(), "state", c.state)

	// we check signature in Add
	if _, err := c.current.Prepares.Add(msg, src); err != nil {
		logger.Error("Failed to record prepare message", "msg", msg, "error", err)
	}
}

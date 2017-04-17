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
	log.Info("sendPrepare", "id", c.ID())
	c.broadcast(MsgPrepare, c.subject)
	c.prepareMsgs[c.ID()] = c.subject
}

func (c *core) handlePrepare(prepare *pbft.Subject, src pbft.Peer) error {
	log.Info("handlePrepare", "id", c.ID(), "from", src.ID())

	if err := c.verifyPrepare(prepare, src); err != nil {
		return err
	}

	c.acceptPrepare(prepare, src)

	// log.Info("Total prepare msgs", "id", pbft.ID(), "num", len(pbft.prepareMsgs))

	// If 2f+1
	if int64(len(c.prepareMsgs)) > 2*c.F && c.state == StatePreprepared {
		c.state = StatePrepared
		c.sendCommit()
	}

	return nil
}

func (c *core) verifyPrepare(prepare *pbft.Subject, src pbft.Peer) error {
	logger := log.New("id", c.ID(), "from", src.ID())

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
	c.prepareMsgs[src.ID()] = prepare
}

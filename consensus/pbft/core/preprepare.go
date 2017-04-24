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
	"github.com/ethereum/go-ethereum/log"
)

func (c *core) sendPreprepare(request *pbft.Request) {
	logger := c.logger.New("state", c.state)
	nextSeqView := c.nextSequence()

	if c.isPrimary() {
		preprepare := pbft.Preprepare{
			View:     nextSeqView,
			Proposal: c.makeProposal(nextSeqView.Sequence, request),
		}

		logger.Debug("sendPreprepare")
		c.broadcast(pbft.MsgPreprepare, preprepare)
	}
}

func (c *core) handlePreprepare(preprepare *pbft.Preprepare, src pbft.Peer) error {
	logger := log.New("from", src.ID(), "state", c.state)
	logger.Debug("handlePreprepare")

	if src.ID() != c.primaryID().Uint64() {
		logger.Warn("Ignore preprepare messages from non-proposer")
		return pbft.ErrNotFromProposer
	}

	view := c.nextSequence()
	if !reflect.DeepEqual(preprepare.View, view) {
		logger.Warn("Preprepare does not match", "expected", view, "got", preprepare.View)
		return pbft.ErrInvalidMessage
	}

	if preprepare.Proposal == nil {
		logger.Warn("Proposal is nil")
		return pbft.ErrNilProposal
	}

	if c.state == StateAcceptRequest {
		c.acceptPreprepare(preprepare)
		c.state = StatePreprepared
		c.sendPrepare()
		c.processBacklog()
	}

	return nil
}

func (c *core) acceptPreprepare(preprepare *pbft.Preprepare) {
	subject := &pbft.Subject{
		View: preprepare.View,
		// TODO: calculate digest
		Digest: []byte{0x01},
	}

	c.subject = subject
	c.current = pbft.NewLog(preprepare)
	c.checkpointMsgs = make(map[uint64]*pbft.Checkpoint)
}

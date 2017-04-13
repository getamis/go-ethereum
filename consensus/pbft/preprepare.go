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

package pbft

import "reflect"

func (pbft *pbft) sendPreprepare(request *Request) {
	nextSeqView := pbft.nextSequence()

	if pbft.isPrimary() {
		preprepare := Preprepare{
			View:     nextSeqView,
			Proposal: pbft.makeProposal(nextSeqView.Sequence, request),
		}

		log.Info("sendPreprepare", "id", pbft.ID())
		pbft.broadcast(MsgPreprepare, preprepare)
		pbft.handleCheckedPreprepare(&preprepare)
	}
}

func (pbft *pbft) handlePreprepare(preprepare *Preprepare, src Peer) error {
	logger := log.New("id", pbft.ID(), "from", src.ID())

	if pbft.ID() == src.ID() {
		logger.Warn("Ignore preprepare message from self")
		return ErrFromSelf
	}

	view := pbft.nextSequence()
	if !reflect.DeepEqual(preprepare.View, view) {
		logger.Warn("Preprepare does not match", "expected", view, "got", preprepare.View)
		return ErrInvalidMessage
	}

	if src.ID() != pbft.primaryID().Uint64() {
		logger.Warn("Ignore preprepare messages from non-primary replicas")
		return ErrNotFromProposer
	}

	if preprepare.Proposal == nil {
		logger.Warn("Proposal is nil")
		return ErrNilProposal
	}

	logger.Info("handlePreprepare")

	return pbft.handleCheckedPreprepare(preprepare)
}

func (pbft *pbft) handleCheckedPreprepare(preprepare *Preprepare) error {
	if pbft.state == StateAcceptRequest {
		pbft.acceptPreprepare(preprepare)
		pbft.state = StatePreprepared
	} else {
		return nil
	}

	if pbft.state == StatePreprepared {
		pbft.sendPrepare()
	}

	return nil
}

func (pbft *pbft) acceptPreprepare(preprepare *Preprepare) {
	subject := &Subject{
		View:   preprepare.View,
		Digest: []byte{0x01},
	}

	pbft.subject = subject
	pbft.preprepareMsg = preprepare
	pbft.prepareMsgs = make(map[uint64]*Subject)
	pbft.commitMsgs = make(map[uint64]*Subject)
	pbft.checkpointMsgs = make(map[uint64]*Checkpoint)
}

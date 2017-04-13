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

func (pbft *pbft) sendPrepare() {
	log.Info("sendPrepare", "id", pbft.ID())
	pbft.broadcast(MsgPrepare, pbft.subject)
	pbft.prepareMsgs[pbft.ID()] = pbft.subject
}

func (pbft *pbft) handlePrepare(prepare *Subject, src Peer) error {
	log.Info("handlePrepare", "id", pbft.ID(), "from", src.ID())

	if err := pbft.verifyPrepare(prepare, src); err != nil {
		return err
	}

	pbft.acceptPrepare(prepare, src)

	// log.Info("Total prepare msgs", "id", pbft.ID(), "num", len(pbft.prepareMsgs))

	// If 2f+1
	if int64(len(pbft.prepareMsgs)) > 2*pbft.F && pbft.state == StatePreprepared {
		pbft.state = StatePrepared
		pbft.sendCommit()
	}

	return nil
}

func (pbft *pbft) verifyPrepare(prepare *Subject, src Peer) error {
	logger := log.New("id", pbft.ID(), "from", src.ID())

	if prepare.View.Sequence != nil &&
		pbft.subject != nil &&
		prepare.View.Sequence.Cmp(pbft.subject.View.Sequence) < 0 {
		logger.Warn("Old message", "expected", pbft.subject, "got", prepare)
		return ErrOldMessage
	}

	if !reflect.DeepEqual(prepare, pbft.subject) {
		logger.Warn("Subject not match", "expected", pbft.subject, "got", prepare)
		return ErrSubjectNotMatched
	}

	return nil
}

func (pbft *pbft) acceptPrepare(prepare *Subject, src Peer) {
	pbft.prepareMsgs[src.ID()] = prepare
}

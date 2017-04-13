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

func (pbft *pbft) sendCommit() {
	log.Info("sendCommit", "id", pbft.ID())
	pbft.broadcast(MsgCommit, pbft.subject)
	pbft.commitMsgs[pbft.ID()] = pbft.subject
}

func (pbft *pbft) handleCommit(commit *Subject, src Peer) error {
	log.Info("handleCommit", "id", pbft.ID(), "from", src.ID())

	if err := pbft.verifyCommit(commit, src); err != nil {
		return err
	}

	pbft.commitMsgs[src.ID()] = commit

	// log.Info("Total commit msgs", "id", pbft.ID(), "num", len(pbft.commitMsgs))

	if int64(len(pbft.commitMsgs)) > 2*pbft.F && pbft.state == StatePrepared {
		// TODO: Enter checkpoint stage?

		pbft.state = StateCommitted
		pbft.backend.Commit(pbft.preprepareMsg.Proposal)
		pbft.state = StateAcceptRequest
	}

	return nil
}

func (pbft *pbft) verifyCommit(commit *Subject, src Peer) error {
	logger := log.New("id", pbft.ID(), "from", src.ID())

	if !reflect.DeepEqual(commit, pbft.subject) {
		logger.Warn("Subject not match", "expected", pbft.subject, "got", commit)
		return ErrSubjectNotMatched
	}

	return nil
}

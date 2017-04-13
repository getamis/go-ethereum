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

type Handler interface {
	Handle(Peer) error
	ID() uint64
}

func (pbft *pbft) Handle(src Peer) error {
	r := src.Reader()

	// Read the next message from the remote peer, and ensure it's fully consumed
	msg, err := r.ReadMsg()
	if err != nil {
		log.Error("Failed to read message", "error", err, "peer", src)
		return err
	}

	// TODO
	// if msg.Size > ProtocolMaxMsgSize {
	// 	return errResp(ErrMsgTooLarge, "%v > %v", msg.Size, ProtocolMaxMsgSize)
	// }

	defer msg.Discard()

	switch msg.Code {
	case MsgRequest:
		var m *Request
		if err := msg.Decode(&m); err != nil {
			log.Error("Failed to decode request", "error", err)
			return err
		}
		return pbft.handleRequest(m, src)
	case MsgPreprepare:
		var m *Preprepare
		if err := msg.Decode(&m); err != nil {
			log.Error("Failed to decode preprepare", "error", err)
			return err
		}
		return pbft.handlePreprepare(m, src)
	case MsgPrepare:
		var m *Subject
		if err := msg.Decode(&m); err != nil {
			log.Error("Failed to decode prepare", "error", err)
			return err
		}
		return pbft.handlePrepare(m, src)
	case MsgCommit:
		var m *Subject
		if err := msg.Decode(&m); err != nil {
			log.Error("Failed to decode commit", "error", err)
			return err
		}
		return pbft.handleCommit(m, src)
	case MsgCheckpoint:
	case MsgViewChange:
	case MsgNewView:
	default:
		log.Error("Invalid message", "msg", msg)
	}

	return nil
}

func (pbft *pbft) ID() uint64 {
	return pbft.id
}

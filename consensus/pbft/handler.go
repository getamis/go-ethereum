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

import "fmt"

func (pbft *pbft) Start() {
	go func() {
		for event := range pbft.events.Chan() {
			// A real event arrived, process interesting content
			switch ev := event.Data.(type) {
			case ConnectionEvent:

			case RequestEvent:

			case MessageEvent:
				pbft.handleMessage(ev.Payload, pbft.backend.Peer(ev.ID))
			}
		}
	}()
}

func (pbft *pbft) Stop() {
	pbft.events.Unsubscribe()
}

func (pbft *pbft) handleMessage(payload []byte, src Peer) error {
	logger := log.New("id", pbft.ID(), "from", src)
	var msg Message

	err := Decode(payload, &msg)
	if err != nil {
		logger.Error("Failed to decode message", "error", err)
		return err
	}

	switch msg.Code {
	case MsgRequest:
		m, ok := msg.Msg.(*Request)
		if !ok {
			return fmt.Errorf("failed to decode Request, err:%v", err)
		}
		return pbft.handleRequest(m, src)
	case MsgPreprepare:
		m, ok := msg.Msg.(*Preprepare)
		if !ok {
			return fmt.Errorf("failed to decode Preprepare, err:%v", err)
		}
		return pbft.handlePreprepare(m, src)
	case MsgPrepare:
		m, ok := msg.Msg.(*Subject)
		if !ok {
			return fmt.Errorf("failed to decode Subject, err:%v", err)
		}
		return pbft.handlePrepare(m, src)
	case MsgCommit:
		m, ok := msg.Msg.(*Subject)
		if !ok {
			return fmt.Errorf("failed to decode Subject, err:%v", err)
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

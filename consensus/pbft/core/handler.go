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
	"fmt"

	"github.com/ethereum/go-ethereum/consensus/pbft"
)

func (c *core) Start() {
	go func() {
		for event := range c.events.Chan() {
			// A real event arrived, process interesting content
			switch ev := event.Data.(type) {
			case pbft.ConnectionEvent:

			case pbft.RequestEvent:

			case pbft.MessageEvent:
				c.handleMessage(ev.Payload, c.backend.Peer(ev.ID))
			}
		}
	}()
}

func (c *core) Stop() {
	c.events.Unsubscribe()
}

func (c *core) handleMessage(payload []byte, src pbft.Peer) error {
	logger := log.New("id", c.ID(), "from", src)
	var msg pbft.Message

	err := pbft.Decode(payload, &msg)
	if err != nil {
		logger.Error("Failed to decode message", "error", err)
		return err
	}

	switch msg.Code {
	case MsgRequest:
		m, ok := msg.Msg.(*pbft.Request)
		if !ok {
			return fmt.Errorf("failed to decode Request, err:%v", err)
		}
		return c.handleRequest(m, src)
	case MsgPreprepare:
		m, ok := msg.Msg.(*pbft.Preprepare)
		if !ok {
			return fmt.Errorf("failed to decode Preprepare, err:%v", err)
		}
		return c.handlePreprepare(m, src)
	case MsgPrepare:
		m, ok := msg.Msg.(*pbft.Subject)
		if !ok {
			return fmt.Errorf("failed to decode Subject, err:%v", err)
		}
		return c.handlePrepare(m, src)
	case MsgCommit:
		m, ok := msg.Msg.(*pbft.Subject)
		if !ok {
			return fmt.Errorf("failed to decode Subject, err:%v", err)
		}
		return c.handleCommit(m, src)
	case MsgCheckpoint:
	case MsgViewChange:
	case MsgNewView:
	default:
		log.Error("Invalid message", "msg", msg)
	}

	return nil
}

func (c *core) ID() uint64 {
	return c.id
}

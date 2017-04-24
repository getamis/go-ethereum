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
				c.handleRequest(&pbft.Request{
					Payload: ev.Payload,
				}, c.backend.Peers().GetByIndex(c.ID()))
			case pbft.MessageEvent:
				c.handleMsg(ev.Payload, c.backend.Peers().GetByIndex(ev.ID))
			case backlogEvent:
				c.handle(ev.msg, ev.src)
			}
		}
	}()
}

func (c *core) Stop() {
	c.events.Unsubscribe()
}

func (c *core) handleMsg(payload []byte, src pbft.Peer) error {
	logger := c.logger.New("id", c.ID(), "from", src.ID())
	var msg pbft.Message

	err := pbft.Decode(payload, &msg)
	if err != nil {
		logger.Error("Failed to decode message", "error", err)
		return err
	}

	return c.handle(&msg, src)
}

func (c *core) handle(msg *pbft.Message, src pbft.Peer) error {
	logger := c.logger.New("id", c.ID(), "from", src.ID())

	testBacklog := func(err error) error {
		if err == errFutureMessage {
			c.storeBacklog(msg, src)
			return nil
		}

		return err
	}

	switch msg.Code {
	case pbft.MsgPreprepare:
		m, ok := msg.Msg.(*pbft.Preprepare)
		if !ok {
			return fmt.Errorf("failed to decode Preprepare")
		}
		return c.handlePreprepare(m, src)
	case pbft.MsgPrepare:
		m, ok := msg.Msg.(*pbft.Subject)
		if !ok {
			return fmt.Errorf("failed to decode Prepare")
		}
		return testBacklog(c.handlePrepare(m, src))
	case pbft.MsgCommit:
		m, ok := msg.Msg.(*pbft.Subject)
		if !ok {
			return fmt.Errorf("failed to decode Commit")
		}
		return testBacklog(c.handleCommit(m, src))
	case pbft.MsgCheckpoint:
	case pbft.MsgViewChange:
	case pbft.MsgNewView:
	default:
		logger.Error("Invalid message", "msg", msg)
	}

	return nil
}

func (c *core) ID() uint64 {
	return c.id
}

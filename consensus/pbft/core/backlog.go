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
	"github.com/ethereum/go-ethereum/consensus/pbft"
	"gopkg.in/karalabe/cookiejar.v2/collections/prque"
)

var (
	waitFor = map[State]uint64{
		StateAcceptRequest: msgPreprepare,
		StatePreprepared:   msgPrepare,
		StatePrepared:      msgCommit,
		StateCommitted:     msgAll,
	}
)

// check whether it's a future message
// It's a future message if the message priority is smaller than current priority
func (c *core) isFutureMessage(msgCode uint64, view *pbft.View) bool {
	if view == nil || view.Sequence == nil || view.Round == nil {
		return false
	}

	waitMsgCode, ok := waitFor[c.state]
	// don't check if not in pre-defined state
	if !ok {
		return false
	}
	priority := toPriority(waitMsgCode, c.currentView())
	newPriority := toPriority(msgCode, view)

	return priority > newPriority
}

func (c *core) storeBacklog(msg *message, src pbft.Validator) {
	logger := c.logger.New("from", src.Address().Hex(), "state", c.state)

	if src.Address() == c.Address() {
		logger.Warn("Backlog from self")
		return
	}

	logger.Trace("Store future message")

	c.backlogsMu.Lock()
	defer c.backlogsMu.Unlock()

	backlog := c.backlogs[src]
	if backlog == nil {
		backlog = prque.New()
	}
	switch msg.Code {
	case msgPreprepare:
		var p *pbft.Preprepare
		err := msg.Decode(&p)
		if err == nil {
			backlog.Push(msg, toPriority(msg.Code, p.View))
		}
		// for pbft.MsgPrepare and pbft.MsgCommit cases
	default:
		var p *pbft.Subject
		err := msg.Decode(&p)
		if err == nil {
			backlog.Push(msg, toPriority(msg.Code, p.View))
		}
	}
	c.backlogs[src] = backlog
}

func (c *core) processBacklog() {
	c.backlogsMu.Lock()
	defer c.backlogsMu.Unlock()

	for src, backlog := range c.backlogs {
		if backlog == nil {
			continue
		}

		logger := c.logger.New("from", src.Address().Hex(), "state", c.state)
		isFuture := false

		// We stop processing if
		//   1. backlog is empty
		//   2. The first message in queue is a future message
		for !(backlog.Empty() || isFuture) {
			m, prio := backlog.Pop()
			msg := m.(*message)
			var view *pbft.View
			switch msg.Code {
			case msgPreprepare:
				var m *pbft.Preprepare
				err := msg.Decode(&m)
				if err == nil {
					view = m.View
				}
				// for pbft.MsgPrepare and pbft.MsgCommit cases
			default:
				var sub *pbft.Subject
				err := msg.Decode(&sub)
				if err == nil {
					view = sub.View
				}
			}
			if view == nil {
				logger.Debug("Nil view", "msg", msg)
				continue
			}
			// Push back if it's a future message
			if c.isFutureMessage(msg.Code, view) {
				logger.Trace("Stop processing backlog", "msg", msg)
				backlog.Push(msg, prio)
				isFuture = true
				break
			}

			logger.Trace("Post backlog event", "msg", msg)

			go c.sendInternalEvent(backlogEvent{
				src: src,
				msg: msg,
			})
		}
	}
}

func toPriority(msgCode uint64, view *pbft.View) float32 {
	// FIXME: round will be reset as 0 while new sequence
	// 10 * Round limits the range of message code is from 0 to 9
	// 1000 * Sequence limits the range of round is from 0 to 99
	return -float32(view.Sequence.Uint64()*1000 + view.Round.Uint64()*10 + msgCode)
}

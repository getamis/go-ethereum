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

// check whether it's a future message
// if this round is completed,
// - if the current state is StateAcceptRequest (waiting for a new MsgPreprepare message)
//   and the message is MsgPreprepare, we will check whether the view is next sequence or
//   view number. If so, it's not a future message because we can process it now and enter
//   the next round.
// - Otherwise, we ignore the msg code, because this round is completed. We only compute
//   the message priority by its sequence and view number
// if this round is not completed, we compute current state and the message priority.
// It's a future message if the message priority is smaller than current priority
func (c *core) isFutureMessage(msgCode uint64, view *pbft.View) bool {
	if view == nil || view.Sequence == nil || view.ViewNumber == nil {
		return false
	}

	if c.subject == nil {
		// only in initial state
		if msgCode == msgPreprepare {
			return false
		}
		return true
	}

	var priority, newPriority float32
	if c.completed {
		// check the next round
		newPriority = toPriority(msgCode, view)
		if c.state == StateAcceptRequest && msgCode == msgPreprepare {
			// next sequence
			if toPriority(msgCode, c.nextSequence()) == newPriority {
				return false
			}
			// next view
			if toPriority(msgCode, c.nextViewNumber()) == newPriority {
				return false
			}
		}
		// if completed, skip the message code because this round is completed
		priority = toPriority(msgCode, c.subject.View)
	} else {
		priority = toPriority(uint64(c.state), c.subject.View)
		newPriority = toPriority(msgCode, view)
	}
	return priority > newPriority
}

func (c *core) storeBacklog(msg *message, src pbft.Validator) {
	logger := c.logger.New("from", src.Address().Hex(), "state", c.state)

	if src.Address() == c.Address() {
		logger.Warn("Backlog from self")
		return
	}

	logger.Debug("Store future message")

	c.backlogsMu.Lock()
	defer c.backlogsMu.Unlock()

	backlog := c.backlogs[src]
	if backlog == nil {
		backlog = prque.New()
	}
	switch msg.Code {
	case msgPreprepare:
		m, ok := msg.Msg.(*pbft.Preprepare)
		if ok {
			backlog.Push(msg, toPriority(msg.Code, m.View))
		}
		// for pbft.MsgPrepare and pbft.MsgCommit cases
	default:
		sub, ok := msg.Msg.(*pbft.Subject)
		if ok {
			backlog.Push(msg, toPriority(msg.Code, sub.View))
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
				m, ok := msg.Msg.(*pbft.Preprepare)
				if ok {
					view = m.View
				}
				// for pbft.MsgPrepare and pbft.MsgCommit cases
			default:
				sub, ok := msg.Msg.(*pbft.Subject)
				if ok {
					view = sub.View
				}
			}
			if view == nil {
				logger.Debug("Nil view", "msg", msg)
				continue
			}
			// Push back if it's a future message
			if c.isFutureMessage(msg.Code, view) {
				logger.Debug("Stop processing backlog", "msg", msg)
				backlog.Push(msg, prio)
				isFuture = true
				break
			}

			logger.Debug("Post backlog event", "msg", msg)

			go c.sendInternalEvent(backlogEvent{
				src: src,
				msg: msg,
			})
		}
	}
}

func toPriority(msgCode uint64, view *pbft.View) float32 {
	// In our case, sequence and view number will never reset to zero
	return -float32((view.Sequence.Uint64()+view.ViewNumber.Uint64())*10 + msgCode)
}

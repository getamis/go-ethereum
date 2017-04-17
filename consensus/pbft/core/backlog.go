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
	"math/big"

	"github.com/ethereum/go-ethereum/consensus/pbft"
	"gopkg.in/karalabe/cookiejar.v2/collections/prque"
)

func (c *core) isFutureMessage(view *pbft.View) bool {
	if view == nil {
		return false
	}

	if c.subject == nil {
		return true
	}

	if view.Sequence.Cmp(c.subject.View.Sequence) > 0 ||
		view.ViewNumber.Cmp(c.subject.View.ViewNumber) > 0 {
		return true
	}

	return false
}

func (c *core) storeBacklog(msg *pbft.Message, src pbft.Peer) {
	if src.ID() == c.ID() {
		log.Warn("Backlog from self")
		return
	}

	log.Debug("Store future message", "id", c.ID(), "from", src.ID())

	c.backlogsMu.Lock()
	defer c.backlogsMu.Unlock()

	backlog := c.backlogs[src]
	if backlog == nil {
		backlog = prque.New()
	}

	// Messages here always includes a pbft.Subject
	sub := msg.Msg.(*pbft.Subject)

	backlog.Push(msg, toPriority(sub.View.Sequence))
	c.backlogs[src] = backlog
}

func (c *core) processBacklog() {
	c.backlogsMu.Lock()
	defer c.backlogsMu.Unlock()

	for src, backlog := range c.backlogs {
		if backlog == nil {
			continue
		}

		isFuture := false

		// We stop processing if
		//   1. backlog is empty
		//   2. The first message in queue is a future message
		for !(backlog.Empty() || isFuture) {
			m, prio := backlog.Pop()
			msg := m.(*pbft.Message)
			sub := msg.Msg.(*pbft.Subject)

			// Push back if it's a future message
			if c.isFutureMessage(sub.View) {
				log.Debug("Stop processing backlog", "id", c.ID(), "msg", msg, "from", src)
				backlog.Push(msg, prio)
				isFuture = true
				break
			}

			log.Debug("Post backlog event", "id", c.ID(), "msg", msg, "from", src)

			go c.backend.EventMux().Post(backlogEvent{
				src: src,
				msg: msg,
			})
		}
	}
}

func toPriority(n *big.Int) float32 {
	return -float32(n.Uint64())
}

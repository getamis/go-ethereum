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

package backends

import (
	"bytes"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/pbft"
)

func NewPeerSet(peers []pbft.Peer) pbft.PeerSet {
	return &peerSet{
		peers: peers,
	}
}

type peerSet struct {
	peers []pbft.Peer
}

func (ps *peerSet) GetPeerByIndex(i uint64) pbft.Peer {
	return ps.peers[i]
}

func (ps *peerSet) GetPeerByAddress(addr common.Address) pbft.Peer {
	idx := sort.Search(len(ps.peers), func(i int) bool {
		return bytes.Compare(addr.Bytes(), ps.peers[i].Address().Bytes()) <= 0
	})

	if idx != len(ps.peers) && bytes.Compare(ps.peers[idx].Address().Bytes(), addr.Bytes()) == 0 {
		return ps.peers[idx]
	} else {
		return nil
	}
}

func (ps *peerSet) GetPeerByPubKey(publicKey string) pbft.Peer {
	idx := sort.Search(len(ps.peers), func(i int) bool {
		return strings.Compare(publicKey, ps.peers[i].PublicKey()) <= 0
	})

	if idx != len(ps.peers) && strings.Compare(ps.peers[idx].PublicKey(), publicKey) == 0 {
		return ps.peers[idx]
	} else {
		return nil
	}
}

func (ps *peerSet) Peers() []pbft.Peer {
	return ps.peers
}

func (ps *peerSet) GetProposer() pbft.Peer {
	// XXX: for workaround, shouldn't return fixed peer always
	if len(ps.peers) == 0 {
		return nil
	}
	return ps.peers[0]
}

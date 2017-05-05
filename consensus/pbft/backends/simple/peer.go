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

package simple

import (
	"bytes"
	"crypto/ecdsa"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type peer struct {
	id        string
	publicKey *ecdsa.PublicKey
	address   common.Address
}

func newPeer(id string, publicKey *ecdsa.PublicKey) *peer {
	return &peer{
		id:        id,
		publicKey: publicKey,
		address:   crypto.PubkeyToAddress(*publicKey),
	}
}

func (p *peer) ID() string                  { return p.id }
func (p *peer) Address() common.Address     { return p.address }
func (p *peer) PublicKey() *ecdsa.PublicKey { return p.publicKey }

//-----------------------------------------------------------------------------

type peerSet struct {
	mtx sync.Mutex

	peers map[string]*peer
	list  []*peer
}

func newPeerSet() *peerSet {
	return &peerSet{
		peers: make(map[string]*peer),
		list:  make([]*peer, 0),
	}
}

func (ps *peerSet) Add(p *peer) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if p == nil {
		return
	}

	if ps.peers[p.ID()] == nil {
		ps.peers[p.ID()] = p
		ps.list = append(ps.list, p)
	} else {
		// replace old one
		if i, peer := getPeer(ps.list, p.ID()); i >= 0 && peer != nil {
			ps.list[i] = p
		}
		ps.peers[p.ID()] = p
	}
}

func (ps *peerSet) Get(id string) *peer {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return ps.peers[id]
}

func (ps *peerSet) GetByAddress(addr common.Address) *peer {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	for _, peer := range ps.peers {
		if bytes.Compare(addr.Bytes(), peer.Address().Bytes()) == 0 {
			return peer
		}
	}
	return nil
}

func (ps *peerSet) Remove(id string) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.peers[id] == nil {
		return
	}

	if i, peer := getPeer(ps.list, id); peer != nil {
		ps.list = append(ps.list[:i], ps.list[i+1:]...)
		delete(ps.peers, id)
	}
}

func (ps *peerSet) List() []*peer {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return ps.list
}

func getPeer(list []*peer, id string) (index int, peer *peer) {
	for i, peer := range list {
		if peer.ID() == id {
			return i, peer
		}
	}
	return -1, nil
}

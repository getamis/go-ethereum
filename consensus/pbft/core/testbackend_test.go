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
	"crypto/ecdsa"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/pbft"
	"github.com/ethereum/go-ethereum/consensus/pbft/backends"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/event"
	elog "github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
)

var testLogger = elog.New()

type testSystemBackend struct {
	id  uint64
	sys *testSystem

	engine Engine
	peers  pbft.PeerSet
	events *event.TypeMux

	commitMsgs []*pbft.Proposal

	privateKey *ecdsa.PrivateKey
}

// ==============================================
//
// define the functions that needs to be provided for PBFT.

func (self *testSystemBackend) ID() uint64 {
	return self.id
}

// Peers returns all connected peers
func (self *testSystemBackend) Peers() pbft.PeerSet {
	return self.peers
}

func (self *testSystemBackend) EventMux() *event.TypeMux {
	return self.events
}

func (self *testSystemBackend) Send(message []byte) {
	testLogger.Info("enqueuing a message...", "id", self.ID())
	self.sys.queuedMessage <- &testMessage{
		From:    self.ID(),
		Message: message,
	}
}

type testMessage struct {
	From    uint64
	Message []byte
}

func (self *testSystemBackend) UpdateState(state *pbft.State) {
	testLogger.Warn("nothing to happen")
}

func (self *testSystemBackend) Commit(proposal *pbft.Proposal) {
	testLogger.Info("commit message", "id", self.ID())
	self.commitMsgs = append(self.commitMsgs, proposal)
}

func (self *testSystemBackend) Verify(proposal *pbft.Proposal) (bool, error) {
	return true, nil
}

func (self *testSystemBackend) Sign(data []byte) ([]byte, error) {
	hashData := crypto.Keccak256([]byte(data))
	return crypto.Sign(hashData, self.privateKey)
}

func (self *testSystemBackend) CheckSignature([]byte, common.Address, []byte) error {
	return nil
}

func (self *testSystemBackend) Hash(b interface{}) common.Hash {
	return common.StringToHash("Test")
}
func (self *testSystemBackend) Encode(b interface{}) ([]byte, error) {
	return []byte(""), nil

}
func (self *testSystemBackend) Decode([]byte, interface{}) error {
	return nil
}

func (self *testSystemBackend) NewRequest(request []byte) {
	go self.events.Post(pbft.RequestEvent{
		Payload: request,
	})
}

// ==============================================
//
// define the functions that need to be provided for PBFT protocol manager.

func (self *testSystemBackend) AddPeer(peerPublicKey string) {
	testLogger.Info(fmt.Sprintf("add peer: %d", self.Peers().GetByPublicKey(peerPublicKey).ID()), "id", self.ID())
}

// Remove a peer
func (self *testSystemBackend) RemovePeer(peerPublicKey string) {
	testLogger.Warn("nothing to happen")
}

// Handle a message from peer
func (self *testSystemBackend) HandleMsg(peerPublicKey string, data []byte) {
	go self.EventMux().Post(pbft.MessageEvent{
		ID:      self.peers.GetByPublicKey(peerPublicKey).ID(),
		Payload: data,
	})
}

// Start is initialized peers
func (self *testSystemBackend) Start(chain consensus.ChainReader) {
	peers := make([]pbft.Peer, len(self.sys.backends))
	for i, backend := range self.sys.backends {
		peers[i] = &testPeer{
			address:   getPublicKeyAddress(backend.privateKey),
			publicKey: getPublicKeyAddress(backend.privateKey).Hex(),
			id:        i, // use the index as id
		}
	}
	self.peers = backends.NewPeerSet(peers)
}

// Stop the engine
func (self *testSystemBackend) Stop() {
	testLogger.Warn("nothing to happen")
}

// ==============================================
//

type testSystem struct {
	backends map[uint64]*testSystemBackend

	queuedMessage chan *testMessage
	quit          chan struct{}
}

func newTestSystem() *testSystem {
	testLogger.SetHandler(elog.StdoutHandler)
	return &testSystem{
		backends: make(map[uint64]*testSystemBackend),

		queuedMessage: make(chan *testMessage),
		quit:          make(chan struct{}),
	}
}

// run is triggered backend, core, and queue system.
func (t *testSystem) run() {
	// start a queue system
	go func() {
		for {
			select {
			case <-t.quit:
				return
			case queuedMessage := <-t.queuedMessage:
				testLogger.Info("consuming a queue message...", "msg from", queuedMessage.From)
				for _, backend := range t.backends {
					go backend.HandleMsg(backend.peers.GetByIndex(queuedMessage.From).PublicKey(), queuedMessage.Message)
				}
			}
		}
	}()
}

func (t *testSystem) stop() {
	close(t.quit)

	for _, backend := range t.backends {
		backend.engine.Stop()
		backend.Stop()
	}
}

func (t *testSystem) NewBackend(id uint64) *testSystemBackend {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		panic(err)
	}
	backend := &testSystemBackend{
		id:         id,
		sys:        t,
		privateKey: privateKey,
		events:     new(event.TypeMux),
	}

	t.backends[id] = backend
	return backend
}

// ==============================================
//

func getPublicKeyAddress(privateKey *ecdsa.PrivateKey) common.Address {
	return crypto.PubkeyToAddress(privateKey.PublicKey)
}

type testPeer struct {
	id        uint64
	publicKey string
	address   common.Address
}

func (p *testPeer) ID() uint64 {
	return p.id
}

func (p *testPeer) Address() common.Address {
	return p.address
}

func (p *testPeer) PublicKey() string {
	return p.publicKey
}

func (p *testPeer) SetPublicKey(pubKey string) {
	p.publicKey = pubKey
}

func (p *testPeer) IsConnected() bool {
	return p.publicKey != ""
}

func (p *testPeer) ReadMsg() (p2p.Msg, error) {
	return p2p.Msg{}, nil
}

func (p *testPeer) WriteMsg(msg p2p.Msg) error {
	return nil
}

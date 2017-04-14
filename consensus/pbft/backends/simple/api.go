package simple

import (
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/log"
)

// API is a user facing RPC API to dump PBFT state
type API struct {
	chain   consensus.ChainReader
	backend *simpleBackend
}

func (self *API) ConsensusState() {
	log.Info("Dump PBFT state")
}

func (self *API) ViewChange() {
	log.Info("Force view change")
}

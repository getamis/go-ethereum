package simple

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/log"
)

// API is a user facing RPC API to dump PBFT state
type API struct {
	chain   consensus.ChainReader
	backend *simpleBackend
}

// Snapshot returns current state and proposer
func (api *API) Snapshot() {
	state, snapshot := api.backend.core.Snapshot()
	p := api.backend.valSet.GetProposer().Address()
	log.Info("Snapshot", "sequence", snapshot.Sequence, "Round", snapshot.Round,
		"state", state, "proposer", p.Hex(),
		"hash", snapshot.Preprepare.Proposal.Hash().Hex(),
		"prepares", snapshot.Prepares, "commits", snapshot.Commits,
		"checkpoint", snapshot.Checkpoints)
}

// Backlog returns backlogs
func (api *API) Backlog() {
	backlog := api.backend.core.Backlog()
	logs := make([]string, 0, len(backlog))
	for address, q := range backlog {
		logs = append(logs, fmt.Sprintf("{%v, %v}", address.Address().Hex(), q.Size()))
	}
	log.Info("Backlog", "logs", fmt.Sprintf("[%v]", strings.Join(logs, ", ")))
}

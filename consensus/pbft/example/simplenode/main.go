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

package main

import (
	"os"
	"time"

	"github.com/ethereum/go-ethereum/consensus/pbft"
	"github.com/ethereum/go-ethereum/consensus/pbft/backends/simulation"
	"github.com/ethereum/go-ethereum/log"
)

const (
	F = 1
	N = 3*F + 1
)

func main() {
	glogger := log.NewGlogHandler(log.StreamHandler(os.Stderr, log.TerminalFormat(false)))
	glogger.Verbosity(log.LvlDebug)
	log.Root().SetHandler(glogger)

	var validators = make([]pbft.Algorithm, N)
	var backends = make([]pbft.Backend, N)
	// var peerList = make([]pbft.Peer, N)

	for i := 0; i < N; i++ {
		// log.Info("Initialize", "peer", i)

		backend := simulation.NewSimulationBackend(uint64(i), N, F)
		validator := pbft.New(backend)
		backend.SetHandler(validator)

		validators[i] = validator
		backends[i] = backend
		// peerList[i] = simulation.NewPeer(uint64(i))
	}

	for i := 0; i < N; i++ {
		for j := 0; j < N; j++ {
			if i != j {
				backend := backends[i]
				if err := backend.AddPeer(backends[j].Peer()); err != nil {
					log.Error("Failed to add peer", "error", err)
				}
			}
		}
	}

	log.Info("Start")

	time.Sleep(3 * time.Second)

	validators[0].NewRequest([]byte(time.Now().String()))

	for {
		select {}
	}
}

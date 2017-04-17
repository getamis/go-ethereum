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
	"bufio"
	"fmt"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/consensus/pbft/backends/simulation"
	pbftCore "github.com/ethereum/go-ethereum/consensus/pbft/core"
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

	var validators = make([]pbftCore.Engine, N)
	var backends = make([]*simulation.Backend, N)

	for i := 0; i < N; i++ {
		backend := simulation.NewBackend(uint64(i))
		backend.Start()
		defer backend.Stop()
		validator := pbftCore.New(backend)
		validator.Start()
		defer validator.Stop()

		validators[i] = validator
		backends[i] = backend
	}

	for i := 0; i < N; i++ {
		for j := 0; j < N; j++ {
			if i != j {
				backends[i].AddPeer(fmt.Sprintf("%v", j))
			}
		}
	}

	log.Info("Start")

	time.Sleep(3 * time.Second)

	for {
		b, _ := bufio.NewReader(os.Stdin).ReadByte()
		if b != '\n' {
			backends[0].NewRequest([]byte(time.Now().String()))
		}
	}
}

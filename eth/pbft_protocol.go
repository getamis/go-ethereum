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

package eth

// Constants to match up protocol versions and messages
const (
	pbft101 = 101
)

// Official short name of the protocol used during capability negotiation.
var PBFTProtocolName = "pbft"

// Supported versions of the eth protocol (first is primary).
var PBFTProtocolVersions = []uint{pbft101}

// Number of implemented message corresponding to different protocol versions.
var PBFTProtocolLengths = []uint64{19}

// pbft protocol message codes
const (
	// Protocol messages belonging to pbft/101
	GetNodeVote = 0x11
	NodeVote    = 0x12
)

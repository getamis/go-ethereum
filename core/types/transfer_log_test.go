// Copyright 2018 The go-ethereum Authors
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

package types

import (
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

func TestUnmarshalTransferLog(t *testing.T) {
	var unmarshalLogTests = map[string]struct {
		input     string
		want      *TransferLog
		wantError error
	}{
		"ok": {
			input: `{"from":"0xecf8f87f810ecf450940c9f60066b4a7a501d6a7","to":"0x3e65e1eefde5ea7ccfc9a9a1634abe90f32262f8","value":"0x4","transactionHash":"0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e"}`,
			want: &TransferLog{
				From:   common.HexToAddress("0xecf8f87f810ecf450940c9f60066b4a7a501d6a7"),
				To:     common.HexToAddress("0x3e65e1eefde5ea7ccfc9a9a1634abe90f32262f8"),
				Value:  big.NewInt(4),
				TxHash: common.HexToHash("0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e"),
			},
		},
		"missing from": {
			input:     `{"to":"0x3e65e1eefde5ea7ccfc9a9a1634abe90f32262f8","value":"0x4","transactionHash":"0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e"}`,
			wantError: fmt.Errorf("missing required field 'from' for TransferLog"),
		},
	}
	dumper := spew.ConfigState{DisableMethods: true, Indent: "    "}
	for name, test := range unmarshalLogTests {
		var log *TransferLog
		err := json.Unmarshal([]byte(test.input), &log)
		checkError(t, name, err, test.wantError)
		if test.wantError == nil && err == nil {
			if !reflect.DeepEqual(log, test.want) {
				t.Errorf("test %q:\nGOT %sWANT %s", name, dumper.Sdump(log), dumper.Sdump(test.want))
			}
		}
	}
}

func TestRLPEncodeAndDecodeTransferLog(t *testing.T) {
	transferLogs := &TransferLog{
		From:   common.HexToAddress("0xecf8f87f810ecf450940c9f60066b4a7a501d6a7"),
		To:     common.HexToAddress("0x3e65e1eefde5ea7ccfc9a9a1634abe90f32262f8"),
		Value:  big.NewInt(4),
		TxHash: common.HexToHash("0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e"),
	}
	bytes, err := rlp.EncodeToBytes(transferLogs)
	if err != nil {
		t.Errorf("unexpected error on rlp encode %v", err)
	}

	got := &TransferLog{}
	if err := rlp.DecodeBytes(bytes, got); err != nil {
		t.Errorf("unexpected error on rlp decode %v", err)
	}
	if !reflect.DeepEqual(transferLogs, got) {
		t.Errorf("TransferLog not equal between rlp encode/decode GOT %v WANT %v", got, transferLogs)
	}
}

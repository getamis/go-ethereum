package simple

import (
	"bytes"
	"crypto/ecdsa"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/istanbul"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
)

// in this test, we can set n to 1, and it means we can process Istanbul and commit a
// block by one node. Otherwise, if n is larger than 1, we have to generate
// other fake events to process Istanbul.
func newBlockChain(n int) (*core.BlockChain, *simpleBackend) {
	genesis, nodeKeys := getGenesisAndKeys(n)
	eventMux := new(event.TypeMux)
	memDB, _ := ethdb.NewMemDatabase()
	config := istanbul.DefaultConfig
	// Use the first key as private key
	backend := New(config, eventMux, nodeKeys[0], memDB)
	genesis.MustCommit(memDB)
	blockchain, err := core.NewBlockChain(memDB, genesis.Config, backend, eventMux, vm.Config{})
	if err != nil {
		panic(err)
	}
	commitBlock := func(block *types.Block) error {
		_, err := blockchain.InsertChain([]*types.Block{block})
		return err
	}
	backend.Start(blockchain, commitBlock)

	b, _ := backend.(*simpleBackend)

	snap, err := b.snapshot(blockchain, 0, common.Hash{}, nil)
	if err != nil {
		panic(err)
	}
	if snap == nil {
		panic("nil snap")
	}
	proposerAddr := snap.ValSet.GetProposer().Address()

	// find proposer key
	for _, key := range nodeKeys {
		addr := crypto.PubkeyToAddress(key.PublicKey)
		if addr.String() == proposerAddr.String() {
			b.privateKey = key
			b.address = addr
		}
	}

	return blockchain, b
}

func getGenesisAndKeys(n int) (*core.Genesis, []*ecdsa.PrivateKey) {
	// Setup validators
	var nodeKeys = make([]*ecdsa.PrivateKey, n)
	var addrs = make([]common.Address, n)
	for i := 0; i < n; i++ {
		nodeKeys[i], _ = crypto.GenerateKey()
		addrs[i] = crypto.PubkeyToAddress(nodeKeys[i].PublicKey)
	}

	// generate genesis block
	genesis := core.DefaultGenesisBlock()
	genesis.Config = params.TestChainConfig
	// force enable Istanbul engine
	genesis.Config.Istanbul = &params.IstanbulConfig{}
	genesis.Config.Ethash = nil
	genesis.Difficulty = defaultDifficulty
	genesis.Nonce = emptyNonce.Uint64()

	appendValidators(genesis, addrs)
	return genesis, nodeKeys
}

func appendValidators(genesis *core.Genesis, addrs []common.Address) {
	if len(genesis.ExtraData) < types.IstanbulExtraVanity {
		genesis.ExtraData = append(genesis.ExtraData, bytes.Repeat([]byte{0x00}, types.IstanbulExtraVanity)...)
	}
	genesis.ExtraData = genesis.ExtraData[:types.IstanbulExtraVanity]

	validatorSize := byte(len(addrs))
	genesis.ExtraData = append(genesis.ExtraData, validatorSize)

	for _, addr := range addrs {
		genesis.ExtraData = append(genesis.ExtraData, addr[:]...)
	}
	genesis.ExtraData = append(genesis.ExtraData, make([]byte, types.IstanbulExtraSeal)...)
}

func makeHeader(parent *types.Block, config *istanbul.Config) *types.Header {
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     parent.Number().Add(parent.Number(), common.Big1),
		GasLimit:   core.CalcGasLimit(parent),
		GasUsed:    new(big.Int),
		Extra:      parent.Extra(),
		Time:       new(big.Int).Add(parent.Time(), new(big.Int).SetUint64(config.BlockPeriod)),
		Difficulty: defaultDifficulty,
	}
	return header
}

func makeBlock(chain *core.BlockChain, engine *simpleBackend, parent *types.Block) *types.Block {
	block := makeBlockWithoutSeal(chain, engine, parent)
	block, _ = engine.Seal(chain, block, nil)
	return block
}

func makeBlockWithoutSeal(chain *core.BlockChain, engine *simpleBackend, parent *types.Block) *types.Block {
	header := makeHeader(parent, engine.config)
	engine.Prepare(chain, header)
	state, _ := chain.StateAt(parent.Root())
	block, _ := engine.Finalize(chain, header, state, nil, nil, nil)
	return block
}

func TestPrepare(t *testing.T) {
	chain, engine := newBlockChain(1)
	header := makeHeader(chain.Genesis(), engine.config)
	err := engine.Prepare(chain, header)
	if err != nil {
		t.Errorf("error should be nil, but got: %v", err)
	}
	header.ParentHash = common.StringToHash("1234567890")
	err = engine.Prepare(chain, header)
	if err != consensus.ErrUnknownAncestor {
		t.Errorf("error should be consensus.ErrUnknownAncestor, but got: %v", err)
	}
}

func TestSealStopChannel(t *testing.T) {
	chain, engine := newBlockChain(4)
	block := makeBlockWithoutSeal(chain, engine, chain.Genesis())
	stop := make(chan struct{}, 1)
	eventSub := engine.EventMux().Subscribe(istanbul.RequestEvent{})
	eventLoop := func() {
		select {
		case ev := <-eventSub.Chan():
			_, ok := ev.Data.(istanbul.RequestEvent)
			if !ok {
				t.Errorf("unexpected event comes, got: %v, expected: istanbul.RequestEvent", reflect.TypeOf(ev.Data))
			}
			stop <- struct{}{}
		}
		eventSub.Unsubscribe()
	}
	go eventLoop()
	finalBlock, err := engine.Seal(chain, block, stop)
	if err != nil {
		t.Errorf("error should be nil, but got: %v", err)
	}
	if finalBlock != nil {
		t.Errorf("block should be nil, but got: %v", finalBlock)
	}
}

func TestSealRoundChange(t *testing.T) {
	chain, engine := newBlockChain(4)
	block := makeBlockWithoutSeal(chain, engine, chain.Genesis())
	eventSub := engine.EventMux().Subscribe(istanbul.RequestEvent{})
	eventLoop := func() {
		select {
		case ev := <-eventSub.Chan():
			_, ok := ev.Data.(istanbul.RequestEvent)
			if !ok {
				t.Errorf("unexpected event comes, got: %v, expected: istanbul.RequestEvent", reflect.TypeOf(ev.Data))
			}
			engine.NextRound()
		}
		eventSub.Unsubscribe()
	}
	go eventLoop()

	seal := func() {
		engine.Seal(chain, block, nil)
		t.Errorf("should not be called")
	}
	go seal()

	const timeoutDura = 2 * time.Second
	timeout := time.NewTimer(timeoutDura)
	select {
	case <-timeout.C:
		// wait 2 seconds to ensure we cannot get any blocks from Istanbul
	}
}

func TestSealCommittedOtherHash(t *testing.T) {
	chain, engine := newBlockChain(4)
	block := makeBlockWithoutSeal(chain, engine, chain.Genesis())
	otherBlock := makeBlockWithoutSeal(chain, engine, block)
	eventSub := engine.EventMux().Subscribe(istanbul.RequestEvent{})
	eventLoop := func() {
		select {
		case ev := <-eventSub.Chan():
			_, ok := ev.Data.(istanbul.RequestEvent)
			if !ok {
				t.Errorf("unexpected event comes, got: %v, expected: istanbul.RequestEvent", reflect.TypeOf(ev.Data))
			}
			engine.Commit(otherBlock)
		}
		eventSub.Unsubscribe()
	}
	go eventLoop()
	seal := func() {
		engine.Seal(chain, block, nil)
		t.Errorf("should not be called")
	}
	go seal()

	const timeoutDura = 2 * time.Second
	timeout := time.NewTimer(timeoutDura)
	select {
	case <-timeout.C:
		// wait 2 seconds to ensure we cannot get any blocks from Istanbul
	}
}

func TestSealCommitted(t *testing.T) {
	chain, engine := newBlockChain(1)
	block := makeBlockWithoutSeal(chain, engine, chain.Genesis())
	expectedBlock, _ := engine.updateBlock(engine.chain.GetHeader(block.ParentHash(), block.NumberU64()-1), block)

	finalBlock, err := engine.Seal(chain, block, nil)
	if err != nil {
		t.Errorf("error should be nil, but got: %v", err)
	}
	if finalBlock.Hash() != expectedBlock.Hash() {
		t.Errorf("block should be equal, got: %v, expected: %v", finalBlock.Hash(), expectedBlock.Hash())
	}
}

func TestVerifyHeader(t *testing.T) {
	chain, engine := newBlockChain(1)

	// correct case
	block := makeBlock(chain, engine, chain.Genesis())
	err := engine.VerifyHeader(chain, block.Header(), false)
	if err != nil {
		t.Errorf("error should be nil, got: %v", err)
	}

	// short extra data
	header := block.Header()
	header.Extra = []byte{}
	err = engine.VerifyHeader(chain, header, false)
	if err != errInvalidExtraDataFormat {
		t.Errorf("unexpected error comes, got: %v, expected: errInvalidExtraDataFormat", err)
	}
	// incorrect extra format
	header.Extra = []byte("0000000000000000000000000000000012300000000000000000000000000000000000000000000000000000000000000000")
	err = engine.VerifyHeader(chain, header, false)
	if err != errInvalidExtraDataFormat {
		t.Errorf("unexpected error comes, got: %v, expected: errInvalidExtraDataFormat", err)
	}

	// non zero MixDigest
	block = makeBlockWithoutSeal(chain, engine, chain.Genesis())
	header = block.Header()
	header.MixDigest = common.StringToHash("123456789")
	err = engine.VerifyHeader(chain, header, false)
	if err != errInvalidMixDigest {
		t.Errorf("unexpected error comes, got: %v, expected: errInvalidMixDigest", err)
	}

	// invalid uncles hash
	block = makeBlockWithoutSeal(chain, engine, chain.Genesis())
	header = block.Header()
	header.UncleHash = common.StringToHash("123456789")
	err = engine.VerifyHeader(chain, header, false)
	if err != errInvalidUncleHash {
		t.Errorf("unexpected error comes, got: %v, expected: errInvalidUncleHash", err)
	}

	// invalid difficulty
	block = makeBlockWithoutSeal(chain, engine, chain.Genesis())
	header = block.Header()
	header.Difficulty = big.NewInt(2)
	err = engine.VerifyHeader(chain, header, false)
	if err != errInvalidDifficulty {
		t.Errorf("unexpected error comes, got: %v, expected: errInvalidDifficulty", err)
	}

	// invalid timestamp
	block = makeBlockWithoutSeal(chain, engine, chain.Genesis())
	header = block.Header()
	header.Time = new(big.Int).Add(chain.Genesis().Time(), new(big.Int).SetUint64(engine.config.BlockPeriod-1))
	err = engine.VerifyHeader(chain, header, false)
	if err != errInvalidTimestamp {
		t.Errorf("unexpected error comes, got: %v, expected: errInvalidTimestamp", err)
	}

	// future block
	block = makeBlockWithoutSeal(chain, engine, chain.Genesis())
	header = block.Header()
	header.Time = new(big.Int).Add(big.NewInt(now().Unix()), new(big.Int).SetUint64(10))
	err = engine.VerifyHeader(chain, header, false)
	if err != consensus.ErrFutureBlock {
		t.Errorf("unexpected error comes, got: %v, expected: consensus.ErrFutureBlock", err)
	}

	// invalid nonce
	block = makeBlockWithoutSeal(chain, engine, chain.Genesis())
	header = block.Header()
	copy(header.Nonce[:], hexutil.MustDecode("0x111111111111"))
	header.Number = big.NewInt(int64(engine.config.Epoch))
	err = engine.VerifyHeader(chain, header, false)
	if err != errInvalidNonce {
		t.Errorf("unexpected error comes, got: %v, expected: errInvalidCoinbase", err)
	}
}

func TestVerifySeal(t *testing.T) {
	chain, engine := newBlockChain(1)
	genesis := chain.Genesis()
	// cannot verify genesis
	err := engine.VerifySeal(chain, genesis.Header())
	if err != errUnknownBlock {
		t.Errorf("unexpected error comes, got: %v, expected: errUnknownBlock", err)
	}

	block := makeBlock(chain, engine, genesis)
	// change block content
	header := block.Header()
	header.Number = big.NewInt(4)
	block1 := block.WithSeal(header)
	err = engine.VerifySeal(chain, block1.Header())
	if err != errUnauthorized {
		t.Errorf("unexpected error comes, got: %v, expected: errUnauthorized", err)
	}

	// unauthorized users but still can get correct signer address
	engine.privateKey, _ = crypto.GenerateKey()
	err = engine.VerifySeal(chain, block.Header())
	if err != nil {
		t.Errorf("error should be nil, but got: %v", err)
	}
}

func TestVerifyHeaders(t *testing.T) {
	chain, engine := newBlockChain(1)
	genesis := chain.Genesis()

	// success case
	headers := []*types.Header{}
	blocks := []*types.Block{}
	size := 100
	for i := 0; i < size; i++ {
		if i == 0 {
			blocks = append(blocks, makeBlock(chain, engine, genesis))
		} else {
			b := makeBlockWithoutSeal(chain, engine, blocks[i-1])
			b, _ = engine.updateBlock(blocks[i-1].Header(), b)
			blocks = append(blocks, b)
		}
		headers = append(headers, blocks[i].Header())
	}
	now = func() time.Time {
		return time.Unix(headers[size-1].Time.Int64(), 0)
	}
	_, results := engine.VerifyHeaders(chain, headers, nil)
	const timeoutDura = 2 * time.Second
	timeout := time.NewTimer(timeoutDura)
	index := 0
OUT1:
	for {
		select {
		case err := <-results:
			if err != nil {
				t.Errorf("error should be nil, but got: %v", err)
				break OUT1
			}
			index++
			if index == size {
				break OUT1
			}
		case <-timeout.C:
			break OUT1
		}
	}
	// abort cases
	abort, results := engine.VerifyHeaders(chain, headers, nil)
	timeout = time.NewTimer(timeoutDura)
	index = 0
OUT2:
	for {
		select {
		case err := <-results:
			if err != nil {
				t.Errorf("error should be nil, but got: %v", err)
				break OUT2
			}
			index++
			if index == 5 {
				abort <- struct{}{}
			}
			if index >= size {
				t.Errorf("verifyheaders should be aborted")
				break OUT2
			}
		case <-timeout.C:
			break OUT2
		}
	}
	// error header cases
	headers[2].Number = big.NewInt(100)
	abort, results = engine.VerifyHeaders(chain, headers, nil)
	timeout = time.NewTimer(timeoutDura)
	index = 0
	errors := 0
	expectedErrors := 2
OUT3:
	for {
		select {
		case err := <-results:
			if err != nil {
				errors++
			}
			index++
			if index == size {
				if errors != expectedErrors {
					t.Errorf("Unexpected error number, got: %v, expected: %v", errors, expectedErrors)
				}
				break OUT3
			}
		case <-timeout.C:
			break OUT3
		}
	}
}

func TestSignaturePosition(t *testing.T) {
	validatorN := 2
	buf := make([]byte, 0)
	buf = append(buf, common.StringToHash("123").Bytes()...)
	buf = append(buf, byte(validatorN))
	buf = append(buf, make([]byte, validatorN*common.AddressLength)...)
	buf = append(buf, make([]byte, types.IstanbulExtraSeal)...)

	expectedStart := types.IstanbulExtraVanity + types.IstanbulExtraValidatorSize + validatorN*common.AddressLength
	expectedtEnd := expectedStart + types.IstanbulExtraSeal

	header := &types.Header{}
	header.Extra = buf

	start, end := signaturePosition(header)
	if expectedStart != start && expectedtEnd != end {
		t.Errorf("expected start: %v, got: %v, expected end: %v, got: %v", expectedStart, start, expectedtEnd, end)
	}
}

func TestValidExtra(t *testing.T) {

	testCases := []struct {
		extra         []byte
		expectedValid bool
	}{
		{
			// normal case
			func() []byte {
				validatorN := 4
				buf := make([]byte, 0)
				buf = append(buf, common.StringToHash("123").Bytes()...)
				buf = append(buf, byte(validatorN))
				buf = append(buf, make([]byte, validatorN*common.AddressLength)...)
				buf = append(buf, make([]byte, types.IstanbulExtraSeal)...)
				return buf
			}(),
			true,
		},
		{
			// missing validator
			func() []byte {
				validatorN := 4
				buf := make([]byte, 0)
				buf = append(buf, common.StringToHash("123").Bytes()...)
				buf = append(buf, byte(validatorN))
				buf = append(buf, make([]byte, types.IstanbulExtraSeal)...)
				return buf
			}(),
			false,
		},
		{
			// validator N is 0
			func() []byte {
				validatorN := 0
				buf := make([]byte, 0)
				buf = append(buf, common.StringToHash("123").Bytes()...)
				buf = append(buf, byte(validatorN))
				buf = append(buf, make([]byte, validatorN*common.AddressLength)...)
				buf = append(buf, make([]byte, types.IstanbulExtraSeal)...)
				return buf
			}(),
			false,
		},
		{
			// validator N is 0, but have 1 validator in field
			func() []byte {
				validatorN := 0
				buf := make([]byte, 0)
				buf = append(buf, common.StringToHash("123").Bytes()...)
				buf = append(buf, byte(validatorN))
				buf = append(buf, make([]byte, common.AddressLength)...)
				buf = append(buf, make([]byte, types.IstanbulExtraSeal)...)
				return buf
			}(),
			false,
		},
		{
			// missing seal
			func() []byte {
				validatorN := 4
				buf := make([]byte, 0)
				buf = append(buf, common.StringToHash("123").Bytes()...)
				buf = append(buf, byte(validatorN))
				buf = append(buf, make([]byte, validatorN*common.AddressLength)...)
				return buf
			}(),
			false,
		},
		{
			// missing few data
			func() []byte {
				buf := make([]byte, 0)
				return buf
			}(),
			false,
		},
	}

	b, _, _ := newSimpleBackend()

	for _, test := range testCases {
		header := &types.Header{}
		header.Extra = test.extra

		valid := b.validExtraFormat(header)
		if valid != test.expectedValid {
			t.Errorf("expected: %v, but: %v", test.expectedValid, valid)
		}
	}
}

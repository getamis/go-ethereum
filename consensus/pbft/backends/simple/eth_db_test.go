package simple

import (
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
)

type Foo struct {
	Name   string
	Number *big.Int
	Hash   common.Hash
}

func TestSaveAndRestore(t *testing.T) {
	db := newDBer(newTestDb())
	key := "key"
	foo := Foo{
		Name:   "name",
		Number: big.NewInt(9),
		Hash:   common.HexToHash("1234567890"),
	}
	err := db.Save(key, foo)
	if err != nil {
		t.Error("Should save the object correctly")
	}

	result := Foo{}
	err = db.Restore(key, &result)
	if err != nil {
		t.Error("Should restore the object correctly")
	}
	if !reflect.DeepEqual(foo, result) {
		t.Errorf("Should have the same data, result = %v, expected = %v", result, foo)
	}
}

func newTestDb() ethdb.Database {
	file := filepath.Join("/", "tmp", "pbfttesttmpfile")
	if common.FileExist(file) {
		os.RemoveAll(file)
	}
	db, _ := ethdb.NewLDBDatabase(file, 0, 0)

	return db
}

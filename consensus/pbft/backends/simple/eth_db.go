package simple

import (
	"encoding/json"
	"strings"

	"github.com/ethereum/go-ethereum/consensus/pbft"
	"github.com/ethereum/go-ethereum/ethdb"
)

const (
	prefixKey = "pbft"
)

type ethDB struct {
	db ethdb.Database
}

func newDBer(db ethdb.Database) pbft.Dber {
	return &ethDB{
		db: db,
	}
}

func (e *ethDB) Save(key string, val interface{}) error {
	blob, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return e.db.Put(append([]byte(e.getKey(key))), blob)
}

func (e *ethDB) Restore(key string, val interface{}) error {
	blob, err := e.db.Get([]byte(e.getKey(key)))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(blob, val); err != nil {
		return err
	}
	return nil
}

func (*ethDB) getKey(key string) string {
	return strings.Join([]string{prefixKey, key}, "_")
}

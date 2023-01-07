package storage

import (
	"bytes"

	"github.com/syndtr/goleveldb/leveldb"
)

// data collection prefixes for different data collections
const (
	colBlockByHash           byte = iota + 1 // block by hash
	colBlockHashByHeight                     // block hash by height
	colBlockHeight                           // last block height
	colLastQC                                // qc for last commited block to be used on restart
	colBlockCommitByHash                     // block commit by block hash
	colTxCount                               // total commited tx count
	colTxByHash                              // tx by hash
	colTxCommitByHash                        // tx commit info by tx hash
	colStateValueByKey                       // state value by state key
	colMerkleIndexByStateKey                 // tree leaf index by state key
	colMerkleTreeHeight                      // tree height
	colMerkleLeafCount                       // tree leaf count
	colMerkleNodeByPosition                  // tree node value by position
)

type setter interface {
	Set(key, value []byte) error
}

type updateFunc func(setter setter) error //包裹setter的更新函数

type getter interface {
	Get(key []byte) ([]byte, error)
	HasKey(key []byte) bool
}

func NewLevelDB(path string) (*leveldb.DB, error) {
	return leveldb.OpenFile(path, nil)
}

type levelDB struct {
	db *leveldb.DB
}

func (lg *levelDB) Get(key []byte) ([]byte, error) {
	return lg.db.Get(key, nil)
}

func (lg *levelDB) HasKey(key []byte) bool {
	_, err := lg.db.Get(key, nil)
	return err == nil
}

func (lg *levelDB) Set(key, value []byte) error {
	return lg.db.Put(key, value, nil)
}

func updateLevelDB(db *levelDB, fns []updateFunc) error {
	for _, fn := range fns {
		if err := fn(db); err != nil {
			return err
		}
	}
	return nil
}

func concatBytes(srcs ...[]byte) []byte {
	buf := bytes.NewBuffer(nil)
	size := 0
	for _, src := range srcs {
		size += len(src)
	}
	buf.Grow(size)
	for _, src := range srcs {
		buf.Write(src)
	}
	return buf.Bytes()
}

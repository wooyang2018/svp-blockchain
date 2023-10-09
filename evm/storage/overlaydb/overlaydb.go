// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package overlaydb

import (
	"crypto/sha256"

	"github.com/syndtr/goleveldb/leveldb/util"
	"github.com/wooyang2018/svp-blockchain/evm/common"
	storecomm "github.com/wooyang2018/svp-blockchain/storage/common"
)

type OverlayDB struct {
	store storecomm.PersistStore
	memdb *MemDB
	dbErr error
}

const initCap = 4 * 1024
const initkvNum = 128

func NewOverlayDB(store storecomm.PersistStore) *OverlayDB {
	return &OverlayDB{
		store: store,
		memdb: NewMemDB(initCap, initkvNum),
	}
}

func (self *OverlayDB) Reset() {
	self.memdb.Reset()
}

func (self *OverlayDB) Error() error {
	return self.dbErr
}

func (self *OverlayDB) SetError(err error) {
	self.dbErr = err
}

// if key is deleted, value == nil
func (self *OverlayDB) Get(key []byte) (value []byte, err error) {
	var unknown bool
	value, unknown = self.memdb.Get(key)
	if !unknown {
		return value, nil
	}

	value, err = self.store.Get(key)
	if err != nil {
		if err == storecomm.ErrNotFound {
			return nil, nil
		}
		self.dbErr = err
		return nil, err
	}

	return
}

func (self *OverlayDB) Put(key []byte, value []byte) {
	self.memdb.Put(key, value)
}

func (self *OverlayDB) Delete(key []byte) {
	self.memdb.Delete(key)
}

func (self *OverlayDB) CommitTo() {
	self.memdb.ForEach(func(key, val []byte) {
		if len(val) == 0 {
			self.store.BatchDelete(key)
		} else {
			self.store.BatchPut(key, val)
		}
	})
}

func (self *OverlayDB) GetWriteSet() *MemDB {
	return self.memdb
}

func (self *OverlayDB) ChangeHash() common.Uint256 {
	stateDiff := sha256.New()
	self.memdb.ForEach(func(key, val []byte) {
		stateDiff.Write(key)
		stateDiff.Write(val)
	})

	var hash common.Uint256
	stateDiff.Sum(hash[:0])
	return hash
}

// param key is referenced by iterator
func (self *OverlayDB) NewIterator(key []byte) storecomm.StoreIterator {
	prefixRange := util.BytesPrefix(key)
	backIter := self.store.NewIterator(key)
	memIter := self.memdb.NewIterator(prefixRange)

	return NewJoinIter(memIter, backIter)
}

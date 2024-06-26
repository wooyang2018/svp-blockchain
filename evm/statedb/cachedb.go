// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package statedb

import (
	ethcomm "github.com/ethereum/go-ethereum/common"
	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/wooyang2018/svp-blockchain/storage"
)

// CacheDB is smart contract execute cache, it contains transaction cache and block cache
// When smart contract execute finish, need to commit transaction cache to block cache
type CacheDB struct {
	memdb      *MemDB
	backend    storage.PersistStore
	keyScratch []byte
	dbErr      error
}

const initMemCap = 1024
const initKvNum = 16

// NewCacheDB return a new contract cache
func NewCacheDB(store storage.PersistStore) *CacheDB {
	return &CacheDB{
		backend: store,
		memdb:   NewMemDB(initMemCap, initKvNum),
	}
}

func (self *CacheDB) SetDbErr(err error) {
	self.dbErr = err
}

func (self *CacheDB) Reset() {
	self.memdb.Reset()
}

func ensureBuffer(b []byte, n int) []byte {
	if cap(b) < n {
		return make([]byte, n)
	}
	return b[:n]
}

func makePrefixedKey(dst []byte, prefix byte, key []byte) []byte {
	dst = ensureBuffer(dst, len(key)+1)
	dst[0] = prefix
	copy(dst[1:], key)
	return dst
}

// Commit current transaction cache to block cache
func (self *CacheDB) Commit() {
	self.memdb.ForEach(func(key, val []byte) {
		if len(val) == 0 {
			self.backend.Delete(key)
		} else {
			self.backend.Put(key, val)
		}
	})
	self.memdb.Reset()
}

func (self *CacheDB) Put(key []byte, value []byte) {
	self.put(storage.STORAGE, key, value)
}

func (self *CacheDB) GenAccountStateKey(addr ethcomm.Address, key []byte) []byte {
	k := make([]byte, 0, 1+20+32)
	k = append(k, byte(storage.STORAGE))
	k = append(k, addr[:]...)
	k = append(k, key[:]...)

	return k
}

func (self *CacheDB) put(prefix storage.DataEntryPrefix, key []byte, value []byte) {
	self.keyScratch = makePrefixedKey(self.keyScratch, byte(prefix), key)
	self.memdb.Put(self.keyScratch, value)
}

func (self *CacheDB) Get(key []byte) ([]byte, error) {
	return self.get(storage.STORAGE, key)
}

func (self *CacheDB) get(prefix storage.DataEntryPrefix, key []byte) ([]byte, error) {
	self.keyScratch = makePrefixedKey(self.keyScratch, byte(prefix), key)
	value, unknown := self.memdb.Get(self.keyScratch)
	if unknown {
		v, err := self.backend.Get(self.keyScratch)
		if err != nil {
			return nil, err
		}
		value = v
	}

	return value, nil
}

func (self *CacheDB) Delete(key []byte) {
	self.delete(storage.STORAGE, key)
}

// Delete item from cache
func (self *CacheDB) delete(prefix storage.DataEntryPrefix, key []byte) {
	self.keyScratch = makePrefixedKey(self.keyScratch, byte(prefix), key)
	self.memdb.Delete(self.keyScratch)
}

func (self *CacheDB) NewIterator(key []byte) storage.StoreIterator {
	pkey := make([]byte, 1+len(key))
	pkey[0] = byte(storage.STORAGE)
	copy(pkey[1:], key)
	prefixRange := util.BytesPrefix(pkey)
	backIter := self.backend.NewIterator(pkey)
	memIter := self.memdb.NewIterator(prefixRange)

	return &Iter{NewJoinIter(memIter, backIter)}
}

type Iter struct {
	*JoinIter
}

func (self *Iter) Key() []byte {
	key := self.JoinIter.Key()
	if len(key) != 0 {
		key = key[1:] // remove the first prefix
	}
	return key
}

func (self *CacheDB) CleanContractStorageData(addr ethcomm.Address) error {
	iter := self.NewIterator(addr[:])

	for has := iter.First(); has; has = iter.Next() {
		self.Delete(iter.Key())
	}
	iter.Release()

	return iter.Error()
}

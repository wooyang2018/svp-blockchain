// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package statedb

import (
	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/wooyang2018/svp-blockchain/evm/common"
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

func (self *CacheDB) GenAccountStateKey(addr common.Address, key []byte) []byte {
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

func (self *CacheDB) IsContractDestroyed(addr common.Address) (bool, error) {
	value, err := self.get(storage.DESTROYED, addr[:])
	if err != nil {
		return true, err
	}

	return len(value) != 0, nil
}

func (self *CacheDB) DeleteContract(address common.Address, height uint32) {
	self.delete(storage.CONTRACT, address[:])
	self.SetContractDestroyed(address, height)
}

func (self *CacheDB) SetContractDestroyed(addr common.Address, height uint32) {
	panic("todo")
}

func (self *CacheDB) UnsetContractDestroyed(addr common.Address, height uint32) {
	panic("todo")
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

func (self *CacheDB) MigrateContractStorage(oldAddress, newAddress common.Address, height uint32) error {
	self.DeleteContract(oldAddress, height)

	iter := self.NewIterator(oldAddress[:])
	for has := iter.First(); has; has = iter.Next() {
		key := iter.Key()
		val := iter.Value()

		newkey := serializeStorageKey(newAddress, key[20:])

		self.Put(newkey, val)
		self.Delete(key)
	}

	iter.Release()

	return iter.Error()
}

func (self *CacheDB) CleanContractStorage(addr common.Address, height uint32) error {
	self.DeleteContract(addr, height)
	return self.CleanContractStorageData(addr)
}

func (self *CacheDB) CleanContractStorageData(addr common.Address) error {
	iter := self.NewIterator(addr[:])

	for has := iter.First(); has; has = iter.Next() {
		self.Delete(iter.Key())
	}
	iter.Release()

	return iter.Error()
}

func serializeStorageKey(address common.Address, key []byte) []byte {
	res := make([]byte, 0, len(address[:])+len(key))
	res = append(res, address[:]...)
	res = append(res, key...)
	return res
}

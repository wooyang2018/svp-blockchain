/*
 * Copyright (C) 2018 The ontology Authors
 * This file is part of The ontology library.
 *
 * The ontology is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The ontology is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with The ontology.  If not, see <http://www.gnu.org/licenses/>.
 */

package storage

import (
	"github.com/syndtr/goleveldb/leveldb/util"
	"github.com/wooyang2018/svp-blockchain/evm/common"
	"github.com/wooyang2018/svp-blockchain/evm/common/payload"
	"github.com/wooyang2018/svp-blockchain/evm/storage/overlaydb"
	storecomm "github.com/wooyang2018/svp-blockchain/storage/common"
)

// CacheDB is smart contract execute cache, it contain transaction cache and block cache
// When smart contract execute finish, need to commit transaction cache to block cache
type CacheDB struct {
	memdb      *overlaydb.MemDB
	backend    *overlaydb.OverlayDB
	keyScratch []byte
}

const initCap = 1024
const initKvNum = 16

// NewCacheDB return a new contract cache
func NewCacheDB(store *overlaydb.OverlayDB) *CacheDB {
	return &CacheDB{
		backend: store,
		memdb:   overlaydb.NewMemDB(initCap, initKvNum),
	}
}

func (self *CacheDB) SetDbErr(err error) {
	self.backend.SetError(err)
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
	self.put(storecomm.STORAGE, key, value)
}

func (self *CacheDB) GenAccountStateKey(addr common.Address, key []byte) []byte {
	k := make([]byte, 0, 1+20+32)
	k = append(k, byte(storecomm.STORAGE))
	k = append(k, addr[:]...)
	k = append(k, key[:]...)

	return k
}

func (self *CacheDB) put(prefix storecomm.DataEntryPrefix, key []byte, value []byte) {
	self.keyScratch = makePrefixedKey(self.keyScratch, byte(prefix), key)
	self.memdb.Put(self.keyScratch, value)
}

func (self *CacheDB) GetContract(addr common.Address) (*payload.DeployCode, bool, error) {
	destroyed, err := self.IsContractDestroyed(addr)
	if err != nil {
		return nil, false, err
	}
	if destroyed {
		return nil, true, nil
	}

	value, err := self.get(storecomm.CONTRACT, addr[:])
	if err != nil {
		return nil, false, err
	}

	if len(value) == 0 {
		return nil, false, nil
	}

	contract := new(payload.DeployCode)
	if err := contract.Deserialization(common.NewZeroCopySource(value)); err != nil {
		return nil, false, err
	}
	return contract, false, nil
}

func (self *CacheDB) PutContract(contract *payload.DeployCode) {
	address := contract.Address()

	sink := common.NewZeroCopySink(nil)
	contract.Serialization(sink)

	value := sink.Bytes()
	self.put(storecomm.CONTRACT, address[:], value)
}

func (self *CacheDB) IsContractDestroyed(addr common.Address) (bool, error) {
	value, err := self.get(storecomm.DESTROYED, addr[:])
	if err != nil {
		return true, err
	}

	return len(value) != 0, nil
}

func (self *CacheDB) DeleteContract(address common.Address, height uint32) {
	self.delete(storecomm.CONTRACT, address[:])
	self.SetContractDestroyed(address, height)
}

func (self *CacheDB) SetContractDestroyed(addr common.Address, height uint32) {
	//TODO 待实现
}

func (self *CacheDB) UnsetContractDestroyed(addr common.Address, height uint32) {
	//TODO 待实现
}

func (self *CacheDB) Get(key []byte) ([]byte, error) {
	return self.get(storecomm.STORAGE, key)
}

func (self *CacheDB) get(prefix storecomm.DataEntryPrefix, key []byte) ([]byte, error) {
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
	self.delete(storecomm.STORAGE, key)
}

// Delete item from cache
func (self *CacheDB) delete(prefix storecomm.DataEntryPrefix, key []byte) {
	self.keyScratch = makePrefixedKey(self.keyScratch, byte(prefix), key)
	self.memdb.Delete(self.keyScratch)
}

func (self *CacheDB) NewIterator(key []byte) storecomm.StoreIterator {
	pkey := make([]byte, 1+len(key))
	pkey[0] = byte(storecomm.STORAGE)
	copy(pkey[1:], key)
	prefixRange := util.BytesPrefix(pkey)
	backIter := self.backend.NewIterator(pkey)
	memIter := self.memdb.NewIterator(prefixRange)

	return &Iter{overlaydb.NewJoinIter(memIter, backIter)}
}

type Iter struct {
	*overlaydb.JoinIter
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

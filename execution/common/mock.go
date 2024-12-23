// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package common

import "github.com/wooyang2018/svp-blockchain/storage"

type MemStateStore struct {
	Storage storage.PersistStore
}

func NewMemStateStore() *MemStateStore {
	return &MemStateStore{
		Storage: storage.NewMemLevelDBStore(),
	}
}

func (store *MemStateStore) VerifyState(key []byte) []byte {
	val, _ := store.Storage.Get(key)
	return val
}

func (store *MemStateStore) GetState(key []byte) []byte {
	val, _ := store.Storage.Get(key)
	return val
}

func (store *MemStateStore) SetState(key, value []byte) {
	store.Storage.Put(key, value)
}

type MockCallContext struct {
	*MemStateStore
	MockSender          []byte
	MockBlockHeight     uint64
	MockTransactionHash []byte
	MockBlockHash       []byte
	MockInput           []byte
}

var _ CallContext = (*MockCallContext)(nil)

func (wc *MockCallContext) Sender() []byte {
	return wc.MockSender
}

func (wc *MockCallContext) TransactionHash() []byte {
	return wc.MockTransactionHash
}

func (wc *MockCallContext) BlockHash() []byte {
	return wc.MockBlockHash
}

func (wc *MockCallContext) BlockHeight() uint64 {
	return wc.MockBlockHeight
}

func (wc *MockCallContext) Input() []byte {
	return wc.MockInput
}

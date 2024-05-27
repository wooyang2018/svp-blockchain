// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package statedb

import (
	"fmt"
	"io"
	"math/big"

	ethcomm "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/wooyang2018/svp-blockchain/evm/common"
	storecomm "github.com/wooyang2018/svp-blockchain/storage"
)

type OngBalanceHandle interface {
	SubBalance(cache *CacheDB, addr common.Address, val *big.Int) error
	AddBalance(cache *CacheDB, addr common.Address, val *big.Int) error
	SetBalance(cache *CacheDB, addr common.Address, val *big.Int) error
	GetBalance(cache *CacheDB, addr common.Address) (*big.Int, error)
}

type Dummy struct {
	Val map[common.Address]*big.Int
}

func NewDummy() Dummy {
	d := Dummy{}
	d.Val = make(map[common.Address]*big.Int)
	return d
}

func (d Dummy) SubBalance(cache *CacheDB, addr common.Address, val *big.Int) error {
	if _, ok := d.Val[addr]; !ok {
		d.Val[addr] = big.NewInt(0)
	}
	d.Val[addr].Sub(d.Val[addr], val)
	return nil
}

func (d Dummy) AddBalance(cache *CacheDB, addr common.Address, val *big.Int) error {
	if _, ok := d.Val[addr]; !ok {
		d.Val[addr] = big.NewInt(0)
	}
	d.Val[addr].Add(d.Val[addr], val)
	return nil
}

func (d Dummy) SetBalance(cache *CacheDB, addr common.Address, val *big.Int) error {
	d.Val[addr] = val
	return nil
}

func (d Dummy) GetBalance(cache *CacheDB, addr common.Address) (*big.Int, error) {
	if _, ok := d.Val[addr]; !ok {
		d.Val[addr] = big.NewInt(0)
	}
	return d.Val[addr], nil
}

type StateDB struct {
	cacheDB          *CacheDB
	Suicided         map[ethcomm.Address]bool
	logs             []*common.StorageLog
	thash, bhash     ethcomm.Hash
	txIndex          int
	refund           uint64
	snapshots        []*snapshot
	OngBalanceHandle OngBalanceHandle
}

func NewStateDB(cacheDB *CacheDB, thash, bhash ethcomm.Hash, balanceHandle OngBalanceHandle) *StateDB {
	return &StateDB{
		cacheDB:          cacheDB,
		Suicided:         make(map[ethcomm.Address]bool),
		logs:             nil,
		thash:            thash,
		bhash:            bhash,
		refund:           0,
		snapshots:        nil,
		OngBalanceHandle: balanceHandle,
	}
}

func (self *StateDB) Prepare(thash, bhash ethcomm.Hash) {
	self.thash = thash
	self.bhash = bhash
	//	s.accessList = newAccessList()
}

func (self *StateDB) BlockHash() ethcomm.Hash {
	return self.bhash
}

func (self *StateDB) GetLogs() []*common.StorageLog {
	return self.logs
}

func (self *StateDB) Commit() error {
	err := self.CommitToCacheDB()
	if err != nil {
		return err
	}
	self.cacheDB.Commit()
	return nil
}

func (self *StateDB) CommitToCacheDB() error {
	for addr := range self.Suicided {
		self.cacheDB.DelEthAccount(addr) //todo : check consistence with ethereum
		err := self.cacheDB.CleanContractStorageData(common.Address(addr))
		if err != nil {
			return err
		}
	}

	self.Suicided = make(map[ethcomm.Address]bool)
	self.snapshots = self.snapshots[:0]

	return nil
}

type snapshot struct {
	changes  *MemDB
	suicided map[ethcomm.Address]bool
	logsSize int
	refund   uint64
}

func (self *StateDB) AddRefund(gas uint64) {
	self.refund += gas
}

// SubRefund removes gas from the refund counter.
// This method will panic if the refund counter goes below zero
func (self *StateDB) SubRefund(gas uint64) {
	if gas > self.refund {
		panic(fmt.Sprintf("Refund counter below zero (gas: %d > refund: %d)", gas, self.refund))
	}

	self.refund -= gas
}

func genKey(contract ethcomm.Address, key ethcomm.Hash) []byte {
	var result []byte
	result = append(result, contract.Bytes()...)
	result = append(result, key.Bytes()...)
	return result
}

func (self *StateDB) GetState(contract ethcomm.Address, key ethcomm.Hash) ethcomm.Hash {
	val, err := self.cacheDB.Get(genKey(contract, key))
	if err != nil {
		self.cacheDB.SetDbErr(err)
	}

	return ethcomm.BytesToHash(val)
}

// GetRefund returns the current value of the refund counter.
func (self *StateDB) GetRefund() uint64 {
	return self.refund
}

func (self *StateDB) SetState(contract ethcomm.Address, key, value ethcomm.Hash) {
	self.cacheDB.Put(genKey(contract, key), value[:])
}

func (self *StateDB) GetCommittedState(addr ethcomm.Address, key ethcomm.Hash) ethcomm.Hash {
	k := self.cacheDB.GenAccountStateKey(common.Address(addr), key[:])
	val, err := self.cacheDB.backend.Get(k)
	if err != nil {
		self.cacheDB.SetDbErr(err)
	}

	return ethcomm.BytesToHash(val)
}

type EthAccount struct {
	Nonce    uint64
	CodeHash ethcomm.Hash
}

func (self *EthAccount) IsEmpty() bool {
	return self.Nonce == 0 && self.CodeHash == ethcomm.Hash{}
}

func (self *EthAccount) IsEmptyContract() bool {
	return self.CodeHash == ethcomm.Hash{}
}

func (self *EthAccount) Serialization(sink *common.ZeroCopySink) {
	sink.WriteUint64(self.Nonce)
	sink.WriteHash(common.Uint256(self.CodeHash))
}

func (self *EthAccount) Deserialization(source *common.ZeroCopySource) error {
	nonce, _ := source.NextUint64()
	hash, eof := source.NextHash()
	if eof {
		return io.ErrUnexpectedEOF
	}
	self.Nonce = nonce
	self.CodeHash = ethcomm.Hash(hash)

	return nil
}

func (self *CacheDB) GetEthAccount(addr ethcomm.Address) (val EthAccount, err error) {
	value, err := self.get(storecomm.ETH_ACCOUNT, addr[:])
	if err != nil {
		return val, err
	}

	if len(value) == 0 {
		return val, nil
	}

	err = val.Deserialization(common.NewZeroCopySource(value))

	return val, err
}

func (self *CacheDB) PutEthAccount(addr ethcomm.Address, val EthAccount) {
	var raw []byte
	if !val.IsEmpty() {
		raw = common.SerializeToBytes(&val)
	}

	self.put(storecomm.ETH_ACCOUNT, addr[:], raw)
}

func (self *CacheDB) DelEthAccount(addr ethcomm.Address) {
	self.put(storecomm.ETH_ACCOUNT, addr[:], nil)
}

func (self *CacheDB) GetEthCode(codeHash ethcomm.Hash) (val []byte, err error) {
	return self.get(storecomm.ETH_CODE, codeHash[:])
}

func (self *CacheDB) PutEthCode(codeHash ethcomm.Hash, val []byte) {
	self.put(storecomm.ETH_CODE, codeHash[:], val)
}

func (self *StateDB) getEthAccount(addr ethcomm.Address) (val EthAccount) {
	account, err := self.cacheDB.GetEthAccount(addr)
	if err != nil {
		self.cacheDB.SetDbErr(err)
		return val
	}

	return account
}

func (self *StateDB) GetNonce(addr ethcomm.Address) uint64 {
	return self.getEthAccount(addr).Nonce
}

func (self *StateDB) SetNonce(addr ethcomm.Address, nonce uint64) {
	account := self.getEthAccount(addr)
	account.Nonce = nonce
	self.cacheDB.PutEthAccount(addr, account)
}

func (self *StateDB) GetCodeHash(addr ethcomm.Address) (hash ethcomm.Hash) {
	return self.getEthAccount(addr).CodeHash
}

func (self *StateDB) GetCode(addr ethcomm.Address) []byte {
	hash := self.GetCodeHash(addr)
	code, err := self.cacheDB.GetEthCode(hash)
	if err != nil {
		self.cacheDB.SetDbErr(err)
		return nil
	}

	return code
}

func (self *StateDB) SetCode(addr ethcomm.Address, code []byte) {
	codeHash := crypto.Keccak256Hash(code)
	account := self.getEthAccount(addr)
	account.CodeHash = codeHash
	self.cacheDB.PutEthAccount(addr, account)
	self.cacheDB.PutEthCode(codeHash, code)
}

func (self *StateDB) GetCodeSize(addr ethcomm.Address) int {
	// todo : add cache to speed up
	return len(self.GetCode(addr))
}

func (self *StateDB) Suicide(addr ethcomm.Address) bool {
	acct := self.getEthAccount(addr)
	if acct.IsEmpty() {
		return false
	}
	self.Suicided[addr] = true
	err := self.OngBalanceHandle.SetBalance(self.cacheDB, common.Address(addr), big.NewInt(0))
	if err != nil {
		self.cacheDB.SetDbErr(err)
	}
	return true
}

func (self *StateDB) HasSuicided(addr ethcomm.Address) bool {
	return self.Suicided[addr]
}

func (self *StateDB) Exist(addr ethcomm.Address) bool {
	if self.Suicided[addr] {
		return true
	}
	acct := self.getEthAccount(addr)
	balance, err := self.OngBalanceHandle.GetBalance(self.cacheDB, common.Address(addr))
	if err != nil {
		self.cacheDB.SetDbErr(err)
		return false
	}
	if !acct.IsEmpty() || balance.Sign() > 0 {
		return true
	}

	return false
}

func (self *StateDB) Empty(addr ethcomm.Address) bool {
	acct := self.getEthAccount(addr)

	balance, err := self.OngBalanceHandle.GetBalance(self.cacheDB, common.Address(addr))
	if err != nil {
		self.cacheDB.SetDbErr(err)
		return false
	}

	return acct.IsEmpty() && balance.Sign() == 0
}

func (self *StateDB) AddLog(log *common.StorageLog) {
	self.logs = append(self.logs, log)
}

func (self *StateDB) AddPreimage(ethcomm.Hash, []byte) {
	// todo
}

func (self *StateDB) ForEachStorage(ethcomm.Address, func(ethcomm.Hash, ethcomm.Hash) bool) error {
	panic("todo")
}

func (self *StateDB) CreateAccount(address ethcomm.Address) {
	return
}

func (self *StateDB) Snapshot() int {
	changes := self.cacheDB.memdb.DeepClone()
	suicided := make(map[ethcomm.Address]bool)
	for k, v := range self.Suicided {
		suicided[k] = v
	}

	sn := &snapshot{
		changes:  changes,
		suicided: suicided,
		logsSize: len(self.logs),
		refund:   self.refund,
	}

	self.snapshots = append(self.snapshots, sn)

	return len(self.snapshots) - 1
}

func (self *StateDB) RevertToSnapshot(idx int) {
	if idx+1 > len(self.snapshots) {
		panic("can not to revert snapshot")
	}

	sn := self.snapshots[idx]

	self.snapshots = self.snapshots[:idx]
	self.cacheDB.memdb = sn.changes
	self.Suicided = sn.suicided
	self.refund = sn.refund
	self.logs = self.logs[:sn.logsSize]
}

func (self *StateDB) SubBalance(addr ethcomm.Address, val *big.Int) {
	err := self.OngBalanceHandle.SubBalance(self.cacheDB, common.Address(addr), val)
	if err != nil {
		self.cacheDB.SetDbErr(err)
		return
	}
}

func (self *StateDB) AddBalance(addr ethcomm.Address, val *big.Int) {
	err := self.OngBalanceHandle.AddBalance(self.cacheDB, common.Address(addr), val)
	if err != nil {
		self.cacheDB.SetDbErr(err)
		return
	}
}

func (self *StateDB) GetBalance(addr ethcomm.Address) *big.Int {
	balance, err := self.OngBalanceHandle.GetBalance(self.cacheDB, common.Address(addr))
	if err != nil {
		self.cacheDB.SetDbErr(err)
		return big.NewInt(0)
	}

	return balance
}

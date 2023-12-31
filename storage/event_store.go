// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/wooyang2018/svp-blockchain/evm/common"
	"github.com/wooyang2018/svp-blockchain/logger"
)

// UseNumber can be set to true to enable the use of json.Number when decoding
// event state.
var UseNumber = true

// eventStore saves event notifies gen by smart contract execution
type eventStore struct {
	dbDir string        //Store path
	store *LevelDBStore //Store handler
}

// NewEventStore return event store instance
func NewEventStore(dbDir string) (*eventStore, error) {
	store, err := NewLevelDBStore(dbDir)
	if err != nil {
		return nil, err
	}
	return &eventStore{
		dbDir: dbDir,
		store: store,
	}, nil
}

// NewBatch start event commit batch
func (this *eventStore) NewBatch() {
	this.store.NewBatch()
}

// SaveEventNotifyByTx persist event notify by transaction hash
func (this *eventStore) SaveEventNotifyByTx(txHash common.Uint256, notify *ExecuteNotify) error {
	result, err := json.Marshal(notify)
	if err != nil {
		return fmt.Errorf("json.Marshal error %s", err)
	}
	key := genEventNotifyByTxKey(txHash)
	this.store.BatchPut(key, result)
	return nil
}

// SaveEventNotifyByBlock persist transaction hash which have event notify to store
func (this *eventStore) SaveEventNotifyByBlock(height uint32, txHashs []common.Uint256) {
	key := genEventNotifyByBlockKey(height)
	values := common.NewZeroCopySink(nil)
	values.WriteUint32(uint32(len(txHashs)))
	for _, txHash := range txHashs {
		values.WriteHash(txHash)
	}
	this.store.BatchPut(key, values.Bytes())
}

// GetEventNotifyByTx return event notify by trasanction hash
func (this *eventStore) GetEventNotifyByTx(txHash common.Uint256) (*ExecuteNotify, error) {
	key := genEventNotifyByTxKey(txHash)
	data, err := this.store.Get(key)
	if err != nil {
		return nil, err
	}
	var notify ExecuteNotify
	if UseNumber {
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		if err = dec.Decode(&notify); err != nil {
			return nil, fmt.Errorf("json.Unmarshal error %s", err)
		}
	} else if err = json.Unmarshal(data, &notify); err != nil {
		return nil, fmt.Errorf("json.Unmarshal error %s", err)
	}
	return &notify, nil
}

// GetEventNotifyByBlock return all event notify of transaction in block
func (this *eventStore) GetEventNotifyByBlock(height uint32) ([]*ExecuteNotify, error) {
	key := genEventNotifyByBlockKey(height)
	data, err := this.store.Get(key)
	if err != nil {
		return nil, err
	}
	reader := bytes.NewBuffer(data)
	size, err := common.ReadUint32(reader)
	if err != nil {
		return nil, fmt.Errorf("ReadUint32 error %s", err)
	}
	evtNotifies := make([]*ExecuteNotify, 0)
	for i := uint32(0); i < size; i++ {
		var txHash common.Uint256
		err = txHash.Deserialize(reader)
		if err != nil {
			return nil, fmt.Errorf("txHash.Deserialize error %s", err)
		}
		evtNotify, err := this.GetEventNotifyByTx(txHash)
		if err != nil {
			logger.I().Errorf("getEventNotifyByTx Height:%d by txhash:%s error:%s", height, txHash.ToHexString(), err)
			continue
		}
		evtNotifies = append(evtNotifies, evtNotify)
	}
	return evtNotifies, nil
}

func (this *eventStore) PruneBlock(height uint32, hashes []common.Uint256) {
	key := genEventNotifyByBlockKey(height)
	this.store.BatchDelete(key)
	for _, hash := range hashes {
		this.store.BatchDelete(genEventNotifyByTxKey(hash))
	}
}

// Commit event store batch to store
func (this *eventStore) Commit() error {
	return this.store.BatchCommit()
}

// Close event store
func (this *eventStore) Close() error {
	return this.store.Close()
}

// ClearAll all data in event store
func (this *eventStore) ClearAll() error {
	this.NewBatch()
	iter := this.store.NewIterator(nil)
	for iter.Next() {
		this.store.BatchDelete(iter.Key())
	}
	iter.Release()
	if err := iter.Error(); err != nil {
		return err
	}
	return this.Commit()
}

// SaveCurrentBlock persist current block height and block hash to event store
func (this *eventStore) SaveCurrentBlock(height uint32, blockHash common.Uint256) {
	key := this.getCurrentBlockKey()
	value := common.NewZeroCopySink(nil)
	value.WriteHash(blockHash)
	value.WriteUint32(height)
	this.store.BatchPut(key, value.Bytes())
}

// GetCurrentBlock return current block hash, and block height
func (this *eventStore) GetCurrentBlock() (common.Uint256, uint32, error) {
	key := this.getCurrentBlockKey()
	data, err := this.store.Get(key)
	if err != nil {
		return common.Uint256{}, 0, err
	}
	reader := bytes.NewReader(data)
	blockHash := common.Uint256{}
	err = blockHash.Deserialize(reader)
	if err != nil {
		return common.Uint256{}, 0, err
	}
	height, err := common.ReadUint32(reader)
	if err != nil {
		return common.Uint256{}, 0, err
	}
	return blockHash, height, nil
}

func (this *eventStore) getCurrentBlockKey() []byte {
	return []byte{byte(SYS_CURRENT_BLOCK)}
}

func genEventNotifyByBlockKey(height uint32) []byte {
	key := make([]byte, 5, 5)
	key[0] = byte(EVENT_NOTIFY)
	binary.LittleEndian.PutUint32(key[1:], height)
	return key
}

func genEventNotifyByTxKey(txHash common.Uint256) []byte {
	data := txHash.ToArray()
	key := make([]byte, 1+len(data))
	key[0] = byte(EVENT_NOTIFY)
	copy(key[1:], data)
	return key
}

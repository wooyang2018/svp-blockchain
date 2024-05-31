// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package storage

type DataEntryPrefix byte

// data collection prefixes for different data collections
const (
	BLOCK_BY_HASH             DataEntryPrefix = iota + 1 // block by hash
	BLOCK_HASH_BY_HEIGHT                                 // block hash by height
	BLOCK_HEIGHT                                         // last block height
	QC_BY_BLOCK_HASH                                     // qc by block hash
	LAST_QC_BLOCK_HASH                                   // qc for last committed block to be used on restart
	BLOCK_COMMIT_BY_HASH                                 // block commit by block hash
	TX_COUNT                                             // total committed tx count
	TX_BY_HASH                                           // tx by hash
	TX_COMMIT_BY_HASH                                    // tx commit info by tx hash
	STATE_VALUE_BY_KEY                                   // state value by state key
	MERKLE_INDEX_BY_STATE_KEY                            // tree leaf index by state key
	MERKLE_TREE_HEIGHT                                   // tree height
	MERKLE_LEAF_COUNT                                    // tree leaf count
	MERKLE_NODE_BY_POSITION                              // tree node value by position
	BOOK_KEEPER                                          // BookKeeper state key prefix
	CONTRACT                                             // Smart contract deploy code key prefix
	STORAGE                                              // Smart contract storage key prefix
	DESTROYED                                            // record destroyed smart contract: prefix+address -> height
	ETH_CODE                                             // eth contract code:hash -> bytes
	ETH_ACCOUNT                                          // eth account: address -> [nonce, codeHash]
	ETH_FILTER_START                                     // support eth filter height
)

type StoreIterator interface {
	Next() bool    // Next item. If item available return true, otherwise return false
	First() bool   // First item. If item available return true, otherwise return false
	Key() []byte   // Return the current item key
	Value() []byte // Return the current item value
	Release()      // Close iterator
	Error() error  // Error returns any accumulated error.
}

// PersistStore of ledger
type PersistStore interface {
	Put(key []byte, value []byte) error      // Put the key-value pair to store
	Get(key []byte) ([]byte, error)          // Get the value if key in store
	Has(key []byte) (bool, error)            // Whether the key is exist in store
	Delete(key []byte) error                 // Delete the key in store
	NewBatch()                               // Start commit batch
	BatchPut(key []byte, value []byte)       // Put a key-value pair to batch
	BatchDelete(key []byte)                  // Delete the key in batch
	BatchCommit() error                      // Commit batch to store
	Close() error                            // Close store
	NewIterator(prefix []byte) StoreIterator // Return the iterator of store
}

type Setter interface {
	Put(key []byte, value []byte) error // Put the key-value pair to store
}

type Getter interface {
	Get(key []byte) ([]byte, error)
	Has(key []byte) (bool, error) // Whether the key is exist in store
}

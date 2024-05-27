// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package storage

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
	Put(key []byte, value []byte) error      //Put the key-value pair to store
	Get(key []byte) ([]byte, error)          //Get the value if key in store
	Has(key []byte) (bool, error)            //Whether the key is exist in store
	Delete(key []byte) error                 //Delete the key in store
	NewBatch()                               //Start commit batch
	BatchPut(key []byte, value []byte)       //Put a key-value pair to batch
	BatchDelete(key []byte)                  //Delete the key in batch
	BatchCommit() error                      //Commit batch to store
	Close() error                            //Close store
	NewIterator(prefix []byte) StoreIterator //Return the iterator of store
}

type Setter interface {
	Put(key []byte, value []byte) error //Put the key-value pair to store
}

type Getter interface {
	Get(key []byte) ([]byte, error)
	Has(key []byte) (bool, error) //Whether the key is exist in store
}

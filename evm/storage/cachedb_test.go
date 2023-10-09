// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package storage

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wooyang2018/svp-blockchain/evm/storage/overlaydb"
	"github.com/wooyang2018/svp-blockchain/storage/common"
	"github.com/wooyang2018/svp-blockchain/storage/leveldb"
)

func genRandKeyVal() (string, string) {
	p := make([]byte, 100)
	rand.Read(p)
	key := string(p)
	rand.Read(p)
	val := string(p)
	return key, val
}

func TestCacheDB(t *testing.T) {
	N := 10000
	mem := make(map[string]string)
	memback := leveldb.NewMemLevelDBStore()
	overlay := overlaydb.NewOverlayDB(memback)

	cache := NewCacheDB(overlay)
	for i := 0; i < N; i++ {
		key, val := genRandKeyVal()
		cache.Put([]byte(key), []byte(val))
		mem[key] = val
	}

	for key := range mem {
		op := rand.Int() % 2
		if op == 0 {
			//delete
			delete(mem, key)
			cache.Delete([]byte(key))
		} else if op == 1 {
			//update
			_, val := genRandKeyVal()
			mem[key] = val
			cache.Put([]byte(key), []byte(val))
		}
	}

	for key, val := range mem {
		value, err := cache.Get([]byte(key))
		assert.Nil(t, err)
		assert.NotNil(t, value)
		assert.Equal(t, []byte(val), value)
	}
	cache.Commit()

	prefix := common.STORAGE
	for key, val := range mem {
		pkey := make([]byte, 1+len(key))
		pkey[0] = byte(prefix)
		copy(pkey[1:], key)
		raw, err := overlay.Get(pkey)
		assert.Nil(t, err)
		assert.NotNil(t, raw)
		assert.Equal(t, []byte(val), raw)
	}

}

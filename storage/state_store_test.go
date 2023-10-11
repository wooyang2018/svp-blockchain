// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package storage

import (
	"crypto"
	"math/big"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	_ "golang.org/x/crypto/sha3"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/merkle"
)

const hashFunc = crypto.SHA3_256

func TestStateStore_loadPrevValuesAndTreeIndexes(t *testing.T) {
	asrt := assert.New(t)

	dir, _ := os.MkdirTemp("", "db")
	db, _ := NewLevelDBStore(dir)
	ss := &stateStore{db, hashFunc, 20}

	updfns := make([]updateFunc, 3)
	updfns[0] = ss.setState([]byte{1}, []byte{100})
	updfns[1] = ss.setState([]byte{2}, []byte{200})
	updfns[2] = ss.setTreeIndex([]byte{1}, big.NewInt(9).Bytes())
	updateLevelDB(db, updfns)

	scList := []*core.StateChange{
		core.NewStateChange().SetKey([]byte{1}),
		core.NewStateChange().SetKey([]byte{2}),
	}

	ss.loadPrevTreeIndexes(scList)
	ss.loadPrevValues(scList)

	asrt.Equal([]byte{100}, scList[0].PrevValue())
	asrt.Equal([]byte{200}, scList[1].PrevValue())
	asrt.Equal(big.NewInt(9).Bytes(), scList[0].PrevTreeIndex())
	asrt.Nil(scList[1].PrevTreeIndex())
}

func TestStateStore_updateState(t *testing.T) {
	asrt := assert.New(t)

	dir, _ := os.MkdirTemp("", "db")
	db, _ := NewLevelDBStore(dir)
	ss := &stateStore{db, hashFunc, 20}

	upd := core.NewStateChange().
		SetKey([]byte{1}).
		SetValue([]byte{2}).
		SetTreeIndex([]byte{1})
	asrt.Nil(ss.getStateNotFoundNil(upd.Key()))

	updateLevelDB(db, ss.commitStateChange(upd))

	asrt.Equal(upd.Value(), ss.getStateNotFoundNil(upd.Key()))

	idx, err := ss.getMerkleIndex(upd.Key())
	asrt.NoError(err)
	asrt.Equal(upd.TreeIndex(), idx)
}

func TestStateStore_computeUpdatedTreeNodes(t *testing.T) {
	asrt := assert.New(t)

	scList := []*core.StateChange{
		core.NewStateChange().
			SetKey([]byte{1}).SetValue([]byte{10}).SetTreeIndex([]byte{9}),
		core.NewStateChange().
			SetKey([]byte{2}).SetValue([]byte{20}).SetTreeIndex([]byte{12}),
	}

	ss := &stateStore{
		hashFunc:        hashFunc,
		concurrentLimit: 20,
	}
	nodes := ss.computeUpdatedTreeNodes(scList)

	p0 := merkle.NewPosition(0, big.NewInt(9))
	p1 := merkle.NewPosition(0, big.NewInt(12))
	asrt.Equal(p0.Bytes(), nodes[0].Position.Bytes())
	asrt.Equal(p1.Bytes(), nodes[1].Position.Bytes())

	d0 := ss.sumStateValue([]byte{10})
	d1 := ss.sumStateValue([]byte{20})
	asrt.Equal(d0, nodes[0].Data)
	asrt.Equal(d1, nodes[1].Data)
}

func TestStateStore_setNewTreeIndexes(t *testing.T) {
	asrt := assert.New(t)

	ss := &stateStore{hashFunc: hashFunc}
	scList := []*core.StateChange{
		core.NewStateChange().
			SetKey([]byte{1}).SetValue([]byte{10}).
			SetPrevTreeIndex(big.NewInt(9).Bytes()),
		core.NewStateChange().
			SetKey([]byte{2}).SetValue([]byte{20}),
	}
	newLeafCount := ss.setNewTreeIndexes(scList, big.NewInt(12))

	asrt.Equal(big.NewInt(13).Bytes(), newLeafCount.Bytes())
	asrt.Equal(scList[0].PrevTreeIndex(), scList[0].TreeIndex())
	asrt.Nil(scList[1].PrevTreeIndex())
	asrt.Equal(big.NewInt(12).Bytes(), scList[1].TreeIndex())
}

// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package storage

import (
	"math/big"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wooyang2018/svp-blockchain/core"
)

func newTestStorage() *Storage {
	dir, _ := os.MkdirTemp("", "db")
	return New(dir, DefaultConfig)
}

func TestStorageStateZero(t *testing.T) {
	asrt := assert.New(t)
	strg := newTestStorage()
	asrt.Nil(strg.GetMerkleRoot())
	_, err := strg.GetLastBlock()
	asrt.Error(err)
	asrt.Nil(strg.GetState([]byte("some key")))
}

func TestStorageCommit(t *testing.T) {
	asrt := assert.New(t)
	strg := newTestStorage()

	priv := core.GenerateKey(nil)
	b0 := core.NewBlock().SetHeight(0).Sign(priv)
	bcmInput := core.NewBlockCommit().
		SetHash(b0.Hash()).
		SetStateChanges([]*core.StateChange{
			core.NewStateChange().SetKey([]byte{1}).SetValue([]byte{10}),
			core.NewStateChange().SetKey([]byte{2}).SetValue([]byte{20}),
		})
	data := &CommitData{
		Block:       b0,
		BlockCommit: bcmInput,
	}
	err := strg.Commit(data)
	asrt.NoError(err)

	blkHeight := strg.GetBlockHeight()
	asrt.EqualValues(0, blkHeight)

	blk, err := strg.GetLastBlock()
	asrt.NoError(err)
	asrt.Equal(b0.Hash(), blk.Hash())

	blk, err = strg.GetBlock(b0.Hash())
	asrt.NoError(err)
	asrt.Equal(b0.Hash(), blk.Hash())

	blk, err = strg.GetBlockByHeight(0)
	asrt.NoError(err)
	asrt.Equal(b0.Hash(), blk.Hash())

	bcm, err := strg.GetBlockCommit(b0.Hash())
	asrt.NoError(err)
	asrt.Equal([]byte{0}, bcm.StateChanges()[0].TreeIndex())
	asrt.Equal(big.NewInt(1).Bytes(), bcm.StateChanges()[1].TreeIndex())
	asrt.Equal(big.NewInt(2).Bytes(), bcm.LeafCount())

	h := hashFunc.New()
	h.Write(strg.stateStore.sumStateValue([]byte{10}))
	h.Write(strg.stateStore.sumStateValue([]byte{20}))
	mroot := h.Sum(nil)
	h.Reset()
	asrt.Equal(mroot, bcm.MerkleRoot())
	asrt.Equal(bcm.MerkleRoot(), strg.GetMerkleRoot())
	asrt.Equal([]byte{10}, strg.GetState([]byte{1}))
	asrt.Equal([]byte{20}, strg.GetState([]byte{2}))

	b1 := core.NewBlock().
		SetHeight(1).
		SetParentHash(b0.Hash()).
		SetMerkleRoot(strg.GetMerkleRoot()).
		Sign(priv)
	tx1 := core.NewTransaction().SetNonce(1).Sign(priv)
	tx2 := core.NewTransaction().SetNonce(2).Sign(priv)
	txc1 := core.NewTxCommit().SetHash(tx1.Hash())
	txc2 := core.NewTxCommit().SetHash(tx2.Hash())

	bcmInput = core.NewBlockCommit().
		SetHash(b1.Hash()).
		SetStateChanges([]*core.StateChange{
			core.NewStateChange().SetKey([]byte{1}).SetValue([]byte{20}),
			// new key, leaf index -> 3, increase leaf count -> 4
			core.NewStateChange().SetKey([]byte{5}).SetValue([]byte{50}),
			// new key, leaf index -> 2, increase leaf count -> 3
			core.NewStateChange().SetKey([]byte{3}).SetValue([]byte{30}),
		})
	data = &CommitData{
		Block:        b1,
		Transactions: []*core.Transaction{tx1, tx2},
		TxCommits:    []*core.TxCommit{txc1, txc2},
		BlockCommit:  bcmInput,
	}
	err = strg.Commit(data)
	asrt.NoError(err)

	blkHeight = strg.GetBlockHeight()
	asrt.NoError(err)
	asrt.EqualValues(1, blkHeight)

	blk, err = strg.GetLastBlock()
	asrt.NoError(err)
	asrt.Equal(b1.Hash(), blk.Hash())

	tx, err := strg.GetTx(tx1.Hash())
	asrt.NoError(err)
	asrt.Equal(tx1.Nonce(), tx.Nonce())

	asrt.True(strg.HasTx(tx2.Hash()))

	txc, err := strg.GetTxCommit(tx2.Hash())
	asrt.NoError(err)
	asrt.Equal(txc2.Hash(), txc.Hash())

	bcm, err = strg.GetBlockCommit(b1.Hash())
	asrt.NoError(err)
	asrt.Equal([]byte{0}, bcm.StateChanges()[0].PrevTreeIndex())
	asrt.Equal(big.NewInt(3).Bytes(), bcm.StateChanges()[1].TreeIndex())
	asrt.Equal(big.NewInt(2).Bytes(), bcm.StateChanges()[2].TreeIndex())
	asrt.Equal(big.NewInt(4).Bytes(), bcm.LeafCount())

	h.Write(strg.stateStore.sumStateValue([]byte{20}))
	h.Write(strg.stateStore.sumStateValue([]byte{20}))
	h.Write(strg.stateStore.sumStateValue([]byte{30}))
	h.Write(strg.stateStore.sumStateValue([]byte{50}))
	mroot = h.Sum(nil)
	h.Reset()
	asrt.Equal(mroot, bcm.MerkleRoot())
	asrt.Equal(bcm.MerkleRoot(), strg.GetMerkleRoot())

	asrt.Equal([]byte{50}, strg.GetState([]byte{5}))
	var value []byte
	asrt.NotPanics(func() {
		value = strg.VerifyState([]byte{5})
	})
	asrt.Equal([]byte{50}, value)

	asrt.NotPanics(func() {
		// non existing state value
		value = strg.VerifyState([]byte{10})
	})
	asrt.Nil(value)

	// tampering state value
	updFn := strg.stateStore.setState([]byte{5}, []byte{100})
	updateLevelDB(strg.PersistStore, []updateFunc{updFn})

	// should panic
	asrt.Panics(func() {
		value = strg.VerifyState([]byte{5})
	})
	asrt.Nil(value)
}

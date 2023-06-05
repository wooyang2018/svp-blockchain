// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package storage

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wooyang2018/posv-blockchain/core"
)

func TestChainStore(t *testing.T) {
	asrt := assert.New(t)

	dir, _ := os.MkdirTemp("", "db")
	rawDB, _ := NewLevelDB(dir)
	db := &levelDB{rawDB}
	cs := &chainStore{db}

	priv := core.GenerateKey(nil)
	blk := core.NewBlock().
		SetHeight(4).
		SetParentHash([]byte{1}).
		SetExecHeight(2).
		SetMerkleRoot([]byte{1}).
		SetTransactions([][]byte{{1}}).
		Sign(priv)

	bcm := core.NewBlockCommit().
		SetHash(blk.Hash()).
		SetMerkleRoot([]byte{1})

	tx := core.NewTransaction().SetNonce(1).Sign(priv)
	txc := core.NewTxCommit().
		SetHash(tx.Hash()).
		SetBlockHash(blk.Hash())

	var err error
	_, err = cs.getBlock(blk.Hash())
	asrt.Error(err)
	_, err = cs.getBlockByHeight(blk.Height())
	asrt.Error(err)
	_, err = cs.getLastBlock()
	asrt.Error(err)
	_, err = cs.getBlockCommit(bcm.Hash())
	asrt.Error(err)
	_, err = cs.getTx(tx.Hash())
	asrt.Error(err)
	asrt.False(cs.hasTx(tx.Hash()))
	_, err = cs.getTxCommit(tx.Hash())
	asrt.Error(err)

	updfns := make([]updateFunc, 0)
	updfns = append(updfns, cs.setBlock(blk)...)
	updfns = append(updfns, cs.setBlockHeight(blk.Height()))
	updfns = append(updfns, cs.setBlockCommit(bcm))
	updfns = append(updfns, cs.setTx(tx))
	updfns = append(updfns, cs.setTxCommit(txc))

	updateLevelDB(db, updfns)

	blk1, err := cs.getBlock(blk.Hash())
	asrt.NoError(err)
	asrt.Equal(blk.Height(), blk1.Height())

	blk2, err := cs.getBlockByHeight(blk.Height())
	asrt.NoError(err)
	asrt.Equal(blk.Hash(), blk2.Hash())

	blk3, err := cs.getLastBlock()
	asrt.NoError(err)
	asrt.Equal(blk.Hash(), blk3.Hash())

	bcm1, err := cs.getBlockCommit(bcm.Hash())
	asrt.NoError(err)
	asrt.Equal(bcm.MerkleRoot(), bcm1.MerkleRoot())

	tx1, err := cs.getTx(tx.Hash())
	asrt.NoError(err)
	asrt.Equal(tx.Nonce(), tx1.Nonce())

	asrt.True(cs.hasTx(tx.Hash()))

	txc1, err := cs.getTxCommit(tx.Hash())
	asrt.NoError(err)
	asrt.Equal(txc.BlockHash(), txc1.BlockHash())
}

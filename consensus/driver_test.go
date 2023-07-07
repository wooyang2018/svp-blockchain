// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wooyang2018/posv-blockchain/core"
	"github.com/wooyang2018/posv-blockchain/storage"
)

func setupTestDriver() *driver {
	resources := &Resources{
		Signer: core.GenerateKey(nil),
	}
	d := &driver{
		resources: resources,
		config:    DefaultConfig,
		status:    new(status),
		state:     newState(),
	}
	return d
}

func newTestBlock(priv core.Signer, height, execHeight uint64,
	parentHash []byte, mRoot []byte, txs [][]byte) *core.Block {
	return core.NewBlock().
		SetHeight(height).
		SetExecHeight(execHeight).
		SetParentHash(parentHash).
		SetMerkleRoot(mRoot).
		SetTransactions(txs).
		Sign(priv)
}

func TestCommit(t *testing.T) {
	d := setupTestDriver()
	parent := newTestBlock(d.resources.Signer, 10, 9,
		nil, nil, nil)
	bfork := newTestBlock(d.resources.Signer, 10, 9,
		nil, nil, [][]byte{[]byte("tx from fork")})
	tx := core.NewTransaction().Sign(d.resources.Signer)
	bexec := newTestBlock(d.resources.Signer, 11, 10,
		parent.Hash(), nil, [][]byte{tx.Hash()})
	d.state.setBlock(parent)
	d.state.setCommittedBlock(parent)
	d.state.setBlock(bfork)
	d.state.setBlock(bexec)

	txs := []*core.Transaction{tx}
	txPool := new(MockTxPool)
	if ExecuteTxFlag {
		txPool.On("GetTxsToExecute", bexec.Transactions()).Return(txs, nil)
	}
	if !PreserveTxFlag { // should remove txs from pool after commit
		txPool.On("RemoveTxs", bexec.Transactions()).Once()
	}
	// should put txs of forked block back to queue from pending
	txPool.On("PutTxsToQueue", bfork.Transactions()).Once()
	d.resources.TxPool = txPool

	bcm := core.NewBlockCommit().SetHash(bexec.Hash())
	txcs := []*core.TxCommit{core.NewTxCommit().SetHash(tx.Hash())}
	cdata := &storage.CommitData{
		Block:        bexec,
		Transactions: txs,
		BlockCommit:  bcm,
		TxCommits:    txcs,
	}

	execution := new(MockExecution)
	if ExecuteTxFlag {
		execution.On("Execute", bexec, txs).Return(bcm, txcs)
	} else {
		cdata.Transactions = nil
		execution.On("MockExecute", bexec).Return(bcm, txcs)
	}
	d.resources.Execution = execution

	strg := new(MockStorage)
	strg.On("Commit", cdata).Return(nil)
	d.resources.Storage = strg

	d.commit(bexec)

	txPool.AssertExpectations(t)
	execution.AssertExpectations(t)
	strg.AssertExpectations(t)

	asrt := assert.New(t)
	asrt.NotNil(d.state.getBlock(bexec.Hash()), "should not delete bexec from state")
	asrt.Nil(d.state.getBlock(bfork.Hash()), "should delete forked block from stata")
}

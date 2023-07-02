// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"bytes"
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

func TestDriver_CreateProposal(t *testing.T) {
	d := setupTestDriver()
	parent := newTestBlock(d.resources.Signer, 4, 3, nil, nil, nil)
	d.state.setBlock(parent)
	d.status.setBLeaf(parent)
	qc := core.NewQuorumCert()
	d.status.setQCHigh(qc)

	txsInQ := [][]byte{[]byte("tx1"), []byte("tx2")}
	txPool := new(MockTxPool)
	if PreserveTxFlag {
		txPool.On("GetTxsFromQueue", d.config.BlockTxLimit).Return(txsInQ)
	} else {
		txPool.On("PopTxsFromQueue", d.config.BlockTxLimit).Return(txsInQ)
	}
	d.resources.TxPool = txPool

	strg := new(MockStorage)
	strg.On("GetBlockHeight").Return(4) // driver should get bexec height from storage
	strg.On("GetMerkleRoot").Return([]byte("merkle-root"))
	d.resources.Storage = strg

	pro := d.CreateProposal()

	txPool.AssertExpectations(t)
	strg.AssertExpectations(t)

	asrt := assert.New(t)
	asrt.NotNil(pro)
	asrt.True(bytes.Equal(parent.Hash(), pro.Block().ParentHash()), "should link to parent")
	asrt.Equal(qc, pro.QuorumCert(), "should add qc")
	asrt.EqualValues(5, pro.Block().Height())

	blk := pro.Block()
	asrt.Equal(txsInQ, blk.Transactions())
	asrt.EqualValues(4, blk.ExecHeight())
	asrt.Equal([]byte("merkle-root"), blk.MerkleRoot())
	asrt.NotEmpty(blk.Timestamp(), "should add timestamp")
	asrt.NotNil(d.state.getBlock(blk.Hash()), "should store leaf block in state")
}

func TestDriver_VoteBlock(t *testing.T) {
	d := setupTestDriver()
	blk1 := newTestBlock(d.resources.Signer, 3, 2, nil, nil, nil)
	pro1 := core.NewProposal().SetBlock(blk1).Sign(d.resources.Signer)
	d.state.setBlock(blk1)
	votes := []*core.Vote{
		pro1.Vote(d.resources.Signer),
		pro1.Vote(core.GenerateKey(nil)),
	}
	qc1 := core.NewQuorumCert().Build(d.resources.Signer, votes)
	d.status.setQCHigh(qc1)

	proposer := core.GenerateKey(nil)
	blk2 := newTestBlock(d.resources.Signer, 4, 3, nil, nil, nil)
	pro2 := core.NewProposal().SetBlock(blk2).SetQuorumCert(qc1).Sign(proposer)
	validators := []string{pro2.Proposer().String()}
	d.resources.RoleStore = core.NewRoleStore(validators)

	txPool := new(MockTxPool)
	if !PreserveTxFlag {
		txPool.On("SetTxsPending", pro2.Block().Transactions())
	}
	d.resources.TxPool = txPool

	msgSvc := new(MockMsgService)
	msgSvc.On("SendVote", proposer.PublicKey(), pro2.Vote(d.resources.Signer)).Return(nil)
	d.resources.MsgSvc = msgSvc

	// should sign block and send vote
	d.VoteProposal(pro2, pro2.Block())

	txPool.AssertExpectations(t)
	msgSvc.AssertExpectations(t)
}

func TestDriver_Commit(t *testing.T) {
	d := setupTestDriver()
	parent := newTestBlock(d.resources.Signer, 10, 9, nil, nil, nil)
	bfork := newTestBlock(d.resources.Signer, 10, 9, nil, nil, [][]byte{[]byte("tx from fork")})
	tx := core.NewTransaction().Sign(d.resources.Signer)
	bexec := newTestBlock(d.resources.Signer, 11, 10, parent.Hash(), nil, [][]byte{tx.Hash()})
	d.state.setBlock(parent)
	d.state.setCommittedBlock(parent)
	d.state.setBlock(bfork)
	d.state.setBlock(bexec)

	txs := []*core.Transaction{tx}
	txPool := new(MockTxPool)
	if ExecuteTxFlag {
		txPool.On("GetTxsToExecute", bexec.Transactions()).Return(txs, nil)
	}
	if !PreserveTxFlag {
		txPool.On("RemoveTxs", bexec.Transactions()).Once() // should remove txs from pool after commit
	}
	txPool.On("PutTxsToQueue", bfork.Transactions()).Once() // should put txs of forked block back to queue from pending
	d.resources.TxPool = txPool

	bcm := core.NewBlockCommit().SetHash(bexec.Hash())
	txcs := []*core.TxCommit{core.NewTxCommit().SetHash(tx.Hash())}
	cdata := &storage.CommitData{
		Block:        bexec,
		QC:           nil,
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
	strg.On("GetQC", bexec.Hash()).Return(nil, nil)
	d.resources.Storage = strg

	d.Commit(bexec)

	txPool.AssertExpectations(t)
	execution.AssertExpectations(t)
	strg.AssertExpectations(t)

	asrt := assert.New(t)
	asrt.NotNil(d.state.getBlock(bexec.Hash()), "should not delete bexec from state")
	asrt.Nil(d.state.getBlock(bfork.Hash()), "should delete forked block from stata")
}

func TestDriver_CreateQC(t *testing.T) {
	d := setupTestDriver()
	blk := newTestBlock(d.resources.Signer, 10, 9, nil, nil, nil)
	pro := core.NewProposal().SetBlock(blk).Sign(d.resources.Signer)
	d.state.setBlock(blk)
	votes := []*core.Vote{
		pro.Vote(d.resources.Signer),
		pro.Vote(core.GenerateKey(nil)),
	}
	qc := core.NewQuorumCert().Build(d.resources.Signer, votes)
	assert.Equal(t, blk, d.state.getBlock(qc.BlockHash()), "should get qc reference block")
}

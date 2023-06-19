// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wooyang2018/posv-blockchain/logger"

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
		state:     newState(),
	}
	return d
}

func TestDriver_CreateProposal(t *testing.T) {
	d := setupTestDriver()
	parent := core.NewBlock().SetHeight(4).Sign(d.resources.Signer)
	d.state.setBlock(parent)
	d.setBLeaf(parent)
	qc := core.NewQuorumCert()
	d.setQCHigh(qc)

	txsInQ := [][]byte{[]byte("tx1"), []byte("tx2")}
	txPool := new(MockTxPool)
	if PreserveTxFlag {
		txPool.On("GetTxsFromQueue", d.config.BlockTxLimit).Return(txsInQ)
	} else {
		txPool.On("PopTxsFromQueue", d.config.BlockTxLimit).Return(txsInQ)
	}
	d.resources.TxPool = txPool

	strg := new(MockStorage)
	strg.On("GetBlockHeight").Return(2) // driver should get bexec height from storage
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
	asrt.EqualValues(2, blk.ExecHeight())
	asrt.Equal([]byte("merkle-root"), blk.MerkleRoot())
	asrt.NotEmpty(blk.Timestamp(), "should add timestamp")
	asrt.NotNil(d.state.getBlock(blk.Hash()), "should store leaf block in innerState")
}

func TestDriver_VoteBlock(t *testing.T) {
	d := setupTestDriver()
	blk2 := newTestBlock(3, 2, nil, nil, d.resources.Signer)
	pro2 := core.NewProposal().SetBlock(blk2).Sign(d.resources.Signer)
	d.state.setBlock(pro2.Block())
	votes := []*core.Vote{
		pro2.Vote(d.resources.Signer),
		pro2.Vote(core.GenerateKey(nil)),
	}
	qc2 := core.NewQuorumCert().Build(d.resources.Signer, votes)
	d.setQCHigh(qc2)

	proposer := core.GenerateKey(nil)
	blk3 := newTestBlock(4, 3, nil, nil, d.resources.Signer)
	pro := core.NewProposal().
		SetBlock(blk3).
		SetQuorumCert(qc2).
		Sign(proposer)
	validators := []string{pro.Proposer().String()}
	d.resources.RoleStore = core.NewRoleStore(validators)

	txPool := new(MockTxPool)
	if !PreserveTxFlag {
		txPool.On("SetTxsPending", pro.Block().Transactions())
	}
	d.resources.TxPool = txPool

	// should sign block and send vote
	msgSvc := new(MockMsgService)
	msgSvc.On("SendVote", proposer.PublicKey(), pro.Vote(d.resources.Signer)).Return(nil)
	d.resources.MsgSvc = msgSvc

	d.VoteProposal(pro, pro.Block())

	txPool.AssertExpectations(t)
	msgSvc.AssertExpectations(t)

	txPool = new(MockTxPool)
	if !PreserveTxFlag {
		txPool.On("SetTxsPending", pro.Block().Transactions())
	}
	d.resources.TxPool = txPool

	d.VoteProposal(pro, pro.Block())

	txPool.AssertExpectations(t)
	msgSvc.AssertExpectations(t)
}

func TestDriver_Commit(t *testing.T) {
	d := setupTestDriver()
	parent := core.NewBlock().SetHeight(10).Sign(d.resources.Signer)
	bfolk := core.NewBlock().SetTransactions([][]byte{[]byte("txfromfolk")}).
		SetHeight(10).Sign(d.resources.Signer)

	tx := core.NewTransaction().Sign(d.resources.Signer)
	bexec := core.NewBlock().SetTransactions([][]byte{tx.Hash()}).SetParentHash(parent.Hash()).
		SetHeight(11).Sign(d.resources.Signer)
	d.state.setBlock(parent)
	d.state.setCommittedBlock(parent)
	d.state.setBlock(bfolk)
	d.state.setBlock(bexec)

	txs := []*core.Transaction{tx}
	txPool := new(MockTxPool)
	if ExecuteTxFlag {
		txPool.On("GetTxsToExecute", bexec.Transactions()).Return(txs, nil)
	}
	if !PreserveTxFlag {
		txPool.On("RemoveTxs", bexec.Transactions()).Once() // should remove txs from pool after commit
	}
	txPool.On("PutTxsToQueue", bfolk.Transactions()).Once() // should put txs of folked block back to queue from pending
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

	storage := new(MockStorage)
	storage.On("Commit", cdata).Return(nil)
	storage.On("GetQC", bexec.Hash()).Return(nil, nil)
	d.resources.Storage = storage

	d.Commit(bexec)

	txPool.AssertExpectations(t)
	execution.AssertExpectations(t)
	storage.AssertExpectations(t)

	asrt := assert.New(t)
	asrt.NotNil(d.state.getBlock(bexec.Hash()),
		"should not delete bexec from innerState")
	asrt.Nil(d.state.getBlock(bfolk.Hash()),
		"should delete folked block from innerState")
}

func TestDriver_CreateQC(t *testing.T) {
	d := setupTestDriver()
	blk := core.NewBlock().SetHeight(10).Sign(d.resources.Signer)
	pro := core.NewProposal().SetBlock(blk).Sign(d.resources.Signer)
	d.state.setBlock(pro.Block())
	votes := []*core.Vote{
		pro.Vote(d.resources.Signer),
		pro.Vote(core.GenerateKey(nil)),
	}
	qc := core.NewQuorumCert().Build(d.resources.Signer, votes)
	assert.Equal(t, pro.Block(), d.state.getBlock(qc.BlockHash()), "should get qc reference block")
}

func TestDriver_BroadcastProposal(t *testing.T) {
	d := setupTestDriver()
	blk := core.NewBlock().SetHeight(10).Sign(d.resources.Signer)
	pro := core.NewProposal().SetBlock(blk).Sign(d.resources.Signer)
	d.state.setBlock(pro.Block())

	msgSvc := new(MockMsgService)
	msgSvc.On("BroadcastProposal", pro).Return(nil)
	d.resources.MsgSvc = msgSvc

	err := d.resources.MsgSvc.BroadcastProposal(pro)
	if err != nil {
		logger.I().Errorw("broadcast proposal failed", "error", err)
	}

	msgSvc.AssertExpectations(t)
}

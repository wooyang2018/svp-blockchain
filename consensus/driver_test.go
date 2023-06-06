// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/wooyang2018/posv-blockchain/core"
	"github.com/wooyang2018/posv-blockchain/storage"
	"github.com/wooyang2018/posv-blockchain/txpool"
)

func setupTestDriver() *driver {
	resources := &Resources{
		Signer: core.GenerateKey(nil),
	}
	state := newState(resources)
	return &driver{
		resources: resources,
		config:    DefaultConfig,
		state:     state,
	}
}

func TestDriver_TestMajorityCount(t *testing.T) {
	d := setupTestDriver()
	validators := []string{
		core.GenerateKey(nil).PublicKey().String(),
		core.GenerateKey(nil).PublicKey().String(),
		core.GenerateKey(nil).PublicKey().String(),
		core.GenerateKey(nil).PublicKey().String(),
	}
	d.resources.VldStore = core.NewValidatorStore(validators, []int{1, 1, 1, 1}, validators)

	res := d.MajorityValidatorCount()

	assert := assert.New(t)
	assert.Equal(d.resources.VldStore.MajorityValidatorCount(), res)
}

func TestDriver_CreateLeaf(t *testing.T) {
	d := setupTestDriver()
	parent := newBlock(core.NewBlock().Sign(d.resources.Signer), d.state)
	d.state.setBlock(parent.block)
	qc := newQC(core.NewQuorumCert(), d.state)
	height := uint64(5)

	txsInQ := [][]byte{[]byte("tx1"), []byte("tx2")}
	txPool := new(MockTxPool)
	if PreserveTxFlag {
		txPool.On("GetTxsFromQueue", d.config.BlockTxLimit).Return(txsInQ)
	} else {
		txPool.On("PopTxsFromQueue", d.config.BlockTxLimit).Return(txsInQ)
	}
	d.resources.TxPool = txPool

	storage := new(MockStorage)
	storage.On("GetBlockHeight").Return(2) // driver should get bexec height from storage
	storage.On("GetMerkleRoot").Return([]byte("merkle-root"))
	d.resources.Storage = storage

	pro := d.CreateProposal(parent, qc, height, 0)

	txPool.AssertExpectations(t)
	storage.AssertExpectations(t)

	assert := assert.New(t)
	assert.NotNil(pro)
	assert.True(parent.Equal(pro.Block().Parent()), "should link to parent")
	assert.Equal(qc, pro.Justify(), "should add qc")
	assert.Equal(height, pro.Block().Height())

	blk := pro.proposal.Block()
	assert.Equal(txsInQ, blk.Transactions())
	assert.EqualValues(2, blk.ExecHeight())
	assert.Equal([]byte("merkle-root"), blk.MerkleRoot())
	assert.NotEmpty(blk.Timestamp(), "should add timestamp")

	assert.NotNil(d.state.getBlock(blk.Hash()), "should store leaf block in posvState")
}

func TestDriver_VoteBlock(t *testing.T) {
	d := setupTestDriver()
	d.checkTxDelay = time.Millisecond
	d.config.TxWaitTime = 20 * time.Millisecond

	proposer := core.GenerateKey(nil)
	blk := core.NewProposal().Sign(proposer)

	validators := []string{blk.Proposer().String()}
	d.resources.VldStore = core.NewValidatorStore(validators, []int{1}, validators)

	txPool := new(MockTxPool)
	txPool.On("GetStatus").Return(txpool.Status{}) // no txs in the pool
	if !PreserveTxFlag {
		txPool.On("SetTxsPending", blk.Block().Transactions())
	}
	d.resources.TxPool = txPool

	// should sign block and send vote
	msgSvc := new(MockMsgService)
	msgSvc.On("SendVote", proposer.PublicKey(), blk.Vote(d.resources.Signer)).Return(nil)
	d.resources.MsgSvc = msgSvc

	start := time.Now()
	d.VoteProposal(newProposal(blk, d.state))
	elapsed := time.Since(start)

	txPool.AssertExpectations(t)
	msgSvc.AssertExpectations(t)

	assert := assert.New(t)
	assert.GreaterOrEqual(elapsed, d.config.TxWaitTime, "should delay if no txs in the pool")

	txPool = new(MockTxPool)
	txPool.On("GetStatus").Return(txpool.Status{Total: 1}) // one txs in the pool
	if !PreserveTxFlag {
		txPool.On("SetTxsPending", blk.Block().Transactions())
	}
	d.resources.TxPool = txPool

	start = time.Now()
	d.VoteProposal(newProposal(blk, d.state))
	elapsed = time.Since(start)

	txPool.AssertExpectations(t)
	msgSvc.AssertExpectations(t)

	assert.Less(elapsed, d.config.TxWaitTime, "should not delay if txs in the pool")
}

func TestDriver_Commit(t *testing.T) {
	d := setupTestDriver()
	parent := core.NewBlock().SetHeight(10).Sign(d.resources.Signer)
	bfolk := core.NewBlock().SetTransactions([][]byte{[]byte("txfromfolk")}).SetHeight(10).Sign(d.resources.Signer)

	tx := core.NewTransaction().Sign(d.resources.Signer)
	bexec := core.NewBlock().SetTransactions([][]byte{tx.Hash()}).
		SetParentHash(parent.Hash()).SetHeight(11).Sign(d.resources.Signer)
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

	d.Commit(newBlock(bexec, d.state))

	txPool.AssertExpectations(t)
	execution.AssertExpectations(t)
	storage.AssertExpectations(t)

	assert := assert.New(t)
	assert.NotNil(d.state.getBlockFromState(bexec.Hash()),
		"should not delete bexec from posvState")
	assert.Nil(d.state.getBlockFromState(bfolk.Hash()),
		"should delete folked block from posvState")
}

func TestDriver_CreateQC(t *testing.T) {
	d := setupTestDriver()
	blk := core.NewProposal().Sign(d.resources.Signer)
	d.state.setBlock(blk.Block())
	votes := []*innerVote{
		newVote(blk.Vote(d.resources.Signer), d.state),
		newVote(blk.Vote(core.GenerateKey(nil)), d.state),
	}
	qc := d.CreateQC(votes)

	assert := assert.New(t)
	assert.Equal(blk.Block(), qc.Block().block, "should get qc reference block")
}

func TestDriver_BroadcastProposal(t *testing.T) {
	d := setupTestDriver()
	blk := core.NewProposal().Sign(d.resources.Signer)
	d.state.setBlock(blk.Block())

	msgSvc := new(MockMsgService)
	msgSvc.On("BroadcastProposal", blk).Return(nil)
	d.resources.MsgSvc = msgSvc

	d.BroadcastProposal(newProposal(blk, d.state))

	msgSvc.AssertExpectations(t)
}

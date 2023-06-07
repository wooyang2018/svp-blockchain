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
	return &driver{
		resources: resources,
		config:    DefaultConfig,
		state:     newState(resources),
	}
}

func TestDriver_CreateProposal(t *testing.T) {
	d := setupTestDriver()
	parent := newBlock(core.NewBlock().Sign(d.resources.Signer), d.state)
	d.state.setBlock(parent.block)
	qc := newQC(core.NewQuorumCert(), d.state)

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

	pro := d.CreateProposal(parent, qc, 5, 1)

	txPool.AssertExpectations(t)
	storage.AssertExpectations(t)

	asrt := assert.New(t)
	asrt.NotNil(pro)
	asrt.True(parent.Equal(pro.Block().Parent()), "should link to parent")
	asrt.Equal(qc, pro.Justify(), "should add qc")
	asrt.EqualValues(5, pro.Block().Height())

	blk := pro.proposal.Block()
	asrt.Equal(txsInQ, blk.Transactions())
	asrt.EqualValues(2, blk.ExecHeight())
	asrt.Equal([]byte("merkle-root"), blk.MerkleRoot())
	asrt.NotEmpty(blk.Timestamp(), "should add timestamp")
	asrt.NotNil(d.state.getBlock(blk.Hash()), "should store leaf block in posvState")
}

func TestDriver_VoteBlock(t *testing.T) {
	d := setupTestDriver()
	d.checkTxDelay = time.Millisecond
	d.config.TxWaitTime = 20 * time.Millisecond

	proposer := core.GenerateKey(nil)
	pro := core.NewProposal().Sign(proposer)
	validators := []string{pro.Proposer().String()}
	d.resources.VldStore = core.NewValidatorStore(validators, []int{1}, validators)

	txPool := new(MockTxPool)
	txPool.On("GetStatus").Return(txpool.Status{}) // no txs in the pool
	if !PreserveTxFlag {
		txPool.On("SetTxsPending", pro.Block().Transactions())
	}
	d.resources.TxPool = txPool

	// should sign block and send vote
	msgSvc := new(MockMsgService)
	msgSvc.On("SendVote", proposer.PublicKey(), pro.Vote(d.resources.Signer)).Return(nil)
	d.resources.MsgSvc = msgSvc

	start := time.Now()
	d.VoteProposal(newProposal(pro, d.state))
	elapsed := time.Since(start)

	txPool.AssertExpectations(t)
	msgSvc.AssertExpectations(t)

	asrt := assert.New(t)
	asrt.GreaterOrEqual(elapsed, d.config.TxWaitTime, "should delay if no txs in the pool")

	txPool = new(MockTxPool)
	txPool.On("GetStatus").Return(txpool.Status{Total: 1}) // one txs in the pool
	if !PreserveTxFlag {
		txPool.On("SetTxsPending", pro.Block().Transactions())
	}
	d.resources.TxPool = txPool

	start = time.Now()
	d.VoteProposal(newProposal(pro, d.state))
	elapsed = time.Since(start)

	txPool.AssertExpectations(t)
	msgSvc.AssertExpectations(t)

	asrt.Less(elapsed, d.config.TxWaitTime, "should not delay if txs in the pool")
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
	asrt.NotNil(d.state.getBlockFromState(bexec.Hash()),
		"should not delete bexec from posvState")
	asrt.Nil(d.state.getBlockFromState(bfolk.Hash()),
		"should delete folked block from posvState")
}

func TestDriver_CreateQC(t *testing.T) {
	d := setupTestDriver()
	pro := core.NewProposal().Sign(d.resources.Signer)
	d.state.setBlock(pro.Block())
	votes := []*innerVote{
		newVote(pro.Vote(d.resources.Signer), d.state),
		newVote(pro.Vote(core.GenerateKey(nil)), d.state),
	}
	qc := d.CreateQC(votes)

	assert.Equal(t, pro.Block(), qc.Block().block, "should get qc reference block")
}

func TestDriver_BroadcastProposal(t *testing.T) {
	d := setupTestDriver()
	pro := core.NewProposal().Sign(d.resources.Signer)
	d.state.setBlock(pro.Block())

	msgSvc := new(MockMsgService)
	msgSvc.On("BroadcastProposal", pro).Return(nil)
	d.resources.MsgSvc = msgSvc

	d.BroadcastProposal(newProposal(pro, d.state))

	msgSvc.AssertExpectations(t)
}

// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"bytes"
	"sync"
	"time"

	"github.com/wooyang2018/posv-blockchain/core"
	"github.com/wooyang2018/posv-blockchain/emitter"
	"github.com/wooyang2018/posv-blockchain/logger"
	"github.com/wooyang2018/posv-blockchain/storage"
)

type driver struct {
	resources *Resources
	config    Config
	state     *state
	status    *status

	tester     *tester
	checkDelay time.Duration // latency to check tx num in pool
	qcEmitter  *emitter.Emitter

	leaderTimer        *time.Timer
	viewTimer          *time.Timer
	leaderTimeoutCount int
	stopCh             chan struct{}

	mtxUpdate sync.Mutex // lock for update call
}

func (d *driver) isLeader(pubKey *core.PublicKey) bool {
	if !d.resources.RoleStore.IsValidator(pubKey) {
		return false
	}
	return d.status.getLeaderIndex() == uint32(d.resources.RoleStore.GetValidatorIndex(pubKey))
}

func (d *driver) cleanStateOnCommitted(blk *core.Block) {
	// qc for bexec is no longer needed here after committed to storage
	d.state.deleteQC(blk.Hash())
	if !PreserveTxFlag {
		d.resources.TxPool.RemoveTxs(blk.Transactions())
	}
	d.state.setCommittedBlock(blk)
	blocks := d.state.getUncommittedOlderBlocks(blk)
	for _, blk := range blocks {
		// put transactions from forked block back to queue
		d.resources.TxPool.PutTxsToQueue(blk.Transactions())
		d.state.deleteBlock(blk.Hash())
		d.state.deleteQC(blk.Hash())
	}
	//delete committed older blocks
	height := blk.Height()
	if height < 20 {
		return
	}
	blks := d.state.getOlderBlocks(height)
	for _, blk := range blks {
		d.state.deleteBlock(blk.Hash())
		d.state.deleteCommitted(blk.Hash())
	}
}

func (d *driver) getBlockByHash(hash []byte) *core.Block {
	blk := d.state.getBlock(hash)
	if blk != nil {
		return blk
	}
	blk, _ = d.resources.Storage.GetBlock(hash)
	if blk == nil {
		return nil
	}
	d.state.setBlock(blk)
	return blk
}

func (d *driver) getQCByBlockHash(blkHash []byte) *core.QuorumCert {
	qc := d.state.getQC(blkHash)
	if qc != nil {
		return qc
	}
	qc, _ = d.resources.Storage.GetQC(blkHash)
	if qc == nil {
		return nil
	}
	d.state.setQC(qc)
	return qc
}

func (d *driver) qcRefHeight(qc *core.QuorumCert) uint64 {
	return d.getBlockByHash(qc.BlockHash()).Height()
}

// cmpBlockHeight compares two blocks by height
func (d *driver) cmpBlockHeight(b1, b2 *core.Block) int {
	if b1 == nil || b2 == nil {
		panic("failed to compare nil block height")
	}
	if b1.Height() == b2.Height() {
		return 0
	} else if b1.Height() > b2.Height() {
		return 1
	}
	return -1
}

// cmpQCPriority compares two qc by view and height
func (d *driver) cmpQCPriority(qc1, qc2 *core.QuorumCert) int {
	if qc1 == nil || qc2 == nil {
		panic("failed to compare nil qc priority")
	}
	if qc1.View() > qc2.View() {
		return 1
	} else if qc1.View() < qc2.View() {
		return -1
	} else { //qc1.View() == qc2.View()
		if d.qcRefHeight(qc1) > d.qcRefHeight(qc2) {
			return 1
		} else if d.qcRefHeight(qc1) < d.qcRefHeight(qc2) {
			return -1
		}
	}
	return 0
}

func (d *driver) delayProposeWhenNoTxs() {
	timer := time.NewTimer(d.config.TxWaitTime)
	defer timer.Stop()
	for d.resources.TxPool.GetStatus().Total == 0 {
		select {
		case <-timer.C:
			return
		case <-time.After(d.checkDelay):
		}
	}
}

func (d *driver) VoteProposal(pro *core.Proposal, blk *core.Block) {
	if d.cmpQCPriority(pro.QuorumCert(), d.status.getQCHigh()) < 0 {
		logger.I().Warnw("can not vote for proposal,", "height", blk.Height())
		return
	}
	vote := pro.Vote(d.resources.Signer)
	if !PreserveTxFlag {
		d.resources.TxPool.SetTxsPending(blk.Transactions())
	}
	proposer := d.resources.RoleStore.GetValidatorIndex(pro.Proposer())
	if uint32(proposer) != d.status.getLeaderIndex() {
		logger.I().Warnf("can not vote proposal height %d", blk.Height())
		return // view changed happened
	}
	d.resources.MsgSvc.SendVote(pro.Proposer(), vote)
	logger.I().Debugw("voted proposal",
		"proposer", proposer,
		"height", blk.Height(),
		"qc", d.qcRefHeight(pro.QuorumCert()),
	)
}

// OnPropose is called to propose a new proposal
func (d *driver) OnPropose() *core.Proposal {
	pro := d.CreateProposal()
	d.status.setBLeaf(pro.Block())
	d.status.startProposal(pro, pro.Block())
	d.onNewProposal(pro)
	err := d.resources.MsgSvc.BroadcastProposal(pro)
	if err != nil {
		logger.I().Errorw("broadcast proposal failed", "error", err)
	}
	return pro
}

func (d *driver) CreateProposal() *core.Proposal {
	var txs [][]byte
	if PreserveTxFlag {
		txs = d.resources.TxPool.GetTxsFromQueue(d.config.BlockTxLimit)
	} else {
		txs = d.resources.TxPool.PopTxsFromQueue(d.config.BlockTxLimit)
	}
	parent := d.status.getBLeaf()
	blk := core.NewBlock().
		SetParentHash(parent.Hash()).
		SetHeight(parent.Height() + 1).
		SetTransactions(txs).
		SetExecHeight(d.resources.Storage.GetBlockHeight()).
		SetMerkleRoot(d.resources.Storage.GetMerkleRoot()).
		SetTimestamp(time.Now().UnixNano()).
		Sign(d.resources.Signer)
	d.state.setBlock(blk)
	pro := core.NewProposal().
		SetBlock(blk).
		SetQuorumCert(d.status.getQCHigh()).
		SetView(d.status.getView()).
		Sign(d.resources.Signer)
	return pro
}

func (d *driver) OnNewViewPropose() *core.Proposal {
	qcHigh := d.status.getQCHigh()
	pro := core.NewProposal().
		SetQuorumCert(qcHigh).
		SetView(d.status.getView()).
		Sign(d.resources.Signer)
	blk := d.getBlockByHash(qcHigh.BlockHash())
	d.status.setBLeaf(blk)
	d.status.startProposal(pro, blk)
	d.onNewProposal(pro)
	err := d.resources.MsgSvc.BroadcastProposal(pro)
	if err != nil {
		logger.I().Errorw("broadcast proposal failed", "error", err)
	}
	return pro
}

// OnReceiveVote is called when received a vote
func (d *driver) OnReceiveVote(vote *core.Vote) error {
	err := d.status.addVote(vote)
	if err != nil {
		return err
	}
	blk := d.getBlockByHash(vote.BlockHash())
	logger.I().Debugw("received vote", "height", blk.Height())
	if d.status.getVoteCount() >= d.resources.RoleStore.MajorityValidatorCount() {
		votes := d.status.getVotes()
		d.status.endProposal()
		qc := core.NewQuorumCert().Build(d.resources.Signer, votes)
		d.state.setQC(qc)
		d.UpdateQCHigh(qc)
		d.qcEmitter.Emit(struct{}{})
	}
	return nil
}

// UpdateQCHigh replaces high qc if the given qc is higher than it
func (d *driver) UpdateQCHigh(qc *core.QuorumCert) {
	if d.cmpQCPriority(qc, d.status.getQCHigh()) == 1 {
		blk := d.getBlockByHash(qc.BlockHash())
		logger.I().Debugw("updated high qc", "view", qc.View(), "qc", d.qcRefHeight(qc))
		d.status.setQCHigh(qc)
		d.status.setBLeaf(blk)
		d.CommitRecursive(blk)
	}
}

func (d *driver) CommitRecursive(blk *core.Block) { // prepare phase for b2
	t1 := time.Now().UnixNano()
	d.onCommit(blk)
	d.status.setBExec(blk)
	t2 := time.Now().UnixNano()
	d.tester.saveItem(blk.Height(), blk.Timestamp(), t1, t2, len(blk.Transactions()))
}

func (d *driver) onCommit(blk *core.Block) {
	if d.cmpBlockHeight(blk, d.status.getBExec()) == 1 {
		// commit parent blocks recursively
		d.onCommit(d.getBlockByHash(blk.ParentHash()))
		d.Commit(blk)
	} else if !bytes.Equal(d.status.getBExec().Hash(), blk.Hash()) {
		logger.I().Fatalw("safety breached", "hash", base64String(blk.Hash()), "height", blk.Height())
	}
}

func (d *driver) Commit(blk *core.Block) {
	start := time.Now()
	rawTxs := blk.Transactions()
	qc := d.getQCByBlockHash(blk.Hash())
	var txCount int
	var data *storage.CommitData
	if ExecuteTxFlag {
		txs, old := d.resources.TxPool.GetTxsToExecute(rawTxs)
		txCount = len(txs)
		logger.I().Debugw("committing block", "height", blk.Height(), "txs", txCount)
		bcm, txcs := d.resources.Execution.Execute(blk, txs)
		bcm.SetOldBlockTxs(old)
		data = &storage.CommitData{
			Block:        blk,
			QC:           qc,
			Transactions: txs,
			BlockCommit:  bcm,
			TxCommits:    txcs,
		}
	} else {
		txCount = len(rawTxs)
		logger.I().Debugw("committing block", "height", blk.Height(), "txs", txCount)
		bcm, txcs := d.resources.Execution.MockExecute(blk)
		bcm.SetOldBlockTxs(rawTxs)
		data = &storage.CommitData{
			Block:        blk,
			QC:           qc,
			Transactions: nil,
			BlockCommit:  bcm,
			TxCommits:    txcs,
		}
	}
	err := d.resources.Storage.Commit(data)
	if err != nil {
		logger.I().Fatalf("commit storage error: %+v", err)
	}
	d.state.addCommittedTxCount(txCount)
	d.cleanStateOnCommitted(blk)
	logger.I().Debugw("committed bock",
		"height", blk.Height(),
		"txs", txCount,
		"elapsed", time.Since(start))
}

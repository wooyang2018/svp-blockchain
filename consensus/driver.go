// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"bytes"
	"sync"
	"time"

	"github.com/wooyang2018/posv-blockchain/core"
	"github.com/wooyang2018/posv-blockchain/logger"
	"github.com/wooyang2018/posv-blockchain/storage"
)

type driver struct {
	resources *Resources
	config    Config
	state     *state
	status    *status
	tester    *tester

	leaderTimer        *time.Timer
	viewTimer          *time.Timer
	leaderTimeoutCount int
	isCommitting       bool

	proposeCh chan struct{}
	stopCh    chan struct{}

	mtxUpdate sync.Mutex // lock for update call
}

func (d *driver) start() {
	if d.stopCh != nil {
		return
	}
	d.stopCh = make(chan struct{})
	d.proposeCh = make(chan struct{})
	d.status.setViewStart()
	go d.newViewLoop()
	logger.I().Info("started rotator")
}

func (d *driver) stop() {
	if d.stopCh == nil {
		return // not started yet
	}
	select {
	case <-d.stopCh: // already stopped
		return
	default:
	}
	for d.isCommitting {
		time.Sleep(200 * time.Millisecond)
	}
	close(d.stopCh)
	logger.I().Info("stopped rotator")
	d.stopCh = nil
}

func (d *driver) isLeader(pubKey *core.PublicKey) bool {
	if !d.resources.RoleStore.IsValidator(pubKey) {
		return false
	}
	return d.status.getLeaderIndex() == uint32(d.resources.RoleStore.GetValidatorIndex(pubKey))
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

// onReceiveVote is called when received a vote
func (d *driver) onReceiveVote(vote *core.Vote) error {
	if err := d.status.addVote(vote); err != nil {
		return err
	}
	blk := d.getBlockByHash(vote.BlockHash())
	logger.I().Debugw("received vote",
		"view", vote.View(),
		"height", blk.Height(),
		"quota", vote.Quota())
	if d.status.getVoteCount() >= d.resources.RoleStore.MajorityValidatorCount() &&
		d.status.getQuotaCount() > d.resources.RoleStore.MajorityQuotaCount() {
		qc := core.NewQuorumCert().Build(d.resources.Signer, d.status.getVotes())
		d.status.endProposal()
		d.state.setQC(qc)
		d.updateQCHigh(qc)
		d.proposeCh <- struct{}{} // trigger propose rule
	}
	return nil
}

// updateQCHigh replaces high qc if the given qc is higher than it
func (d *driver) updateQCHigh(qc *core.QuorumCert) {
	blk := d.getBlockByHash(qc.BlockHash())
	if d.cmpQCPriority(qc, d.status.getQCHigh()) == 1 &&
		d.cmpBlockHeight(blk, d.status.getBExec()) == 1 {
		d.resources.Storage.StoreBlock(blk)
		d.resources.Storage.StoreQC(qc)
		d.status.setQCHigh(qc)
		d.status.setBLeaf(blk)
		d.status.updateWindow(qc.Quota(), blk.Height())

		quotas, _ := d.status.getWindow()
		acc := 0.0
		for i := len(quotas) - 1; i >= 0 && blk != nil; i-- {
			acc += quotas[i]
			if acc > d.resources.RoleStore.MajorityQuotaCount() {
				d.commitRecursive(blk)
				break
			}
			blk = d.getBlockByHash(blk.ParentHash())
		}

		logger.I().Infow("updated high qc",
			"view", qc.View(),
			"qc", d.qcRefHeight(qc),
			"quota", qc.Quota(),
			"window", quotas)
	}
}

func (d *driver) commitRecursive(blk *core.Block) {
	t1 := time.Now().UnixNano()
	d.onCommit(blk)
	d.status.setBExec(blk)
	t2 := time.Now().UnixNano()
	d.tester.saveItem(blk.Height(), blk.Timestamp(), t1, t2, len(blk.Transactions()))
}

func (d *driver) onCommit(blk *core.Block) {
	if d.cmpBlockHeight(blk, d.status.getBExec()) == 1 {
		d.onCommit(d.getBlockByHash(blk.ParentHash())) // commit parent blocks recursively
		d.commit(blk)
	} else if !bytes.Equal(d.status.getBExec().Hash(), blk.Hash()) {
		logger.I().Fatalw("safety breached",
			"height", blk.Height(),
			"hash", base64String(blk.Hash()))
	}
}

func (d *driver) commit(blk *core.Block) {
	start := time.Now()
	rawTxs := blk.Transactions()
	var txCount int
	var data *storage.CommitData
	if ExecuteTxFlag {
		txs, old := d.resources.TxPool.GetTxsToExecute(rawTxs)
		txCount = len(txs)
		logger.I().Debugw("executing block", "height", blk.Height(), "txs", txCount)
		bcm, txcs := d.resources.Execution.Execute(blk, txs)
		bcm.SetOldBlockTxs(old)
		data = &storage.CommitData{
			Block:        blk,
			Transactions: txs,
			BlockCommit:  bcm,
			TxCommits:    txcs,
		}
	} else {
		txCount = len(rawTxs)
		logger.I().Debugw("executing block", "height", blk.Height(), "txs", txCount)
		bcm, txcs := d.resources.Execution.MockExecute(blk)
		bcm.SetOldBlockTxs(rawTxs)
		data = &storage.CommitData{
			Block:        blk,
			Transactions: nil,
			BlockCommit:  bcm,
			TxCommits:    txcs,
		}
	}
	d.isCommitting = true
	if err := d.resources.Storage.Commit(data); err != nil {
		logger.I().Fatalf("commit storage error, %+v", err)
	}
	d.isCommitting = false
	d.state.addCommittedTxCount(txCount)
	d.cleanStateOnCommitted(blk)
	logger.I().Infow("committed bock",
		"height", blk.Height(),
		"txs", txCount,
		"elapsed", time.Since(start))
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
		// put txs from forked block back to queue
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

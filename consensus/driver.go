// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"time"

	"github.com/wooyang2018/posv-blockchain/core"
	"github.com/wooyang2018/posv-blockchain/logger"
	"github.com/wooyang2018/posv-blockchain/storage"
)

type driver struct {
	resources    *Resources
	config       Config
	state        *state
	checkTxDelay time.Duration //检测TxPool交易数量的延迟
}

// 验证Driver实现了PoSV的Driver
var _ Driver = (*driver)(nil)

func (d *driver) MajorityValidatorCount() int {
	return d.resources.VldStore.MajorityValidatorCount()
}

func (d *driver) CreateLeaf(parent Block, qc QC, height uint64) Block {
	var txs [][]byte
	if PreserveTxFlag {
		txs = d.resources.TxPool.GetTxsFromQueue(d.config.BlockTxLimit)
	} else {
		txs = d.resources.TxPool.PopTxsFromQueue(d.config.BlockTxLimit)
	}
	//core.Block的链式调用
	blk := core.NewBlock().
		SetParentHash(parent.(*innerBlock).block.Hash()).
		SetQuorumCert(qc.(*innerQC).qc).
		SetHeight(height).
		SetTransactions(txs).
		SetExecHeight(d.resources.Storage.GetBlockHeight()).
		SetMerkleRoot(d.resources.Storage.GetMerkleRoot()).
		SetTimestamp(time.Now().UnixNano()).
		Sign(d.resources.Signer)

	d.state.setBlock(blk)
	return newBlock(blk, d.state)
}

func (d *driver) CreateQC(innerVotes []Vote) QC {
	votes := make([]*core.Vote, len(innerVotes))
	for i, v := range innerVotes {
		votes[i] = v.(*innerVote).vote
	}
	qc := core.NewQuorumCert().Build(votes)
	return newQC(qc, d.state)
}

func (d *driver) BroadcastProposal(iBlk Block) {
	blk := iBlk.(*innerBlock).block
	d.resources.MsgSvc.BroadcastProposal(blk)
}

func (d *driver) VoteBlock(iBlk Block) {
	blk := iBlk.(*innerBlock).block
	vote := blk.Vote(d.resources.Signer)
	if !PreserveTxFlag {
		d.resources.TxPool.SetTxsPending(blk.Transactions())
	}
	d.delayVoteWhenNoTxs()
	proposer := d.resources.VldStore.GetWorkerIndex(blk.Proposer())
	if proposer != d.state.getLeaderIndex() {
		return // view changed happened
	}
	d.resources.MsgSvc.SendVote(blk.Proposer(), vote)
	logger.I().Debugw("voted block",
		"proposer", proposer,
		"height", iBlk.Height(),
		"qc", qcRefHeight(iBlk.Justify()),
	)
}

func (d *driver) delayVoteWhenNoTxs() {
	timer := time.NewTimer(d.config.TxWaitTime)
	defer timer.Stop()
	for d.resources.TxPool.GetStatus().Total == 0 {
		select {
		case <-timer.C:
			return
		case <-time.After(d.checkTxDelay):
		}
	}
}

func (d *driver) Commit(iBlk Block) {
	bexe := iBlk.(*innerBlock).block
	start := time.Now()
	rawTxs := bexe.Transactions()
	var txCount int
	var data *storage.CommitData
	if ExecuteTxFlag {
		txs, old := d.resources.TxPool.GetTxsToExecute(rawTxs)
		txCount = len(txs)
		logger.I().Debugw("committing block", "height", bexe.Height(), "txs", txCount)
		bcm, txcs := d.resources.Execution.Execute(bexe, txs)
		bcm.SetOldBlockTxs(old)
		data = &storage.CommitData{
			Block:        bexe,
			QC:           d.state.getQC(bexe.Hash()),
			Transactions: txs,
			BlockCommit:  bcm,
			TxCommits:    txcs,
		}
	} else {
		txCount = len(rawTxs)
		logger.I().Debugw("committing block", "height", bexe.Height(), "txs", txCount)
		bcm, txcs := d.resources.Execution.MockExecute(bexe)
		bcm.SetOldBlockTxs(rawTxs)
		data = &storage.CommitData{
			Block:        bexe,
			QC:           d.state.getQC(bexe.Hash()),
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
	d.cleanStateOnCommitted(bexe)
	logger.I().Debugw("committed bock",
		"height", bexe.Height(),
		"txs", txCount,
		"elapsed", time.Since(start))
}

func (d *driver) cleanStateOnCommitted(bexec *core.Block) {
	// qc for bexec is no longer needed here after committed to storage
	d.state.deleteQC(bexec.Hash())
	if !PreserveTxFlag {
		d.resources.TxPool.RemoveTxs(bexec.Transactions())
	}
	d.state.setCommittedBlock(bexec)

	blks := d.state.getUncommittedOlderBlocks(bexec)
	for _, blk := range blks {
		// put transactions from forked block back to queue
		d.resources.TxPool.PutTxsToQueue(blk.Transactions())
		d.state.deleteBlock(blk.Hash())
		d.state.deleteQC(blk.Hash())
	}
	d.deleteCommittedOlderBlocks(bexec)
}

func (d *driver) deleteCommittedOlderBlocks(bexec *core.Block) {
	height := int64(bexec.Height()) - 20
	if height < 0 {
		return
	}
	blks := d.state.getOlderBlocks(uint64(height))
	for _, blk := range blks {
		d.state.deleteBlock(blk.Hash())
		d.state.deleteCommitted(blk.Hash())
	}
}

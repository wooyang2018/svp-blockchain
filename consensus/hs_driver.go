// Copyright (C) 2021 Aung Maw
// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"time"

	"github.com/wooyang2018/ppov-blockchain/core"
	"github.com/wooyang2018/ppov-blockchain/hotstuff"
	"github.com/wooyang2018/ppov-blockchain/logger"
	"github.com/wooyang2018/ppov-blockchain/storage"
)

type hsDriver struct {
	resources *Resources
	config    Config

	checkTxDelay time.Duration //检测TxPool交易数量的延迟

	state *state
}

// 验证hsDriver实现了hotstuff的Driver
var _ hotstuff.Driver = (*hsDriver)(nil)

func (hsd *hsDriver) MajorityValidatorCount() int {
	return hsd.resources.VldStore.MajorityValidatorCount()
}

func (hsd *hsDriver) CreateLeaf(parent hotstuff.Block, qc hotstuff.QC, height uint64) hotstuff.Block {
	var txs [][]byte
	if ExecuteTxFlag {
		txs = hsd.resources.TxPool.PopTxsFromQueue(hsd.config.BlockTxLimit)
	} else {
		txs = hsd.resources.TxPool.GetTxsFromQueue(hsd.config.BlockTxLimit)
	}
	//core.Block的链式调用
	blk := core.NewBlock().
		SetParentHash(parent.(*hsBlock).block.Hash()).
		SetQuorumCert(qc.(*hsQC).qc).
		SetHeight(height).
		SetTransactions(txs).
		SetExecHeight(hsd.resources.Storage.GetBlockHeight()).
		SetMerkleRoot(hsd.resources.Storage.GetMerkleRoot()).
		SetTimestamp(time.Now().UnixNano()).
		Sign(hsd.resources.Signer)

	hsd.state.setBlock(blk)
	return newHsBlock(blk, hsd.state)
}

func (hsd *hsDriver) CreateQC(hsVotes []hotstuff.Vote) hotstuff.QC {
	votes := make([]*core.Vote, len(hsVotes))
	for i, hsv := range hsVotes {
		votes[i] = hsv.(*hsVote).vote
	}
	qc := core.NewQuorumCert().Build(votes)
	return newHsQC(qc, hsd.state)
}

func (hsd *hsDriver) BroadcastProposal(hsBlk hotstuff.Block) {
	blk := hsBlk.(*hsBlock).block
	hsd.resources.MsgSvc.BroadcastProposal(blk)
}

func (hsd *hsDriver) VoteBlock(hsBlk hotstuff.Block) {
	blk := hsBlk.(*hsBlock).block
	vote := blk.Vote(hsd.resources.Signer)
	hsd.resources.TxPool.SetTxsPending(blk.Transactions())
	hsd.delayVoteWhenNoTxs()
	proposer := hsd.resources.VldStore.GetWorkerIndex(blk.Proposer())
	if proposer != hsd.state.getLeaderIndex() {
		return // view changed happened
	}
	hsd.resources.MsgSvc.SendVote(blk.Proposer(), vote)
	logger.I().Debugw("voted block",
		"proposer", proposer,
		"height", hsBlk.Height(),
		"qc", qcRefHeight(hsBlk.Justify()),
	)
}

func (hsd *hsDriver) delayVoteWhenNoTxs() {
	timer := time.NewTimer(hsd.config.TxWaitTime)
	defer timer.Stop()
	for hsd.resources.TxPool.GetStatus().Total == 0 {
		select {
		case <-timer.C:
			return
		case <-time.After(hsd.checkTxDelay):
		}
	}
}

func (hsd *hsDriver) Commit(hsBlk hotstuff.Block) {
	bexe := hsBlk.(*hsBlock).block
	start := time.Now()
	rawTxs := bexe.Transactions()
	var txCount int
	var data *storage.CommitData
	if ExecuteTxFlag {
		txs, old := hsd.resources.TxPool.GetTxsToExecute(rawTxs)
		txCount = len(txs)
		logger.I().Debugw("committing block", "height", bexe.Height(), "txs", txCount)
		bcm, txcs := hsd.resources.Execution.Execute(bexe, txs)
		bcm.SetOldBlockTxs(old)
		data = &storage.CommitData{
			Block:        bexe,
			QC:           hsd.state.getQC(bexe.Hash()),
			Transactions: txs,
			BlockCommit:  bcm,
			TxCommits:    txcs,
		}
	} else {
		txCount = len(rawTxs)
		logger.I().Debugw("committing block", "height", bexe.Height(), "txs", txCount)
		bcm, txcs := hsd.resources.Execution.MockExecute(bexe)
		bcm.SetOldBlockTxs(rawTxs)
		data = &storage.CommitData{
			Block:        bexe,
			QC:           hsd.state.getQC(bexe.Hash()),
			Transactions: nil,
			BlockCommit:  bcm,
			TxCommits:    txcs,
		}
	}
	err := hsd.resources.Storage.Commit(data)
	if err != nil {
		logger.I().Fatalf("commit storage error: %+v", err)
	}
	hsd.state.addCommittedTxCount(txCount)
	hsd.cleanStateOnCommitted(bexe)
	logger.I().Debugw("committed bock",
		"height", bexe.Height(),
		"txs", txCount,
		"elapsed", time.Since(start))
}

func (hsd *hsDriver) cleanStateOnCommitted(bexec *core.Block) {
	// qc for bexec is no longer needed here after committed to storage
	hsd.state.deleteQC(bexec.Hash())
	if ExecuteTxFlag {
		hsd.resources.TxPool.RemoveTxs(bexec.Transactions())
	}
	hsd.state.setCommittedBlock(bexec)

	blks := hsd.state.getUncommittedOlderBlocks(bexec)
	for _, blk := range blks {
		// put transactions from forked block back to queue
		hsd.resources.TxPool.PutTxsToQueue(blk.Transactions())
		hsd.state.deleteBlock(blk.Hash())
		hsd.state.deleteQC(blk.Hash())
	}
	hsd.deleteCommittedOlderBlocks(bexec)
}

func (hsd *hsDriver) deleteCommittedOlderBlocks(bexec *core.Block) {
	height := int64(bexec.Height()) - 20
	if height < 0 {
		return
	}
	blks := hsd.state.getOlderBlocks(uint64(height))
	for _, blk := range blks {
		hsd.state.deleteBlock(blk.Hash())
		hsd.state.deleteCommitted(blk.Hash())
	}
}

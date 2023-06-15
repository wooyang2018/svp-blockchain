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
	resources *Resources
	config    Config
	state     *state
	posvState *posvState
	tester    *tester
}

func (d *driver) CreateProposal() *iProposal {
	var txs [][]byte
	if PreserveTxFlag {
		txs = d.resources.TxPool.GetTxsFromQueue(d.config.BlockTxLimit)
	} else {
		txs = d.resources.TxPool.PopTxsFromQueue(d.config.BlockTxLimit)
	}
	parent := d.posvState.GetBLeaf()
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
		SetQuorumCert(d.posvState.GetQCHigh().qc).
		SetView(d.posvState.GetView()).
		Sign(d.resources.Signer)
	return newProposal(pro, d.state)
}

func (d *driver) CreateQC(iVotes []*iVote) *iQC {
	votes := make([]*core.Vote, len(iVotes))
	for i, v := range iVotes {
		votes[i] = v.vote
	}
	qc := core.NewQuorumCert().Build(votes)
	return newQC(qc, d.state)
}

func (d *driver) BroadcastProposal(pro *iProposal) {
	err := d.resources.MsgSvc.BroadcastProposal(pro.proposal)
	if err != nil {
		logger.I().Errorw("broadcast proposal failed", "error", err)
	}
}

func (d *driver) VoteProposal(pro *iProposal, blk *iBlock) {
	proposal := pro.proposal
	vote := proposal.Vote(d.resources.Signer)
	if !PreserveTxFlag {
		d.resources.TxPool.SetTxsPending(proposal.Block().Transactions())
	}
	proposer := d.resources.VldStore.GetWorkerIndex(proposal.Proposer())
	if proposer != d.state.getLeaderIndex() {
		return // view changed happened
	}
	d.resources.MsgSvc.SendVote(proposal.Proposer(), vote)
	logger.I().Debugw("voted proposal",
		"proposer", proposer,
		"height", blk.Height(),
		"qc", qcRefHeight(pro.Justify()),
	)
}

func (d *driver) Commit(bexec *iBlock) {
	start := time.Now()
	rawTxs := bexec.Transactions()
	var txCount int
	var data *storage.CommitData
	if ExecuteTxFlag {
		txs, old := d.resources.TxPool.GetTxsToExecute(rawTxs)
		txCount = len(txs)
		logger.I().Debugw("committing block", "height", bexec.Height(), "txs", txCount)
		bcm, txcs := d.resources.Execution.Execute(bexec.block, txs)
		bcm.SetOldBlockTxs(old)
		data = &storage.CommitData{
			Block:        bexec.block,
			QC:           d.state.getQC(bexec.Hash()),
			Transactions: txs,
			BlockCommit:  bcm,
			TxCommits:    txcs,
		}
	} else {
		txCount = len(rawTxs)
		logger.I().Debugw("committing block", "height", bexec.Height(), "txs", txCount)
		bcm, txcs := d.resources.Execution.MockExecute(bexec.block)
		bcm.SetOldBlockTxs(rawTxs)
		data = &storage.CommitData{
			Block:        bexec.block,
			QC:           d.state.getQC(bexec.Hash()),
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
	d.cleanStateOnCommitted(bexec)
	logger.I().Debugw("committed bock",
		"height", bexec.Height(),
		"txs", txCount,
		"elapsed", time.Since(start))
}

func (d *driver) cleanStateOnCommitted(bexec *iBlock) {
	// qc for bexec is no longer needed here after committed to storage
	d.state.deleteQC(bexec.Hash())
	if !PreserveTxFlag {
		d.resources.TxPool.RemoveTxs(bexec.Transactions())
	}
	d.state.setCommittedBlock(bexec.block)
	blocks := d.state.getUncommittedOlderBlocks(bexec.block)
	for _, blk := range blocks {
		// put transactions from forked block back to queue
		d.resources.TxPool.PutTxsToQueue(blk.Transactions())
		d.state.deleteBlock(blk.Hash())
		d.state.deleteQC(blk.Hash())
	}
	//delete committed older blocks
	height := bexec.Height()
	if height < 20 {
		return
	}
	blks := d.state.getOlderBlocks(height)
	for _, blk := range blks {
		d.state.deleteBlock(blk.Hash())
		d.state.deleteCommitted(blk.Hash())
	}
}

// OnPropose is called to propose a new proposal
func (d *driver) OnPropose() *iProposal {
	pro := d.CreateProposal()
	if pro == nil {
		return nil
	}
	d.posvState.setBLeaf(pro.Block())
	d.posvState.startProposal(pro)
	d.BroadcastProposal(pro)
	return pro
}

func (d *driver) NewViewPropose() *iProposal {
	qcHigh := d.posvState.GetQCHigh()
	pro := core.NewProposal().
		SetQuorumCert(qcHigh.qc).
		SetView(d.posvState.GetView()).
		Sign(d.resources.Signer)
	proposal := newProposal(pro, d.state)
	if proposal == nil {
		return nil
	}
	d.posvState.setBLeaf(qcHigh.Block())
	d.posvState.startProposal(proposal)
	d.BroadcastProposal(proposal)
	return proposal
}

// OnReceiveVote is called when received a vote
func (d *driver) OnReceiveVote(vote *iVote) error {
	err := d.posvState.addVote(vote)
	if err != nil {
		return err
	}
	logger.I().Debugw("received vote", "height", vote.Block().Height())
	if d.posvState.GetVoteCount() >= d.resources.VldStore.MajorityValidatorCount() {
		votes := d.posvState.GetVotes()
		d.posvState.endProposal()
		d.posvState.UpdateQCHigh(d.CreateQC(votes))
	}
	return nil
}

func (d *driver) Update(qc *iQC) {
	d.posvState.UpdateQCHigh(qc)
	b := qc.Block()
	if b != nil {
		d.CommitRecursive(b)
	}
}

func (d *driver) CommitRecursive(b *iBlock) { // prepare phase for b2
	t1 := time.Now().UnixNano()
	d.onCommit(b)
	d.posvState.setBExec(b)
	t2 := time.Now().UnixNano()
	d.tester.saveItem(b.Height(), b.Timestamp(), t1, t2, len(b.Transactions()))
}

func (d *driver) onCommit(b *iBlock) {
	if cmpBlockHeight(b, d.posvState.GetBExec()) == 1 {
		// commit parent blocks recursively
		d.onCommit(b.Parent())
		d.Commit(b)
	} else if !d.posvState.GetBExec().Equal(b) {
		logger.I().Warnf("safety breached b-recurrsive: %+v, bexec: %d", b, d.posvState.GetBExec().Height())
	}
}

func (d *driver) Lock() {
	d.state.mtxUpdate.Lock()
}

func (d *driver) Unlock() {
	d.state.mtxUpdate.Unlock()
}

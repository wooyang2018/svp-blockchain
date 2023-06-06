// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"os"
	"time"

	"github.com/wooyang2018/posv-blockchain/core"
	"github.com/wooyang2018/posv-blockchain/logger"
	"github.com/wooyang2018/posv-blockchain/storage"
)

type driver struct {
	resources    *Resources
	config       Config
	state        *state
	posvState    *posvState
	tester       *tester
	checkTxDelay time.Duration //检测TxPool交易数量的延迟
}

func NewDriver(resources *Resources, config Config, state *state) *driver {
	d := &driver{
		resources:    resources,
		config:       config,
		checkTxDelay: 10 * time.Millisecond,
		state:        state,
	}
	var file *os.File
	if d.config.BenchmarkPath != "" {
		var err error
		file, err = os.Create(d.config.BenchmarkPath)
		if err != nil {
			logger.I().Warnf("create benchmark log file failed, %+v", err.Error())
		}
	}
	d.tester = newTester(file)
	return d
}

func (d *driver) MajorityValidatorCount() int {
	return d.resources.VldStore.MajorityValidatorCount()
}

func (d *driver) CreateProposal(parent *innerBlock, qc *innerQC, height uint64, view uint32) *innerProposal {
	var txs [][]byte
	if PreserveTxFlag {
		txs = d.resources.TxPool.GetTxsFromQueue(d.config.BlockTxLimit)
	} else {
		txs = d.resources.TxPool.PopTxsFromQueue(d.config.BlockTxLimit)
	}
	blk := core.NewBlock().
		SetParentHash(parent.Hash()).
		SetHeight(height).
		SetTransactions(txs).
		SetExecHeight(d.resources.Storage.GetBlockHeight()).
		SetMerkleRoot(d.resources.Storage.GetMerkleRoot()).
		SetTimestamp(time.Now().UnixNano()).
		Sign(d.resources.Signer)
	pro := core.NewProposal().
		SetBlock(blk).
		SetQuorumCert(qc.qc).
		Sign(d.resources.Signer)
	d.state.setBlock(blk)
	return newProposal(pro, d.state)
}

func (d *driver) CreateQC(innerVotes []*innerVote) *innerQC {
	votes := make([]*core.Vote, len(innerVotes))
	for i, v := range innerVotes {
		votes[i] = v.vote
	}
	qc := core.NewQuorumCert().Build(votes)
	return newQC(qc, d.state)
}

func (d *driver) BroadcastProposal(pro *innerProposal) {
	err := d.resources.MsgSvc.BroadcastProposal(pro.proposal)
	if err != nil {
		logger.I().Errorf("BroadcastProposal failed: %v", err) //TODO 表述
	}
}

func (d *driver) VoteProposal(pro *innerProposal) {
	proposal := pro.proposal
	vote := proposal.Vote(d.resources.Signer)
	if !PreserveTxFlag {
		d.resources.TxPool.SetTxsPending(proposal.Block().Transactions())
	}
	d.delayVoteWhenNoTxs()
	proposer := d.resources.VldStore.GetWorkerIndex(proposal.Proposer())
	if proposer != d.state.getLeaderIndex() {
		return // view changed happened
	}
	d.resources.MsgSvc.SendVote(proposal.Proposer(), vote)
	logger.I().Debugw("voted block",
		"proposer", proposer,
		"height", pro.Block().Height(),
		"qc", qcRefHeight(pro.Justify()),
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

func (d *driver) Commit(blk *innerBlock) { //TODO Block接口转*core.Block
	bexe := blk.block
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

// OnPropose is called to propose a new block
func (d *driver) OnPropose() *innerProposal {
	bLeaf := d.posvState.GetBLeaf()
	bNew := d.CreateProposal(bLeaf, d.posvState.GetQCHigh(), bLeaf.Height()+1, 1) //TODO: ViewNum
	if bNew == nil {
		return nil
	}
	d.posvState.setBLeaf(bNew.Block())
	d.posvState.startProposal(bNew)
	d.BroadcastProposal(bNew)
	return bNew
}

// OnReceiveVote is called when received a vote
func (d *driver) OnReceiveVote(v *innerVote) error {
	err := d.posvState.addVote(v)
	if err != nil {
		return err
	}
	logger.I().Debugw("posv received vote", "height", v.Block().Height())
	if d.posvState.GetVoteCount() >= d.MajorityValidatorCount() {
		votes := d.posvState.GetVotes()
		d.posvState.endProposal()
		d.posvState.UpdateQCHigh(d.CreateQC(votes))
	}
	return nil
}

// OnReceiveProposal is called when a new proposal is received
func (d *driver) OnReceiveProposal(bNew *innerProposal) error {
	d.Update(bNew)
	if d.posvState.CanVote(bNew) {
		d.VoteProposal(bNew)
		d.posvState.setBVote(bNew.Block())
	}
	return nil
}

// Update perform two/three chain consensus phases
func (d *driver) Update(bNew *innerProposal) {
	d.posvState.UpdateQCHigh(bNew.Justify()) // prepare phase for b2
	b := bNew.Justify().Block()
	if b != nil {
		t1 := time.Now().UnixNano()
		d.onCommit(b)
		d.posvState.setBExec(b)
		t2 := time.Now().UnixNano()
		d.tester.saveItem(b.Height(), b.Timestamp(), t1, t2, len(b.Transactions()))
	}
}

func (d *driver) CommitRecursive(b *innerBlock) { // prepare phase for b2
	t1 := time.Now().UnixNano()
	d.onCommit(b)
	d.posvState.setBExec(b)
	t2 := time.Now().UnixNano()
	d.tester.saveItem(b.Height(), b.Timestamp(), t1, t2, len(b.Transactions()))
}

func (d *driver) onCommit(b *innerBlock) { // prepare phase for b
	if CmpBlockHeight(b, d.posvState.GetBExec()) == 1 {
		// commit parent blocks recursively
		d.onCommit(b.Parent())
		d.Commit(b)
	} else if !d.posvState.GetBExec().Equal(b) {
		logger.I().Warnf("safety breached b-recurrsive: %+v, bexec: %d", b, d.posvState.GetBExec().Height())
	}
}

// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"bytes"
	"sync/atomic"
	"time"

	"github.com/wooyang2018/posv-blockchain/core"
	"github.com/wooyang2018/posv-blockchain/emitter"
	"github.com/wooyang2018/posv-blockchain/logger"
	"github.com/wooyang2018/posv-blockchain/storage"
)

type driver struct {
	resources     *Resources
	config        Config
	state         *state
	innerState    *innerState
	tester        *tester
	viewChange    int32 // -1:failed ; 0:success ; 1:ongoing
	qcHighEmitter *emitter.Emitter
	qcEmitter     *emitter.Emitter
}

func newDriver(resources *Resources, config Config, state *state) *driver {
	return &driver{
		resources:     resources,
		config:        config,
		state:         state,
		qcHighEmitter: emitter.New(),
		qcEmitter:     emitter.New(),
	}
}

func (d *driver) CreateProposal() *core.Proposal {
	var txs [][]byte
	if PreserveTxFlag {
		txs = d.resources.TxPool.GetTxsFromQueue(d.config.BlockTxLimit)
	} else {
		txs = d.resources.TxPool.PopTxsFromQueue(d.config.BlockTxLimit)
	}
	parent := d.innerState.GetBLeaf()
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
		SetQuorumCert(d.innerState.GetQCHigh()).
		SetView(d.innerState.GetView()).
		Sign(d.resources.Signer)
	return pro
}

func (d *driver) CreateQuorumCert(iVotes []*core.Vote) *core.QuorumCert {
	votes := make([]*core.Vote, len(iVotes))
	for i, v := range iVotes {
		votes[i] = v
	}
	qc := core.NewQuorumCert().Build(votes)
	return qc
}

func (d *driver) BroadcastProposal(pro *core.Proposal) {
	err := d.resources.MsgSvc.BroadcastProposal(pro)
	if err != nil {
		logger.I().Errorw("broadcast proposal failed", "error", err)
	}
}

func (d *driver) VoteProposal(pro *core.Proposal, blk *core.Block) {
	vote := pro.Vote(d.resources.Signer)
	if !PreserveTxFlag {
		d.resources.TxPool.SetTxsPending(blk.Transactions())
	}
	proposer := d.resources.VldStore.GetWorkerIndex(pro.Proposer())
	if proposer != d.state.getLeaderIndex() {
		return // view changed happened
	}
	d.resources.MsgSvc.SendVote(pro.Proposer(), vote)
	logger.I().Debugw("voted proposal",
		"proposer", proposer,
		"height", blk.Height(),
		"qc", d.qcRefHeight(pro.QuorumCert()),
	)
}

func (d *driver) Commit(bexec *core.Block) {
	start := time.Now()
	rawTxs := bexec.Transactions()
	var txCount int
	var data *storage.CommitData
	if ExecuteTxFlag {
		txs, old := d.resources.TxPool.GetTxsToExecute(rawTxs)
		txCount = len(txs)
		logger.I().Debugw("committing block", "height", bexec.Height(), "txs", txCount)
		bcm, txcs := d.resources.Execution.Execute(bexec, txs)
		bcm.SetOldBlockTxs(old)
		data = &storage.CommitData{
			Block:        bexec,
			QC:           d.state.getQC(bexec.Hash()),
			Transactions: txs,
			BlockCommit:  bcm,
			TxCommits:    txcs,
		}
	} else {
		txCount = len(rawTxs)
		logger.I().Debugw("committing block", "height", bexec.Height(), "txs", txCount)
		bcm, txcs := d.resources.Execution.MockExecute(bexec)
		bcm.SetOldBlockTxs(rawTxs)
		data = &storage.CommitData{
			Block:        bexec,
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

func (d *driver) cleanStateOnCommitted(bexec *core.Block) {
	// qc for bexec is no longer needed here after committed to storage
	d.state.deleteQC(bexec.Hash())
	if !PreserveTxFlag {
		d.resources.TxPool.RemoveTxs(bexec.Transactions())
	}
	d.state.setCommittedBlock(bexec)
	blocks := d.state.getUncommittedOlderBlocks(bexec)
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
func (d *driver) OnPropose() *core.Proposal {
	pro := d.CreateProposal()
	if pro == nil {
		return nil
	}
	d.innerState.setBLeaf(pro.Block())
	d.innerState.startProposal(pro)
	d.BroadcastProposal(pro)
	return pro
}

func (d *driver) NewViewPropose() *core.Proposal {
	qcHigh := d.innerState.GetQCHigh()
	pro := core.NewProposal().
		SetQuorumCert(qcHigh).
		SetView(d.innerState.GetView()).
		Sign(d.resources.Signer)
	if pro == nil {
		return nil
	}
	blk := d.state.getBlock(qcHigh.BlockHash())
	d.innerState.setBLeaf(blk)
	d.innerState.startProposal(pro)
	d.BroadcastProposal(pro)
	return pro
}

// OnReceiveVote is called when received a vote
func (d *driver) OnReceiveVote(vote *core.Vote) error {
	err := d.innerState.addVote(vote)
	if err != nil {
		return err
	}
	blk := d.state.getBlock(vote.BlockHash())
	logger.I().Debugw("received vote", "height", blk.Height())
	if d.innerState.GetVoteCount() >= d.resources.VldStore.MajorityValidatorCount() {
		votes := d.innerState.GetVotes()
		d.innerState.endProposal()
		qc := d.CreateQuorumCert(votes)
		d.UpdateQCHigh(qc)
		d.qcEmitter.Emit(qc)
	}
	return nil
}

func (d *driver) CommitRecursive(b *core.Block) { // prepare phase for b2
	t1 := time.Now().UnixNano()
	d.onCommit(b)
	d.innerState.setBExec(b)
	t2 := time.Now().UnixNano()
	d.tester.saveItem(b.Height(), b.Timestamp(), t1, t2, len(b.Transactions()))
}

func (d *driver) onCommit(b *core.Block) {
	if d.cmpBlockHeight(b, d.innerState.GetBExec()) == 1 {
		// commit parent blocks recursively
		d.onCommit(d.state.getBlock(b.ParentHash()))
		d.Commit(b)
	} else if !bytes.Equal(d.innerState.GetBExec().Hash(), b.Hash()) {
		logger.I().Warnf("safety breached b-recurrsive: %+v, bexec: %d", b, d.innerState.GetBExec().Height())
	}
}

func (d *driver) Lock() {
	d.state.mtxUpdate.Lock()
}

func (d *driver) Unlock() {
	d.state.mtxUpdate.Unlock()
}

func (d *driver) qcRefHeight(qc *core.QuorumCert) (height uint64) {
	ref := d.state.getBlock(qc.BlockHash())
	if ref != nil {
		height = ref.Height()
	}
	return height
}

func (d *driver) qcRefProposer(qc *core.QuorumCert) *core.PublicKey {
	ref := d.state.getBlock(qc.BlockHash())
	if ref == nil {
		return nil
	}
	return ref.Proposer()
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
		return 0
	}
}

// UpdateQCHigh replaces qcHigh if the block of given qc is higher than the qcHigh block
func (d *driver) UpdateQCHigh(qc *core.QuorumCert) {
	if d.cmpQCPriority(qc, d.innerState.GetQCHigh()) < 1 {
		return
	}
	blk := d.state.getBlock(qc.BlockHash())
	logger.I().Debugw("posv updated high qc", "height", d.qcRefHeight(qc))
	d.innerState.setQCHigh(qc)
	d.innerState.setBLeaf(blk)
	d.CommitRecursive(blk)
	d.qcHighEmitter.Emit(qc)
}

func (d *driver) SubscribeNewQCHigh() *emitter.Subscription {
	return d.qcHighEmitter.Subscribe(10)
}

func (d *driver) SubscribeNewQC() *emitter.Subscription {
	return d.qcEmitter.Subscribe(1)
}

// CanVote returns true if the posv instance can vote the given block
func (d *driver) CanVote(pro *core.Proposal) bool {
	return d.cmpQCPriority(pro.QuorumCert(), d.innerState.GetQCHigh()) >= 0
}

func (d *driver) setViewChange(val int32) {
	atomic.StoreInt32(&d.viewChange, val)
}

func (d *driver) getViewChange() int32 {
	return atomic.LoadInt32(&d.viewChange)
}

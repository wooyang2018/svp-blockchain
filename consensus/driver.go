// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wooyang2018/posv-blockchain/core"
	"github.com/wooyang2018/posv-blockchain/emitter"
	"github.com/wooyang2018/posv-blockchain/logger"
	"github.com/wooyang2018/posv-blockchain/storage"
)

type driver struct {
	resources    *Resources
	state        *state
	blockTxLimit int

	bExec  atomic.Value
	qcHigh atomic.Value
	bLeaf  atomic.Value
	view   uint32

	proposal *core.Proposal
	block    *core.Block
	votes    map[string]*core.Vote
	mtx      sync.RWMutex

	mtxUpdate   sync.Mutex // lock for posv update call
	leaderIndex int64

	tester     *tester
	qcEmitter  *emitter.Emitter
	proEmitter *emitter.Emitter
}

func newDriver(resources *Resources, config Config, state *state) *driver {
	return &driver{
		resources:    resources,
		blockTxLimit: config.BlockTxLimit,
		state:        state,
		qcEmitter:    emitter.New(),
		proEmitter:   emitter.New(),
	}
}

func (d *driver) setInnerState(b0 *core.Block, q0 *core.QuorumCert) {
	d.setBLeaf(b0)
	d.setBExec(b0)
	d.setQCHigh(q0)
	d.setView(q0.View())
	// proposer of b0 may not be leader, but it doesn't matter
	d.setLeaderIndex(d.resources.VldStore.GetWorkerIndex(b0.Proposer()))
}

func (d *driver) setBExec(b *core.Block)        { d.bExec.Store(b) }
func (d *driver) setBLeaf(b *core.Block)        { d.bLeaf.Store(b) }
func (d *driver) setQCHigh(qc *core.QuorumCert) { d.qcHigh.Store(qc) }
func (d *driver) setView(num uint32)            { atomic.StoreUint32(&d.view, num) }
func (d *driver) setLeaderIndex(idx int)        { atomic.StoreInt64(&d.leaderIndex, int64(idx)) }

func (d *driver) getBExec() *core.Block       { return d.bExec.Load().(*core.Block) }
func (d *driver) getBLeaf() *core.Block       { return d.bLeaf.Load().(*core.Block) }
func (d *driver) getQCHigh() *core.QuorumCert { return d.qcHigh.Load().(*core.QuorumCert) }
func (d *driver) getView() uint32             { return atomic.LoadUint32(&d.view) }
func (d *driver) getLeaderIndex() int         { return int(atomic.LoadInt64(&d.leaderIndex)) }

func (d *driver) startProposal(b *core.Proposal) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	d.proposal = b
	d.block = b.Block()
	if d.block == nil { //received new view proposal
		d.block = d.state.getBlock(b.QuorumCert().BlockHash())
		if d.block == nil {
			panic("received proposal with nil block")
		}
	}
	d.votes = make(map[string]*core.Vote)
}

func (d *driver) endProposal() {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	d.proposal = nil
	d.block = nil
	d.votes = nil
}

func (d *driver) addVote(vote *core.Vote) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	if d.proposal == nil {
		return fmt.Errorf("no proposal in progress")
	}
	if !bytes.Equal(d.block.Hash(), vote.BlockHash()) {
		return fmt.Errorf("not same block")
	}
	if d.proposal.View() != vote.View() {
		return fmt.Errorf("not same view")
	}
	key := vote.Voter().String()
	if _, found := d.votes[key]; found {
		return fmt.Errorf("duplicate vote")
	}
	d.votes[key] = vote
	return nil
}

func (d *driver) getVoteCount() int {
	d.mtx.RLock()
	defer d.mtx.RUnlock()

	return len(d.votes)
}

func (d *driver) getVotes() []*core.Vote {
	d.mtx.RLock()
	defer d.mtx.RUnlock()

	votes := make([]*core.Vote, 0, len(d.votes))
	for _, v := range d.votes {
		votes = append(votes, v)
	}
	return votes
}

func (d *driver) isLeader(pubKey *core.PublicKey) bool {
	if !d.resources.VldStore.IsWorker(pubKey) {
		return false
	}
	return d.getLeaderIndex() == d.resources.VldStore.GetWorkerIndex(pubKey)
}

func (d *driver) nextLeader() int {
	leaderIdx := d.getLeaderIndex() + 1
	if leaderIdx >= d.resources.VldStore.WorkerCount() {
		leaderIdx = 0
	}
	return leaderIdx
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

func (d *driver) qcRefHeight(qc *core.QuorumCert) (height uint64) {
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

func (d *driver) SubscribeQC() *emitter.Subscription {
	return d.qcEmitter.Subscribe(1)
}

func (d *driver) SubscribeProposal() *emitter.Subscription {
	return d.proEmitter.Subscribe(1)
}

func (d *driver) VoteProposal(pro *core.Proposal, blk *core.Block) {
	if d.cmpQCPriority(pro.QuorumCert(), d.getQCHigh()) < 0 {
		logger.I().Warnf("can not vote proposal height %d", blk.Height())
		return
	}
	vote := pro.Vote(d.resources.Signer)
	if !PreserveTxFlag {
		d.resources.TxPool.SetTxsPending(blk.Transactions())
	}
	proposer := d.resources.VldStore.GetWorkerIndex(pro.Proposer())
	if proposer != d.getLeaderIndex() {
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
	if pro == nil {
		return nil
	}
	d.setBLeaf(pro.Block())
	d.startProposal(pro)
	d.proEmitter.Emit(pro)
	err := d.resources.MsgSvc.BroadcastProposal(pro)
	if err != nil {
		logger.I().Errorw("broadcast proposal failed", "error", err)
	}
	return pro
}

func (d *driver) CreateProposal() *core.Proposal {
	var txs [][]byte
	if PreserveTxFlag {
		txs = d.resources.TxPool.GetTxsFromQueue(d.blockTxLimit)
	} else {
		txs = d.resources.TxPool.PopTxsFromQueue(d.blockTxLimit)
	}
	parent := d.getBLeaf()
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
		SetQuorumCert(d.getQCHigh()).
		SetView(d.getView()).
		Sign(d.resources.Signer)
	return pro
}

func (d *driver) OnNewPropose() *core.Proposal {
	qcHigh := d.getQCHigh()
	pro := core.NewProposal().
		SetQuorumCert(qcHigh).
		SetView(d.getView()).
		Sign(d.resources.Signer)
	if pro == nil {
		return nil
	}
	blk := d.state.getBlock(qcHigh.BlockHash())
	d.setBLeaf(blk)
	d.startProposal(pro)
	d.proEmitter.Emit(pro)
	err := d.resources.MsgSvc.BroadcastProposal(pro)
	if err != nil {
		logger.I().Errorw("broadcast proposal failed", "error", err)
	}
	return pro
}

// OnReceiveVote is called when received a vote
func (d *driver) OnReceiveVote(vote *core.Vote) error {
	err := d.addVote(vote)
	if err != nil {
		return err
	}
	blk := d.state.getBlock(vote.BlockHash())
	logger.I().Debugw("received vote", "height", blk.Height())
	if d.getVoteCount() >= d.resources.VldStore.MajorityValidatorCount() {
		votes := d.getVotes()
		d.endProposal()
		qc := d.CreateQuorumCert(votes)
		d.UpdateQCHigh(qc)
		d.qcEmitter.Emit(qc)
	}
	return nil
}

func (d *driver) CreateQuorumCert(iVotes []*core.Vote) *core.QuorumCert {
	votes := make([]*core.Vote, len(iVotes))
	for i, v := range iVotes {
		votes[i] = v
	}
	qc := core.NewQuorumCert().Build(votes)
	return qc
}

// UpdateQCHigh replaces high qc if the given qc is higher than it
func (d *driver) UpdateQCHigh(qc *core.QuorumCert) {
	if d.cmpQCPriority(qc, d.getQCHigh()) == 1 {
		blk := d.getBlockByHash(qc.BlockHash())
		logger.I().Debugw("updated high qc", "view", qc.View(), "height", d.qcRefHeight(qc))
		d.setQCHigh(qc)
		d.setBLeaf(blk)
		d.CommitRecursive(blk)
	} else {
		logger.I().Warnw("failed to update high qc", "view", qc.View(), "height", d.qcRefHeight(qc))
	}
}

func (d *driver) CommitRecursive(b *core.Block) { // prepare phase for b2
	t1 := time.Now().UnixNano()
	d.OnCommit(b)
	d.setBExec(b)
	t2 := time.Now().UnixNano()
	d.tester.saveItem(b.Height(), b.Timestamp(), t1, t2, len(b.Transactions()))
}

func (d *driver) OnCommit(b *core.Block) {
	if d.cmpBlockHeight(b, d.getBExec()) == 1 {
		// commit parent blocks recursively
		d.OnCommit(d.getBlockByHash(b.ParentHash()))
		d.Commit(b)
	} else if !bytes.Equal(d.getBExec().Hash(), b.Hash()) {
		logger.I().Fatalw("safety breached", "hash", b.Hash(), "height", b.Height())
	}
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

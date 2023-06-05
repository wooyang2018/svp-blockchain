// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"os"
	"time"

	"github.com/wooyang2018/posv-blockchain/logger"
)

// PoSV consensus engine
type posv struct {
	posvState *posvState
	driver    Driver
	tester    *tester
}

func NewPoSV(driver Driver, file *os.File, b0 Block, q0 QC) *posv {
	return &posv{
		posvState: newInnerState(b0, q0),
		driver:    driver,
		tester:    newTester(file),
	}
}

// OnPropose is called to propose a new block
func (posv *posv) OnPropose() Proposal {
	bLeaf := posv.posvState.GetBLeaf()
	bNew := posv.driver.CreateProposal(bLeaf, posv.posvState.GetQCHigh(), bLeaf.Height()+1, 1) //TODO: ViewNum
	if bNew == nil {
		return nil
	}
	posv.posvState.setBLeaf(bNew.Block())
	posv.posvState.startProposal(bNew)
	posv.driver.BroadcastProposal(bNew)
	return bNew
}

// OnReceiveVote is called when received a vote
func (posv *posv) OnReceiveVote(v Vote) error {
	err := posv.posvState.addVote(v)
	if err != nil {
		return err
	}
	logger.I().Debugw("posv received vote", "height", v.Block().Height())
	if posv.posvState.GetVoteCount() >= posv.driver.MajorityValidatorCount() {
		votes := posv.posvState.GetVotes()
		posv.posvState.endProposal()
		posv.UpdateQCHigh(posv.driver.CreateQC(votes))
	}
	return nil
}

// OnReceiveProposal is called when a new proposal is received
func (posv *posv) OnReceiveProposal(bNew Proposal) error {
	posv.Update(bNew)
	if posv.CanVote(bNew) {
		posv.driver.VoteProposal(bNew)
		posv.posvState.setBVote(bNew.Block())
	}
	return nil
}

// CanVote returns true if the posv instance can vote the given block
func (posv *posv) CanVote(bNew Proposal) bool {
	bLock := posv.posvState.GetBVote()
	if bLock.Equal(bNew.Block().Parent()) {
		return true
	}
	return false
}

// Update perform two/three chain consensus phases
func (posv *posv) Update(bNew Proposal) {
	posv.UpdateQCHigh(bNew.Justify()) // prepare phase for b2
	b := bNew.Justify().Block()
	if b != nil {
		t1 := time.Now().UnixNano()
		posv.onCommit(b)
		posv.posvState.setBExec(b)
		t2 := time.Now().UnixNano()
		posv.tester.saveItem(b.Height(), b.Timestamp(), t1, t2, len(b.Transactions()))
	}
}

func (posv *posv) Commit(b Block) { // prepare phase for b2
	t1 := time.Now().UnixNano()
	posv.onCommit(b)
	posv.posvState.setBExec(b)
	t2 := time.Now().UnixNano()
	posv.tester.saveItem(b.Height(), b.Timestamp(), t1, t2, len(b.Transactions()))
}

func (posv *posv) onCommit(b Block) {
	if CmpBlockHeight(b, posv.posvState.GetBExec()) == 1 {
		// commit parent blocks recursively
		posv.onCommit(b.Parent())
		posv.driver.Commit(b)
	} else if !posv.posvState.GetBExec().Equal(b) {
		logger.I().Warnf("safety breached b-recurrsive: %+v, bexec: %d", b, posv.posvState.GetBExec().Height())
	}
}

// UpdateQCHigh replaces qcHigh if the block of given qc is higher than the qcHigh block
func (posv *posv) UpdateQCHigh(qc QC) {
	if CmpBlockHeight(qc.Block(), posv.posvState.GetQCHigh().Block()) == 1 {
		logger.I().Debugw("posv updated high qc", "height", qc.Block().Height())
		posv.posvState.setQCHigh(qc)
		posv.posvState.setBLeaf(qc.Block())
		posv.posvState.qcHighEmitter.Emit(qc)
	}
}

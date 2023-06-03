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
	*innerState
	driver Driver
	tester *tester
}

func NewPoSV(driver Driver, file *os.File, b0 Block, q0 QC) *posv {
	return &posv{
		innerState: newInnerState(b0, q0),
		driver:     driver,
		tester:     newTester(file),
	}
}

// OnPropose is called to propose a new block
func (posv *posv) OnPropose() Block {
	bLeaf := posv.GetBLeaf()
	bNew := posv.driver.CreateLeaf(bLeaf, posv.GetQCHigh(), bLeaf.Height()+1)
	if bNew == nil {
		return nil
	}
	posv.setBLeaf(bNew)
	posv.startProposal(bNew)
	posv.driver.BroadcastProposal(bNew)
	return bNew
}

// OnReceiveVote is called when received a vote
func (posv *posv) OnReceiveVote(v Vote) {
	err := posv.addVote(v)
	if err != nil {
		return
	}
	logger.I().Debugw("posv received vote", "height", v.Block().Height())
	if posv.GetVoteCount() >= posv.driver.MajorityValidatorCount() {
		votes := posv.GetVotes()
		posv.endProposal()
		posv.UpdateQCHigh(posv.driver.CreateQC(votes))
	}
}

// OnReceiveProposal is called when a new proposal is received
func (posv *posv) OnReceiveProposal(bNew Block) {
	posv.Update(bNew)
	if posv.CanVote(bNew) {
		posv.driver.VoteBlock(bNew)
		posv.setBVote(bNew)
	}
}

// CanVote returns true if the posv instance can vote the given block
func (posv *posv) CanVote(bNew Block) bool {
	bLock := posv.GetBVote()
	if bLock.Equal(bNew.Parent()) {
		return true
	}
	return false
}

// Update perform two/three chain consensus phases
func (posv *posv) Update(bNew Block) {
	posv.UpdateQCHigh(bNew.Justify()) // prepare phase for b2
	b := bNew.Justify().Block()
	t1 := time.Now().UnixNano()
	posv.onCommit(b)
	posv.setBExec(b)
	t2 := time.Now().UnixNano()
	posv.tester.saveItem(b.Height(), b.Timestamp(), t1, t2, len(b.Transactions()))
}

func (posv *posv) onCommit(b Block) {
	if CmpBlockHeight(b, posv.GetBExec()) == 1 {
		// commit parent blocks recursively
		posv.onCommit(b.Parent())
		posv.driver.Commit(b)
	} else if !posv.GetBExec().Equal(b) {
		logger.I().Warnf("safety breached b-recurrsive: %+v, bexec: %d", b, posv.GetBExec().Height())
	}
}

// UpdateQCHigh replaces qcHigh if the block of given qc is higher than the qcHigh block
func (posv *posv) UpdateQCHigh(qc QC) {
	if CmpBlockHeight(qc.Block(), posv.GetQCHigh().Block()) == 1 {
		logger.I().Debugw("posv updated high qc", "height", qc.Block().Height())
		posv.setQCHigh(qc)
		posv.setBLeaf(qc.Block())
		posv.qcHighEmitter.Emit(qc)
	}
}

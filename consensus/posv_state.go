// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/wooyang2018/posv-blockchain/emitter"
	"github.com/wooyang2018/posv-blockchain/logger"
)

type posvState struct {
	bVote  atomic.Value
	bExec  atomic.Value
	qcHigh atomic.Value
	bLeaf  atomic.Value

	viewNum  uint32
	proposal *innerProposal
	votes    map[string]*innerVote
	pMtx     sync.RWMutex

	qcHighEmitter *emitter.Emitter
	viewEmitter   *emitter.Emitter
}

func newInnerState(b0 *innerBlock, q0 *innerQC) *posvState {
	s := new(posvState)
	s.qcHighEmitter = emitter.New()
	s.viewEmitter = emitter.New()
	s.setBVote(b0)
	s.setBLeaf(b0)
	s.setBExec(b0)
	s.setQCHigh(q0)
	return s
}

func (s *posvState) setBVote(b *innerBlock) { s.bVote.Store(b) }
func (s *posvState) setBExec(b *innerBlock) { s.bExec.Store(b) }
func (s *posvState) setBLeaf(b *innerBlock) { s.bLeaf.Store(b) }
func (s *posvState) setQCHigh(qc *innerQC)  { s.qcHigh.Store(qc) }

func (s *posvState) SubscribeNewQCHigh() *emitter.Subscription {
	return s.qcHighEmitter.Subscribe(10)
}

func (s *posvState) SubscribeNewView() *emitter.Subscription {
	return s.viewEmitter.Subscribe(10)
}

func (s *posvState) GetBVote() *innerBlock {
	return s.bVote.Load().(*innerBlock)
}

func (s *posvState) GetBExec() *innerBlock {
	return s.bExec.Load().(*innerBlock)
}

func (s *posvState) GetBLeaf() *innerBlock {
	return s.bLeaf.Load().(*innerBlock)
}

func (s *posvState) GetQCHigh() *innerQC {
	return s.qcHigh.Load().(*innerQC)
}

func (s *posvState) GetViewNum() uint32 {
	return s.viewNum
}

func (s *posvState) IsProposing() bool {
	s.pMtx.RLock()
	defer s.pMtx.RUnlock()

	return s.proposal != nil
}

func (s *posvState) startProposal(b *innerProposal) {
	s.pMtx.Lock()
	defer s.pMtx.Unlock()

	s.proposal = b
	s.votes = make(map[string]*innerVote)
}

func (s *posvState) endProposal() {
	s.pMtx.Lock()
	defer s.pMtx.Unlock()

	s.proposal = nil
	s.votes = nil
}

func (s *posvState) addVote(vote *innerVote) error {
	s.pMtx.Lock()
	defer s.pMtx.Unlock()

	if s.proposal == nil {
		return fmt.Errorf("no proposal in progress")
	}
	if !s.proposal.Block().Equal(vote.Block()) {
		return fmt.Errorf("not same block")
	}
	key := vote.Voter()
	if _, found := s.votes[key]; found {
		return fmt.Errorf("duplicate vote")
	}
	s.votes[key] = vote
	return nil
}

func (s *posvState) GetVoteCount() int {
	s.pMtx.RLock()
	defer s.pMtx.RUnlock()

	return len(s.votes)
}

func (s *posvState) GetVotes() []*innerVote {
	s.pMtx.RLock()
	defer s.pMtx.RUnlock()

	votes := make([]*innerVote, 0, len(s.votes))
	for _, v := range s.votes {
		votes = append(votes, v)
	}
	return votes
}

// UpdateQCHigh replaces qcHigh if the block of given qc is higher than the qcHigh block
func (s *posvState) UpdateQCHigh(qc *innerQC) {
	if CmpBlockHeight(qc.Block(), s.GetQCHigh().Block()) == 1 { //TODO 添加View
		logger.I().Debugw("posv updated high qc", "height", qc.Block().Height())
		s.setQCHigh(qc)
		s.setBLeaf(qc.Block())
		s.qcHighEmitter.Emit(qc)
	} else if CmpBlockHeight(qc.Block(), s.GetQCHigh().Block()) == 0 {
		logger.I().Debugw("new view updated high qc", "height", qc.Block().Height())
		s.setQCHigh(qc)
		s.setBLeaf(qc.Block())
		s.qcHighEmitter.Emit(qc)
	}
}

// CanVote returns true if the posv instance can vote the given block
func (s *posvState) CanVote(pro *innerProposal) bool {
	bVote := s.GetBVote()
	if bVote.Equal(pro.Block().Parent()) {
		return true
	}
	return false
}

func (s *posvState) UpdateView(num uint32) {
	if num > s.viewNum {
		s.viewEmitter.Emit(num)
		s.viewNum = num
	}
}

func (s *posvState) LockQCHigh(qc *innerQC) {
	if CmpBlockHeight(qc.Block(), s.GetQCHigh().Block()) == 1 {
		logger.I().Debugw("posv updated high qc", "height", qc.Block().Height())
		s.setQCHigh(qc)
		s.setBLeaf(qc.Block())
	}
}

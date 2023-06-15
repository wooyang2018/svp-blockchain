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
	view   uint32

	proposal *iProposal
	votes    map[string]*iVote
	mtx      sync.RWMutex

	qcHighEmitter *emitter.Emitter
}

func newInnerState(b0 *iBlock, q0 *iQC) *posvState {
	s := new(posvState)
	s.qcHighEmitter = emitter.New()
	s.setBVote(b0)
	s.setBLeaf(b0)
	s.setBExec(b0)
	s.setQCHigh(q0)
	s.setView(q0.View())
	return s
}

func (s *posvState) setBVote(b *iBlock) { s.bVote.Store(b) }
func (s *posvState) setBExec(b *iBlock) { s.bExec.Store(b) }
func (s *posvState) setBLeaf(b *iBlock) { s.bLeaf.Store(b) }
func (s *posvState) setQCHigh(qc *iQC)  { s.qcHigh.Store(qc) }
func (s *posvState) setView(num uint32) { atomic.StoreUint32(&s.view, num) }

func (s *posvState) SubscribeNewQCHigh() *emitter.Subscription {
	return s.qcHighEmitter.Subscribe(10)
}

func (s *posvState) GetBVote() *iBlock {
	return s.bVote.Load().(*iBlock)
}

func (s *posvState) GetBExec() *iBlock {
	return s.bExec.Load().(*iBlock)
}

func (s *posvState) GetBLeaf() *iBlock {
	return s.bLeaf.Load().(*iBlock)
}

func (s *posvState) GetQCHigh() *iQC {
	return s.qcHigh.Load().(*iQC)
}

func (s *posvState) GetView() uint32 {
	return atomic.LoadUint32(&s.view)
}

func (s *posvState) IsProposing() bool {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	return s.proposal != nil
}

func (s *posvState) startProposal(b *iProposal) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.proposal = b
	s.votes = make(map[string]*iVote)
}

func (s *posvState) endProposal() {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.proposal = nil
	s.votes = nil
}

func (s *posvState) addVote(vote *iVote) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if s.proposal == nil {
		return fmt.Errorf("no proposal in progress")
	}
	if s.proposal.Block() != nil && !s.proposal.Block().Equal(vote.Block()) ||
		s.proposal.Block() == nil && vote.Block() == nil {
		return fmt.Errorf("not same block")
	}
	if s.proposal.View() != vote.View() {
		return fmt.Errorf("not same view")
	}
	key := vote.Voter()
	if _, found := s.votes[key]; found {
		return fmt.Errorf("duplicate vote")
	}
	s.votes[key] = vote
	return nil
}

func (s *posvState) GetVoteCount() int {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	return len(s.votes)
}

func (s *posvState) GetVotes() []*iVote {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	votes := make([]*iVote, 0, len(s.votes))
	for _, v := range s.votes {
		votes = append(votes, v)
	}
	return votes
}

// UpdateQCHigh replaces qcHigh if the block of given qc is higher than the qcHigh block
func (s *posvState) UpdateQCHigh(qc *iQC) {
	if cmpQCPriority(qc, s.GetQCHigh()) == 1 {
		logger.I().Debugw("posv updated high qc", "height", qc.Block().Height())
		s.setQCHigh(qc)
		s.setBLeaf(qc.Block())
		s.qcHighEmitter.Emit(qc)
	}
}

// CanVote returns true if the posv instance can vote the given block
func (s *posvState) CanVote(blk *iBlock) bool {
	bVote := s.GetBVote()
	if bVote.Equal(blk.Parent()) {
		return true
	}
	return false
}

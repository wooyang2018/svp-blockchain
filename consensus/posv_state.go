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
	proposal Proposal
	votes    map[string]Vote
	pMtx     sync.RWMutex

	qcHighEmitter *emitter.Emitter
}

func newInnerState(b0 Block, q0 QC) *posvState {
	s := new(posvState)
	s.qcHighEmitter = emitter.New()
	s.setBVote(b0)
	s.setBLeaf(b0)
	s.setBExec(b0)
	s.setQCHigh(q0)
	return s
}

func (s *posvState) setBVote(b Block)    { s.bVote.Store(b) }
func (s *posvState) setBExec(b Block)    { s.bExec.Store(b) }
func (s *posvState) setBLeaf(bNew Block) { s.bLeaf.Store(bNew) }
func (s *posvState) setQCHigh(qcHigh QC) { s.qcHigh.Store(qcHigh) }

func (s *posvState) SubscribeNewQCHigh() *emitter.Subscription {
	return s.qcHighEmitter.Subscribe(10)
}

func (s *posvState) GetBVote() Block {
	return s.bVote.Load().(Block)
}

func (s *posvState) GetBExec() Block {
	return s.bExec.Load().(Block)
}

func (s *posvState) GetBLeaf() Block {
	return s.bLeaf.Load().(Block)
}

func (s *posvState) GetQCHigh() QC {
	return s.qcHigh.Load().(QC)
}

func (s *posvState) IsProposing() bool {
	s.pMtx.RLock()
	defer s.pMtx.RUnlock()

	return s.proposal != nil
}

func (s *posvState) startProposal(b Proposal) {
	s.pMtx.Lock()
	defer s.pMtx.Unlock()

	s.proposal = b
	s.votes = make(map[string]Vote)
}

func (s *posvState) endProposal() {
	s.pMtx.Lock()
	defer s.pMtx.Unlock()

	s.proposal = nil
	s.votes = nil
}

func (s *posvState) addVote(v Vote) error {
	s.pMtx.Lock()
	defer s.pMtx.Unlock()

	if s.proposal == nil {
		return fmt.Errorf("no proposal in progress")
	}
	if !s.proposal.Block().Equal(v.Block()) {
		return fmt.Errorf("not same block")
	}
	key := v.Voter()
	if _, found := s.votes[key]; found {
		return fmt.Errorf("duplicate vote")
	}
	s.votes[key] = v
	return nil
}

func (s *posvState) GetVoteCount() int {
	s.pMtx.RLock()
	defer s.pMtx.RUnlock()

	return len(s.votes)
}

func (s *posvState) GetVotes() []Vote {
	s.pMtx.RLock()
	defer s.pMtx.RUnlock()

	votes := make([]Vote, 0, len(s.votes))
	for _, v := range s.votes {
		votes = append(votes, v)
	}
	return votes
}

// UpdateQCHigh replaces qcHigh if the block of given qc is higher than the qcHigh block
func (s *posvState) UpdateQCHigh(qc QC) {
	if CmpBlockHeight(qc.Block(), s.GetQCHigh().Block()) == 1 {
		logger.I().Debugw("posv updated high qc", "height", qc.Block().Height())
		s.setQCHigh(qc)
		s.setBLeaf(qc.Block())
		s.qcHighEmitter.Emit(qc)
	}
}

// CanVote returns true if the posv instance can vote the given block
func (s *posvState) CanVote(bNew Proposal) bool {
	bLock := s.GetBVote()
	if bLock.Equal(bNew.Block().Parent()) {
		return true
	}
	return false
}

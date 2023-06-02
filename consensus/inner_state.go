// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/wooyang2018/posv-blockchain/emitter"
)

type innerState struct {
	bVote  atomic.Value
	bExec  atomic.Value
	qcHigh atomic.Value
	bLeaf  atomic.Value

	proposal Block
	votes    map[string]Vote
	pMtx     sync.RWMutex

	qcHighEmitter *emitter.Emitter
}

func newInnerState(b0 Block, q0 QC) *innerState {
	s := new(innerState)
	s.qcHighEmitter = emitter.New()
	s.setBVote(b0)
	s.setBLeaf(b0)
	s.setBExec(b0)
	s.setQCHigh(q0)
	return s
}

func (s *innerState) setBVote(b Block)    { s.bVote.Store(b) }
func (s *innerState) setBExec(b Block)    { s.bExec.Store(b) }
func (s *innerState) setBLeaf(bNew Block) { s.bLeaf.Store(bNew) }
func (s *innerState) setQCHigh(qcHigh QC) { s.qcHigh.Store(qcHigh) }

func (s *innerState) SubscribeNewQCHigh() *emitter.Subscription {
	return s.qcHighEmitter.Subscribe(10)
}

func (s *innerState) GetBVote() Block {
	return s.bVote.Load().(Block)
}

func (s *innerState) GetBExec() Block {
	return s.bExec.Load().(Block)
}

func (s *innerState) GetBLeaf() Block {
	return s.bLeaf.Load().(Block)
}

func (s *innerState) GetQCHigh() QC {
	return s.qcHigh.Load().(QC)
}

func (s *innerState) IsProposing() bool {
	s.pMtx.RLock()
	defer s.pMtx.RUnlock()

	return s.proposal != nil
}

func (s *innerState) startProposal(b Block) {
	s.pMtx.Lock()
	defer s.pMtx.Unlock()

	s.proposal = b
	s.votes = make(map[string]Vote)
}

func (s *innerState) endProposal() {
	s.pMtx.Lock()
	defer s.pMtx.Unlock()

	s.proposal = nil
	s.votes = nil
}

func (s *innerState) addVote(v Vote) error {
	s.pMtx.Lock()
	defer s.pMtx.Unlock()

	if s.proposal == nil {
		return fmt.Errorf("no proposal in progress")
	}
	if !s.proposal.Equal(v.Block()) {
		return fmt.Errorf("not same block")
	}
	key := v.Voter()
	if _, found := s.votes[key]; found {
		return fmt.Errorf("duplicate vote")
	}
	s.votes[key] = v
	return nil
}

func (s *innerState) GetVoteCount() int {
	s.pMtx.RLock()
	defer s.pMtx.RUnlock()

	return len(s.votes)
}

func (s *innerState) GetVotes() []Vote {
	s.pMtx.RLock()
	defer s.pMtx.RUnlock()

	votes := make([]Vote, 0, len(s.votes))
	for _, v := range s.votes {
		votes = append(votes, v)
	}
	return votes
}

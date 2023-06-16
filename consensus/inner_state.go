// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/wooyang2018/posv-blockchain/core"
)

type innerState struct {
	bExec  atomic.Value
	qcHigh atomic.Value
	bLeaf  atomic.Value
	view   uint32

	proposal *core.Proposal
	votes    map[string]*core.Vote
	mtx      sync.RWMutex
}

func newInnerState(b0 *core.Block, q0 *core.QuorumCert) *innerState {
	s := new(innerState)
	s.setBLeaf(b0)
	s.setBExec(b0)
	s.setQCHigh(q0)
	s.setView(q0.View())
	return s
}

func (s *innerState) setBExec(b *core.Block)        { s.bExec.Store(b) }
func (s *innerState) setBLeaf(b *core.Block)        { s.bLeaf.Store(b) }
func (s *innerState) setQCHigh(qc *core.QuorumCert) { s.qcHigh.Store(qc) }
func (s *innerState) setView(num uint32)            { atomic.StoreUint32(&s.view, num) }

func (s *innerState) GetBExec() *core.Block {
	return s.bExec.Load().(*core.Block)
}

func (s *innerState) GetBLeaf() *core.Block {
	return s.bLeaf.Load().(*core.Block)
}

func (s *innerState) GetQCHigh() *core.QuorumCert {
	return s.qcHigh.Load().(*core.QuorumCert)
}

func (s *innerState) GetView() uint32 {
	return atomic.LoadUint32(&s.view)
}

func (s *innerState) IsProposing() bool {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	return s.proposal != nil
}

func (s *innerState) startProposal(b *core.Proposal) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.proposal = b
	s.votes = make(map[string]*core.Vote)
}

func (s *innerState) endProposal() {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.proposal = nil
	s.votes = nil
}

func (s *innerState) addVote(vote *core.Vote) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if s.proposal == nil {
		return fmt.Errorf("no proposal in progress")
	}
	if s.proposal.Block() != nil && !bytes.Equal(s.proposal.Block().Hash(), vote.BlockHash()) ||
		s.proposal.Block() == nil && vote.BlockHash() == nil {
		return fmt.Errorf("not same block")
	}
	if s.proposal.View() != vote.View() {
		return fmt.Errorf("not same view")
	}
	key := vote.Voter().String()
	if _, found := s.votes[key]; found {
		return fmt.Errorf("duplicate vote")
	}
	s.votes[key] = vote
	return nil
}

func (s *innerState) GetVoteCount() int {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	return len(s.votes)
}

func (s *innerState) GetVotes() []*core.Vote {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	votes := make([]*core.Vote, 0, len(s.votes))
	for _, v := range s.votes {
		votes = append(votes, v)
	}
	return votes
}

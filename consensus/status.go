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
)

type Status struct {
	StartTime int64

	// committed tx count since node is up
	CommittedTxCount int
	BlockPoolSize    int
	QCPoolSize       int

	// start timestamp of current view
	ViewStart int64
	// set to true when current view timeout
	// set to false once the view leader created the first qc
	ViewChange  int32
	LeaderIndex uint32

	// current status (block height)
	BExec  uint64
	BLeaf  uint64
	QCHigh uint64
	View   uint32
}

type status struct {
	bExec       atomic.Value
	bLeaf       atomic.Value
	qcHigh      atomic.Value
	view        uint32
	leaderIndex uint32
	viewStart   int64 // start timestamp of current view
	viewChange  int32 // -1:failed ; 0:success ; 1:ongoing

	proposal   *core.Proposal
	block      *core.Block
	votes      map[string]*core.Vote
	quotaCount float64
	window     []float64
	mtx        sync.RWMutex
}

func (s *status) setBExec(blk *core.Block)      { s.bExec.Store(blk) }
func (s *status) setBLeaf(blk *core.Block)      { s.bLeaf.Store(blk) }
func (s *status) setQCHigh(qc *core.QuorumCert) { s.qcHigh.Store(qc) }
func (s *status) setView(view uint32)           { atomic.StoreUint32(&s.view, view) }
func (s *status) setLeaderIndex(index uint32)   { atomic.StoreUint32(&s.leaderIndex, index) }
func (s *status) setViewStart()                 { atomic.StoreInt64(&s.viewStart, time.Now().Unix()) }
func (s *status) setViewChange(val int32)       { atomic.StoreInt32(&s.viewChange, val) }

func (s *status) getBExec() *core.Block       { return s.bExec.Load().(*core.Block) }
func (s *status) getBLeaf() *core.Block       { return s.bLeaf.Load().(*core.Block) }
func (s *status) getQCHigh() *core.QuorumCert { return s.qcHigh.Load().(*core.QuorumCert) }
func (s *status) getView() uint32             { return atomic.LoadUint32(&s.view) }
func (s *status) getLeaderIndex() uint32      { return atomic.LoadUint32(&s.leaderIndex) }
func (s *status) getViewStart() int64         { return atomic.LoadInt64(&s.viewStart) }
func (s *status) getViewChange() int32        { return atomic.LoadInt32(&s.viewChange) }

func (s *status) setWindow(quotas []float64) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.window = quotas
}

func (s *status) getWindow() []float64 {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.window
}

func (s *status) startProposal(pro *core.Proposal, blk *core.Block) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.proposal = pro
	s.block = blk
	s.votes = make(map[string]*core.Vote)
	s.quotaCount = 0
}

func (s *status) endProposal() {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.proposal = nil
	s.block = nil
	s.votes = nil
	s.window = s.window[1:]
	s.window = append(s.window, s.quotaCount)
	s.quotaCount = 0
}

func (s *status) addVote(vote *core.Vote) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if s.proposal == nil {
		return fmt.Errorf("no proposal in progress")
	}
	if !bytes.Equal(s.block.Hash(), vote.BlockHash()) {
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
	s.quotaCount += vote.Quota()
	return nil
}

func (s *status) getVoteCount() int {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	return len(s.votes)
}

func (s *status) getQuotaCount() float64 {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	res := s.quotaCount
	for _, v := range s.window {
		res += v
	}
	return res
}

func (s *status) getVotes() []*core.Vote {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	votes := make([]*core.Vote, 0, len(s.votes))
	for _, v := range s.votes {
		votes = append(votes, v)
	}
	return votes
}

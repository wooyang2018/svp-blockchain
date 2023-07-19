// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"bytes"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wooyang2018/posv-blockchain/core"
)

// VoteStrategy vote strategy for validator
type VoteStrategy byte

const (
	_ VoteStrategy = iota
	AverageVote
	RandomVote
)

type Window struct {
	qcQuotas []float64
	qcAcc    float64
	size     int
	height   uint64

	voteLimit float64
	strategy  VoteStrategy
}

func (w *Window) update(val float64, height uint64) {
	if height > w.height {
		w.qcAcc -= w.qcQuotas[0]
		w.qcQuotas = w.qcQuotas[1:]
		w.qcQuotas = append(w.qcQuotas, val)
		w.qcAcc += val
		w.height = height
	} else {
		panic("cannot update window with lower height")
	}
}

func (w *Window) sum() float64 {
	return w.qcAcc - w.qcQuotas[0]
}

func (w *Window) vote() float64 {
	switch w.strategy {
	case AverageVote:
		return w.voteLimit / float64(w.size)
	case RandomVote:
		panic("no support vote strategy")
	default:
		panic("no support vote strategy")
	}
}

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

	proposal   *core.Block
	votes      map[string]*core.Vote
	quotaCount float64
	window     *Window
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

func (s *status) setupWindow(quotas []float64, height uint64, limit float64, strategy VoteStrategy) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.window = &Window{
		qcQuotas:  quotas,
		size:      len(quotas),
		height:    height,
		voteLimit: limit,
		strategy:  strategy,
	}
	for _, v := range quotas {
		s.window.qcAcc += v
	}
}

func (s *status) getWindow() []float64 {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.window.qcQuotas
}

func (s *status) updateWindow(quota float64, height uint64) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.window.update(quota, height)
}

func (s *status) startProposal(blk *core.Block) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.proposal = blk
	s.votes = make(map[string]*core.Vote)
	s.quotaCount = 0
}

func (s *status) endProposal() {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.proposal = nil
	s.votes = nil
	s.quotaCount = 0
}

func (s *status) getVoteQuota() float64 {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.window.vote()
}

func (s *status) addVote(vote *core.Vote) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if s.proposal == nil {
		return errors.New("no proposal in progress")
	}
	if !bytes.Equal(s.proposal.Hash(), vote.BlockHash()) {
		return errors.New("not same block")
	}
	if s.proposal.View() != vote.View() {
		return errors.New("not same view")
	}
	key := vote.Voter().String()
	if _, found := s.votes[key]; found {
		return errors.New("duplicate vote")
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

	return s.quotaCount + s.window.sum()
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

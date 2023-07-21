// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"bytes"
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shopspring/decimal"
	"github.com/wooyang2018/posv-blockchain/core"
	"github.com/wooyang2018/posv-blockchain/logger"
)

// VoteStrategy vote strategy for validator
type VoteStrategy byte

const (
	_ VoteStrategy = iota
	AverageVote
	RandomVote
)

type window struct {
	qcQuotas []float64
	qcAcc    float64

	voteLimit  float64
	voteQuotas []float64
	voteAcc    float64
	strategy   VoteStrategy

	height uint64
	size   int
}

func (w *window) update(qc, vote float64, height uint64) {
	if height > w.height {
		w.qcAcc -= w.qcQuotas[0]
		w.qcQuotas = w.qcQuotas[1:]
		w.qcQuotas = append(w.qcQuotas, qc)
		w.qcAcc += qc

		w.voteAcc -= w.voteQuotas[0]
		w.voteQuotas = w.voteQuotas[1:]
		w.voteQuotas = append(w.voteQuotas, vote)
		w.voteAcc += vote

		w.height = height
	} else {
		logger.I().Error("must update window with higher height")
	}
}

func (w *window) vote() float64 {
	switch w.strategy {
	case AverageVote:
		return w.voteLimit / float64(w.size)
	case RandomVote:
		if w.height > 50 {
			max := decimal.NewFromFloat(w.voteLimit)
			max = max.Sub(decimal.NewFromFloat(w.voteAcc))
			max = max.Add(decimal.NewFromFloat(w.voteQuotas[0]))
			quota, _ := max.Round(2).Float64()
			return quota
		}
		pre := w.voteAcc - w.voteQuotas[0]
		max := w.voteLimit - pre
		min := 1/2*w.voteLimit - pre + 0.01
		if min < 0 {
			min = 0
		}
		tmp := decimal.NewFromFloat(rand.Float64() * (max - min))
		tmp = tmp.Add(decimal.NewFromFloat(min))
		quota, _ := tmp.Round(2).Float64()
		return quota
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
	window     *window
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

func (s *status) getQCWindow() []float64 {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.window.qcQuotas
}

func (s *status) getVoteWindow() []float64 {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.window.voteQuotas
}

func (s *status) updateWindow(qc, vote float64, height uint64) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.window.update(qc, vote, height)
}

func (s *status) getQuotaCount() float64 {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	return s.quotaCount + s.window.qcAcc - s.window.qcQuotas[0]
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

func (s *status) getVotes() []*core.Vote {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	votes := make([]*core.Vote, 0, len(s.votes))
	for _, v := range s.votes {
		votes = append(votes, v)
	}
	return votes
}

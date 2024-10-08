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

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/logger"
)

const (
	AverageVote uint8 = iota
	RandomVote
	OrdinaryVote
	MonopolyVote
)

type window struct {
	qcQuotas   []uint64
	qcAcc      uint64
	voteQuotas []uint64
	voteAcc    uint64

	majority uint64
	limit    uint64
	height   uint64
	size     int

	strategy uint8
	recover  bool
}

func (w *window) update(qc, vote uint64, height uint64) {
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

func (w *window) vote() (uint64, bool) {
	switch w.strategy {
	case AverageVote:
		//several blocks starting with genesis block may not satisfy the rule for window voting
		return w.limit / uint64(w.size), true
	case RandomVote:
		if !PreserveTxFlag && w.height > 50 {
			return w.limit - w.voteAcc + w.voteQuotas[0], true
		}
		pre := w.voteAcc - w.voteQuotas[0]
		max := (3*w.limit+3)/4 - pre
		if int((3*w.limit+3)/4-pre) < 0 {
			max = 0
		}
		min := (w.limit+1)/2 - pre
		if int((w.limit+1)/2-pre) < 0 {
			min = 0
		}
		quota := rand.Uint64()%(max-min+1) + min
		time.Sleep(time.Duration(w.size-4) * 15 * time.Millisecond)
		return quota, true
	case OrdinaryVote: //only for experiment 3: ordinary BFT validators over f
		//makes it impossible to exceed the stake threshold for window of height 100 to 99+w.size
		if !w.recover && w.height >= 100 && w.height <= 99+uint64(w.size) &&
			w.limit >= w.majority {
			return 0, false
		}
		return w.limit / uint64(w.size), true
	case MonopolyVote: //only for experiment 4: monopoly BFT validators with 1/2s stake
		if !w.recover && w.height == 100 { //BFT block at height 100
			if w.limit >= w.majority {
				return w.limit - w.voteAcc + w.voteQuotas[0], true
			} else {
				return 0, false
			}
		}
		return w.limit / uint64(w.size), true
	default:
		panic("no support voting strategy")
	}
}

type Status struct {
	StartTime int64

	CommittedTxCount int // committed tx count since node is up
	BlockPoolSize    int
	QCPoolSize       int

	ViewStart   int64 // start timestamp of current view
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
	quotaCount uint64
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

func (s *status) getQCWindow() []uint64 {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.window.qcQuotas
}

func (s *status) getVoteWindow() []uint64 {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.window.voteQuotas
}

func (s *status) updateWindow(qc, vote uint64, height uint64) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.window.update(qc, vote, height)
}

func (s *status) recoverFlag() {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.window.recover = true
}

func (s *status) getQuotaCount() uint64 {
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

func (s *status) getVoteQuota() (uint64, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	quota, ok := s.window.vote()
	if !ok {
		return 0, errors.New("cannot vote because of strategy")
	}
	return quota, nil
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

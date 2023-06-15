// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"sync/atomic"
	"time"

	"github.com/wooyang2018/posv-blockchain/logger"
)

type rotator struct {
	resources *Resources
	config    Config

	state  *state
	driver *driver

	leaderTimer        *time.Timer
	viewTimer          *time.Timer
	leaderTimeoutCount int

	viewStart  int64 // start timestamp of current view
	viewChange int32 // -1:failed ; 0:success ; 1:ongoing

	stopCh chan struct{}
}

func (rot *rotator) start() {
	if rot.stopCh != nil {
		return
	}
	rot.stopCh = make(chan struct{})
	rot.setViewStart()
	go rot.run()
	logger.I().Info("started rotator")
}

func (rot *rotator) stop() {
	if rot.stopCh == nil {
		return // not started yet
	}
	select {
	case <-rot.stopCh: // already stopped
		return
	default:
	}
	close(rot.stopCh)
	logger.I().Info("stopped rotator")
	rot.stopCh = nil
}

func (rot *rotator) run() {
	subQC := rot.driver.posvState.SubscribeNewQCHigh()
	defer subQC.Unsubscribe()

	rot.viewTimer = time.NewTimer(rot.config.ViewWidth)
	defer rot.viewTimer.Stop()

	rot.leaderTimer = time.NewTimer(rot.config.LeaderTimeout)
	defer rot.leaderTimer.Stop()

	for {
		select {
		case <-rot.stopCh:
			return

		case <-rot.viewTimer.C:
			rot.onViewTimeout()

		case <-rot.leaderTimer.C:
			rot.onLeaderTimeout()

		case e := <-subQC.Events():
			rot.onNewQCHigh(e.(*iQC))
		}
	}
}

func (rot *rotator) drainStopTimer(timer *time.Timer) {
	if !timer.Stop() { // timer triggered before another stop/reset call
		t := time.NewTimer(5 * time.Millisecond)
		defer t.Stop()
		select {
		case <-timer.C:
		case <-t.C: // to make sure it's not stuck more than 5ms
		}
	}
}

func (rot *rotator) onLeaderTimeout() {
	logger.I().Warnw("leader timeout", "leader", rot.state.getLeaderIndex())
	rot.leaderTimeoutCount++
	rot.changeView()
	rot.drainStopTimer(rot.leaderTimer)
	if rot.leaderTimeoutCount > rot.state.getFaultyCount() {
		rot.leaderTimer.Stop()
		rot.setViewChange(-1) //failed to change view when leader timeout
	} else {
		rot.leaderTimer.Reset(rot.config.LeaderTimeout)
	}
}

func (rot *rotator) onViewTimeout() {
	rot.changeView()
	rot.drainStopTimer(rot.leaderTimer)
	rot.leaderTimer.Reset(rot.config.LeaderTimeout)
}

func (rot *rotator) changeView() {
	rot.setViewChange(1)
	t := time.NewTimer(rot.config.Delta)
	select {
	case <-rot.stopCh:
		return
	case <-t.C:
	}

	rot.setViewStart()
	leaderIdx := rot.nextLeader()
	rot.state.setLeaderIndex(leaderIdx)
	leader := rot.resources.VldStore.GetWorker(leaderIdx)
	err := rot.resources.MsgSvc.SendQC(leader, rot.driver.posvState.GetQCHigh().qc)
	if err != nil {
		logger.I().Errorw("send high qc to new leader failed", "error", err)
	}
	rot.driver.posvState.setView(rot.driver.posvState.GetView() + 1)
	logger.I().Infow("view changed", "view", rot.driver.posvState.GetView(),
		"leader", leaderIdx, "qc", qcRefHeight(rot.driver.posvState.GetQCHigh()))

	if !rot.state.isThisNodeLeader() {
		t := time.NewTimer(rot.config.Delta * 2)
		select {
		case <-rot.stopCh:
			return
		case <-t.C:
			rot.newViewProposal()
		}
	}
}

func (rot *rotator) newViewProposal() {
	rot.state.mtxUpdate.Lock()
	defer rot.state.mtxUpdate.Unlock()

	pro := rot.driver.NewViewPropose()
	logger.I().Debugw("proposed new view block", "view", pro.View(), "qc", qcRefHeight(pro.Justify()))
	vote := pro.proposal.Vote(rot.resources.Signer)
	rot.driver.OnReceiveVote(newVote(vote, rot.state))
	rot.driver.Update(pro.Justify())
}

func (rot *rotator) nextLeader() int {
	leaderIdx := rot.state.getLeaderIndex() + 1
	if leaderIdx >= rot.resources.VldStore.WorkerCount() {
		leaderIdx = 0
	}
	return leaderIdx
}

func (rot *rotator) onNewQCHigh(qc *iQC) {
	rot.state.setQC(qc.qc)
	proposer := rot.resources.VldStore.GetWorkerIndex(qcRefProposer(qc))
	logger.I().Debugw("updated qc", "proposer", proposer, "qc", qcRefHeight(qc))
	var ltreset, vtreset bool
	if proposer == rot.state.getLeaderIndex() { // if qc is from current leader
		ltreset = true
	}
	if rot.isNewViewApproval(proposer) {
		ltreset = true
		vtreset = true
		rot.approveViewLeader(proposer)
	}
	if ltreset {
		rot.drainStopTimer(rot.leaderTimer)
		rot.leaderTimer.Reset(rot.config.LeaderTimeout)
	}
	if vtreset {
		rot.drainStopTimer(rot.viewTimer)
		rot.viewTimer.Reset(rot.config.ViewWidth)
	}
}

func (rot *rotator) isNewViewApproval(proposer int) bool {
	leaderIdx := rot.state.getLeaderIndex()
	pending := rot.getViewChange()
	return (pending == 0 && proposer != leaderIdx) || // node first run or out of sync
		(pending == 1 && proposer == leaderIdx) // expecting leader
}

func (rot *rotator) approveViewLeader(proposer int) {
	rot.setViewChange(0)
	rot.state.setLeaderIndex(proposer)
	rot.setViewStart()
	logger.I().Infow("approved leader", "leader", rot.state.getLeaderIndex())
	rot.leaderTimeoutCount = 0
}

func (rot *rotator) setViewStart() {
	atomic.StoreInt64(&rot.viewStart, time.Now().Unix())
}

func (rot *rotator) getViewStart() int64 {
	return atomic.LoadInt64(&rot.viewStart)
}

func (rot *rotator) setViewChange(val int32) {
	atomic.StoreInt32(&rot.viewChange, val)
}

func (rot *rotator) getViewChange() int32 {
	return atomic.LoadInt32(&rot.viewChange)
}

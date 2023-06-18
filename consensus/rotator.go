// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"sync/atomic"
	"time"

	"github.com/wooyang2018/posv-blockchain/core"
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
	pro := rot.driver.SubscribeProposal()
	defer pro.Unsubscribe()

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

		case e := <-pro.Events():
			rot.onNewProposal(e.(*core.Proposal))
		}
	}
}

func (rot *rotator) onLeaderTimeout() {
	logger.I().Warnw("leader timeout", "leader", rot.driver.getLeaderIndex())
	rot.leaderTimeoutCount++
	rot.changeView()
	rot.drainStopTimer(rot.leaderTimer)
	faultyCount := rot.resources.RoleStore.ValidatorCount() - rot.resources.RoleStore.MajorityValidatorCount()
	if rot.leaderTimeoutCount > faultyCount {
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
	leaderIdx := rot.driver.nextLeader()
	rot.driver.setLeaderIndex(uint32(leaderIdx))
	rot.driver.setView(rot.driver.getView() + 1)
	logger.I().Infow("view changed", "view", rot.driver.getView(),
		"leader", leaderIdx, "qc", rot.driver.qcRefHeight(rot.driver.getQCHigh()))

	isLeader := rot.driver.isLeader(rot.resources.Signer.PublicKey())
	if isLeader {
		t := time.NewTimer(rot.config.Delta * 2)
		select {
		case <-rot.stopCh:
			return
		case <-t.C:
			rot.newViewProposal()
		}
	} else {
		leader := rot.resources.RoleStore.GetValidator(leaderIdx)
		err := rot.resources.MsgSvc.SendQC(leader, rot.driver.getQCHigh())
		if err != nil {
			logger.I().Errorw("send high qc to new leader failed", "idx", leaderIdx, "error", err)
		}
	}
}

func (rot *rotator) newViewProposal() {
	rot.driver.mtxUpdate.Lock()
	defer rot.driver.mtxUpdate.Unlock()

	if !rot.driver.isLeader(rot.resources.Signer.PublicKey()) {
		return
	}

	pro := rot.driver.OnNewViewPropose()
	logger.I().Debugw("proposed new view proposal", "view", pro.View(), "qc", rot.driver.qcRefHeight(pro.QuorumCert()))
	vote := pro.Vote(rot.resources.Signer)
	rot.driver.OnReceiveVote(vote)
	rot.driver.UpdateQCHigh(pro.QuorumCert())
}

func (rot *rotator) onNewProposal(pro *core.Proposal) {
	proposer := uint32(rot.resources.RoleStore.GetValidatorIndex(pro.Proposer()))
	var ltreset, vtreset bool
	if rot.isNormalApproval(pro.View(), proposer) {
		ltreset = true
	}
	if rot.isNewViewApproval(pro.View(), proposer) {
		ltreset = true
		vtreset = true
		rot.approveViewLeader(pro.View(), proposer)
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

func (rot *rotator) isNewViewApproval(view uint32, proposer uint32) bool {
	leaderIdx := rot.driver.getLeaderIndex()
	pending := rot.getViewChange()
	return view > rot.driver.getView() || (pending == 0 && proposer != leaderIdx) || // node first run or out of sync
		(pending == 1 && proposer == leaderIdx) // expecting leader
}

func (rot *rotator) isNormalApproval(view uint32, proposer uint32) bool {
	leaderIdx := rot.driver.getLeaderIndex()
	return view == rot.driver.getView() && proposer == leaderIdx
}

func (rot *rotator) approveViewLeader(view uint32, proposer uint32) {
	rot.setViewChange(0)
	rot.driver.setView(view)
	rot.driver.setLeaderIndex(proposer)
	rot.setViewStart()
	logger.I().Infow("approved leader", "view", view, "leader", rot.driver.getLeaderIndex())
	rot.leaderTimeoutCount = 0
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

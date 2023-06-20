// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"time"

	"github.com/wooyang2018/posv-blockchain/core"
	"github.com/wooyang2018/posv-blockchain/logger"
)

type rotator struct {
	resources *Resources
	config    Config
	driver    *driver

	leaderTimer        *time.Timer
	viewTimer          *time.Timer
	leaderTimeoutCount int

	stopCh chan struct{}
}

func (rot *rotator) start() {
	if rot.stopCh != nil {
		return
	}
	rot.stopCh = make(chan struct{})
	rot.driver.setViewStart()
	go rot.newViewLoop()
	go rot.proposalLoop()
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

func (rot *rotator) newViewLoop() {
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
		}
	}
}

func (rot *rotator) proposalLoop() {
	for {
		select {
		case <-rot.stopCh:
			return
		case e := <-rot.driver.proposalCh:
			rot.onNewProposal(e)
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
		rot.driver.setViewChange(-1) //failed to change view when leader timeout
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
	rot.driver.setViewChange(1)
	rot.sleepTime(rot.config.DeltaTime)
	if err := rot.resources.MsgSvc.BroadcastQC(rot.driver.getQCHigh()); err != nil {
		logger.I().Errorw("send high qc to new leader failed", "error", err)
	}
	rot.sleepTime(rot.config.DeltaTime * 2)
	rot.driver.setViewStart()
	rot.driver.setView(rot.driver.getView() + 1)
	leaderIdx := rot.driver.getView() % uint32(rot.resources.RoleStore.ValidatorCount())
	rot.driver.setLeaderIndex(leaderIdx)
	logger.I().Infow("view changed", "view", rot.driver.getView(),
		"leader", rot.driver.getLeaderIndex(), "qc", rot.driver.qcRefHeight(rot.driver.getQCHigh()))
	rot.newViewProposal()
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
		logger.I().Debugw("refresh leader", "view", pro.View(), "proposer", proposer)
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

func (rot *rotator) isNormalApproval(view uint32, proposer uint32) bool {
	curView := rot.driver.getView()
	leaderIdx := rot.driver.getLeaderIndex()
	pending := rot.driver.getViewChange()
	return pending == 0 && view == curView && proposer == leaderIdx
}

func (rot *rotator) isNewViewApproval(view uint32, proposer uint32) bool {
	curView := rot.driver.getView()
	if view > curView {
		return true
	} else if view == curView {
		leaderIdx := rot.driver.getLeaderIndex()
		pending := rot.driver.getViewChange()
		return pending == 0 && proposer != leaderIdx || pending == 1 && proposer == leaderIdx
	}
	return false
}

func (rot *rotator) approveViewLeader(view uint32, proposer uint32) {
	rot.driver.setViewChange(0)
	rot.driver.setView(view)
	rot.driver.setLeaderIndex(proposer)
	rot.driver.setViewStart()
	rot.leaderTimeoutCount = 0
	logger.I().Infow("approved leader", "view", view, "leader", proposer)
}

func (rot *rotator) sleepTime(delta time.Duration) {
	t := time.NewTimer(delta)
	defer t.Stop()
	select {
	case <-rot.stopCh:
		return
	case <-t.C:
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

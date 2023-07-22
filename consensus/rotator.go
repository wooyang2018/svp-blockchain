// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"fmt"
	"time"

	"github.com/wooyang2018/posv-blockchain/core"
	"github.com/wooyang2018/posv-blockchain/logger"
)

type rotator struct {
	resources *Resources
	config    Config
	state     *state
	status    *status
	driver    *driver

	leaderTimer  *time.Timer
	viewTimer    *time.Timer
	timeoutCount int
	highQCCount  int
	newViewCh    chan struct{}
	stopCh       chan struct{}
}

func (rot *rotator) start() {
	if rot.stopCh != nil {
		return
	}
	rot.stopCh = make(chan struct{})
	rot.status.setViewStart()
	go rot.proposalLoop()
	go rot.newViewLoop()
	go rot.timerLoop()
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

func (rot *rotator) proposalLoop() {
	for {
		select {
		case <-rot.stopCh:
			return

		case blk := <-rot.driver.receiveCh:
			rot.onReceiveProposal(blk)
		}
	}
}

func (rot *rotator) newViewLoop() {
	sub := rot.resources.MsgSvc.SubscribeQC(10)
	defer sub.Unsubscribe()

	for {
		select {
		case <-rot.stopCh:
			return

		case e := <-sub.Events():
			if err := rot.onReceiveQC(e.(*core.QuorumCert)); err != nil {
				logger.I().Warnf("receive qc failed, %+v", err)
			}
		}
	}
}

func (rot *rotator) timerLoop() {
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

func (rot *rotator) onReceiveProposal(blk *core.Block) {
	rot.driver.mtxUpdate.Lock()
	defer rot.driver.mtxUpdate.Unlock()

	proposer := uint32(rot.resources.RoleStore.GetValidatorIndex(blk.Proposer()))
	var ltreset, vtreset bool
	if rot.isNormalApproval(blk.View(), proposer) {
		ltreset = true
		logger.I().Debugw("refreshed leader",
			"view", blk.View(),
			"leader", proposer)
	}
	if rot.isNewViewApproval(blk.View(), proposer) {
		ltreset = true
		vtreset = true
		rot.approveViewLeader(blk.View(), proposer)
	}
	if ltreset {
		drainStopTimer(rot.leaderTimer)
		rot.leaderTimer.Reset(rot.config.LeaderTimeout)
	}
	if vtreset {
		drainStopTimer(rot.viewTimer)
		rot.viewTimer.Reset(rot.config.ViewWidth)
	}
}

func (rot *rotator) isNormalApproval(view uint32, proposer uint32) bool {
	curView := rot.status.getView()
	leaderIdx := rot.status.getLeaderIndex()
	pending := rot.status.getViewChange()
	return pending == 0 && view == curView && proposer == leaderIdx
}

func (rot *rotator) isNewViewApproval(view uint32, proposer uint32) bool {
	curView := rot.status.getView()
	if view > curView {
		return true
	} else if view == curView {
		leaderIdx := rot.status.getLeaderIndex()
		pending := rot.status.getViewChange()
		return pending == 0 && proposer != leaderIdx ||
			pending == 1 && proposer == leaderIdx
	}
	return false
}

func (rot *rotator) approveViewLeader(view uint32, proposer uint32) {
	rot.status.setViewChange(0)
	rot.status.setView(view)
	rot.status.setLeaderIndex(proposer)
	rot.status.setViewStart()
	rot.timeoutCount = 0
	rot.highQCCount = 0
	logger.I().Infow("approved leader",
		"view", view,
		"leader", proposer)
}

func (rot *rotator) onReceiveQC(qc *core.QuorumCert) error {
	var err error
	if err = qc.Validate(rot.resources.RoleStore); err != nil {
		return err
	}
	blk := rot.driver.getBlockByHash(qc.BlockHash())
	if blk == nil {
		if blk, err = rot.driver.requestBlock(qc.Proposer(), qc.BlockHash()); err != nil {
			return err
		}
	}
	if _, err = rot.driver.syncParentBlock(blk); err != nil { // fetch parent block recursively
		return err
	}
	if ExecuteTxFlag { // must sync transactions before updating block
		if err = rot.resources.TxPool.SyncTxs(qc.Proposer(), blk.Transactions()); err != nil {
			return fmt.Errorf("sync txs failed, %w", err)
		}
	}

	rot.driver.mtxUpdate.Lock()
	defer rot.driver.mtxUpdate.Unlock()

	rot.driver.updateQCHigh(qc)
	rot.highQCCount++
	if rot.highQCCount >= rot.resources.RoleStore.MajorityValidatorCount() {
		rot.newViewCh <- struct{}{}
		rot.highQCCount = 0
	}
	return nil
}

func (rot *rotator) onLeaderTimeout() {
	logger.I().Warnw("leader timeout", "view", rot.status.getView(), "leader", rot.status.getLeaderIndex())
	rot.timeoutCount++
	rot.changeView()
	drainStopTimer(rot.leaderTimer)
	faultyCount := rot.resources.RoleStore.ValidatorCount() - rot.resources.RoleStore.MajorityValidatorCount()
	if rot.timeoutCount > faultyCount {
		rot.leaderTimer.Stop()
		rot.status.setViewChange(-1) //failed to change view when leader timeout
	} else {
		rot.leaderTimer.Reset(rot.config.LeaderTimeout)
	}
}

func (rot *rotator) onViewTimeout() {
	logger.I().Warnw("view timeout", "view", rot.status.getView(), "leader", rot.status.getLeaderIndex())
	rot.changeView()
	drainStopTimer(rot.leaderTimer)
	rot.leaderTimer.Reset(rot.config.LeaderTimeout)
}

func (rot *rotator) changeView() {
	rot.status.setViewChange(1)
	view := rot.status.getView()
	if err := rot.resources.MsgSvc.BroadcastQC(rot.status.getQCHigh()); err != nil {
		logger.I().Errorf("broadcast qc failed, %+v", err)
	}
	rot.highQCCount++
	<-rot.newViewCh //wait to receive n-f qcs
	time.Sleep(1 * time.Second)

	rot.driver.mtxUpdate.Lock()
	defer rot.driver.mtxUpdate.Unlock()

	if view == rot.status.getView() {
		rot.status.setView(rot.status.getView() + 1)
		leaderIdx := rot.status.getView() % uint32(rot.resources.RoleStore.ValidatorCount())
		rot.status.setLeaderIndex(leaderIdx)

		logger.I().Infow("view changed",
			"view", rot.status.getView(),
			"leader", rot.status.getLeaderIndex(),
			"qc", rot.driver.qcRefHeight(rot.status.getQCHigh()))

		if rot.driver.isLeader(rot.resources.Signer.PublicKey()) {
			rot.newViewProposal()
		}
	}
}

func (rot *rotator) newViewProposal() {
	qcHigh := rot.status.getQCHigh()
	parent := rot.driver.getBlockByHash(qcHigh.BlockHash())
	blk := rot.driver.createProposal(rot.status.getView(), parent, qcHigh)
	if err := rot.resources.MsgSvc.BroadcastProposal(blk); err != nil {
		logger.I().Errorf("broadcast proposal failed, %+v", err)
	}
	vote := blk.Vote(rot.resources.Signer, rot.status.getVoteQuota())
	rot.driver.onReceiveVote(vote)
	rot.driver.updateQCHigh(blk.QuorumCert())
}

func drainStopTimer(timer *time.Timer) {
	if !timer.Stop() {
		t := time.NewTimer(5 * time.Millisecond)
		defer t.Stop()
		select {
		case <-timer.C:
		case <-t.C: // to make sure it's not stuck more than 5ms
		}
	}
}

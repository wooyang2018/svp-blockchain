// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"fmt"
	"sync"
	"time"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/logger"
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
	oldView      uint32
	mtxView      sync.Mutex

	stopCh chan struct{}
}

func (rot *rotator) start() {
	if rot.stopCh != nil {
		return
	}
	rot.stopCh = make(chan struct{})
	rot.status.setViewStart()
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
	var ltreset, vtreset bool
	proposer := uint32(rot.resources.RoleStore.GetValidatorIndex(blk.Proposer().String()))
	if rot.isNormalApproval(blk.View(), proposer) {
		ltreset = true
		logger.I().Debugw("refreshed leader",
			"view", blk.View(),
			"leader", proposer)
	}

	rot.mtxView.Lock()
	if rot.isNewViewApproval(blk.View(), proposer) {
		ltreset = true
		vtreset = true
		rot.approveViewLeader(blk.View(), proposer)
	}
	rot.mtxView.Unlock()

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
	if !TwoPhaseBFTFlag && view == 2 {
		rot.status.recoverFlag() // only for experiments to verify security
	}
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
	rot.driver.updateQCHigh(qc)
	rot.driver.mtxUpdate.Unlock()
	logger.I().Debugw("received qc",
		"view", qc.View(),
		"height", blk.Height(),
		"count", rot.highQCCount)

	if rot.status.getViewChange() == 1 && rot.oldView == rot.status.getView() {
		rot.status.setView(rot.oldView + 1)
		nextIdx := (rot.oldView + 1) % uint32(rot.resources.RoleStore.ValidatorCount())
		rot.status.setLeaderIndex(nextIdx)
		logger.I().Infow("view changed",
			"view", rot.status.getView(),
			"leader", rot.status.getLeaderIndex(),
			"qc", rot.driver.qcRefHeight(rot.status.getQCHigh()))
	}

	rot.highQCCount++
	if rot.highQCCount >= rot.resources.RoleStore.MajorityValidatorCount() {
		if rot.driver.isLeader(rot.resources.Signer.PublicKey()) {
			rot.driver.mtxUpdate.Lock()
			rot.newViewProposal()
			rot.driver.mtxUpdate.Unlock()
		}
	}
	return nil
}

func (rot *rotator) onLeaderTimeout() {
	view := rot.status.getView()
	logger.I().Warnw("leader timeout", "view", view, "leader", rot.status.getLeaderIndex())
	rot.timeoutCount++
	rot.oldView = view
	rot.changeView()
	drainStopTimer(rot.leaderTimer)

	faultyCount := rot.resources.RoleStore.ValidatorCount() - rot.resources.RoleStore.MajorityValidatorCount()
	if rot.timeoutCount > faultyCount {
		rot.leaderTimer.Stop()
		rot.status.setViewChange(-1) // failed to change view when leader timeout
	} else {
		rot.leaderTimer.Reset(rot.config.LeaderTimeout)
	}
}

func (rot *rotator) onViewTimeout() {
	view := rot.status.getView()
	logger.I().Warnw("view timeout", "view", view, "leader", rot.status.getLeaderIndex())
	rot.oldView = view
	rot.changeView()
	drainStopTimer(rot.leaderTimer)
	rot.leaderTimer.Reset(rot.config.LeaderTimeout)
}

func (rot *rotator) changeView() {
	nextIdx := (rot.oldView + 1) % uint32(rot.resources.RoleStore.ValidatorCount())
	nextLeader := rot.resources.RoleStore.GetValidator(int(nextIdx))

	rot.mtxView.Lock()
	defer rot.mtxView.Unlock()
	if rot.oldView == rot.status.getView() {
		rot.status.setViewChange(1)
		rot.highQCCount = 0
		rot.resources.Host.SetLeader(int(nextIdx))
		if !rot.resources.Signer.PublicKey().Equal(nextLeader) {
			rot.resources.MsgSvc.SendQC(nextLeader, rot.status.getQCHigh())
		} else {
			rot.highQCCount++
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

	rot.driver.updateQCHigh(blk.QuorumCert())

	var quota uint64 = 1
	if !TwoPhaseBFTFlag {
		var err error
		if quota, err = rot.status.getVoteQuota(); err != nil {
			return
		}
	}
	vote := blk.Vote(rot.resources.Signer, quota)
	rot.driver.onReceiveVote(vote)
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

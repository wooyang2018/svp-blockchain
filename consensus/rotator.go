// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"time"

	"github.com/wooyang2018/posv-blockchain/core"
	"github.com/wooyang2018/posv-blockchain/logger"
)

func (d *driver) newViewLoop() {
	d.viewTimer = time.NewTimer(d.config.ViewWidth)
	defer d.viewTimer.Stop()

	d.leaderTimer = time.NewTimer(d.config.LeaderTimeout)
	defer d.leaderTimer.Stop()

	for {
		select {
		case <-d.stopCh:
			return

		case <-d.viewTimer.C:
			d.onViewTimeout()

		case <-d.leaderTimer.C:
			d.onLeaderTimeout()
		}
	}
}

func (d *driver) onLeaderTimeout() {
	logger.I().Warnw("leader timeout", "leader", d.status.getLeaderIndex())
	d.leaderTimeoutCount++
	d.changeView()
	drainStopTimer(d.leaderTimer)
	faultyCount := d.resources.RoleStore.ValidatorCount() - d.resources.RoleStore.MajorityValidatorCount()
	if d.leaderTimeoutCount > faultyCount {
		d.leaderTimer.Stop()
		d.status.setViewChange(-1) //failed to change view when leader timeout
	} else {
		d.leaderTimer.Reset(d.config.LeaderTimeout)
	}
}

func (d *driver) onViewTimeout() {
	logger.I().Warnw("view timeout", "leader", d.status.getLeaderIndex())
	d.changeView()
	drainStopTimer(d.leaderTimer)
	d.leaderTimer.Reset(d.config.LeaderTimeout)
}

func (d *driver) changeView() {
	d.status.setViewChange(1)
	d.sleepTime(d.config.DeltaTime)
	if err := d.resources.MsgSvc.BroadcastQC(d.status.getQCHigh()); err != nil {
		logger.I().Errorf("broadcast qc failed, %+v", err)
	}
	d.sleepTime(d.config.DeltaTime * 2)
	if d.status.getViewChange() == 1 {
		d.status.setViewStart()
		d.status.setView(d.status.getView() + 1)
		leaderIdx := d.status.getView() % uint32(d.resources.RoleStore.ValidatorCount())
		d.status.setLeaderIndex(leaderIdx)
		logger.I().Infow("view changed",
			"view", d.status.getView(),
			"leader", d.status.getLeaderIndex(),
			"qc", d.qcRefHeight(d.status.getQCHigh()))
		d.newViewProposal()
	}
}

func (d *driver) newViewProposal() {
	d.mtxUpdate.Lock()
	defer d.mtxUpdate.Unlock()

	if !d.isLeader(d.resources.Signer.PublicKey()) {
		return
	}

	qcHigh := d.status.getQCHigh()
	pro := core.NewProposal().
		SetQuorumCert(qcHigh).
		SetView(d.status.getView()).
		Sign(d.resources.Signer)
	blk := d.getBlockByHash(qcHigh.BlockHash())
	d.status.setBLeaf(blk)
	d.status.startProposal(pro, blk)
	d.onNewProposal(pro)
	if err := d.resources.MsgSvc.BroadcastProposal(pro); err != nil {
		logger.I().Errorf("broadcast proposal failed, %+v", err)
	}

	logger.I().Infow("proposed new view proposal",
		"view", pro.View(),
		"qc", d.qcRefHeight(pro.QuorumCert()))
	quota := d.resources.RoleStore.GetValidatorQuota(d.resources.Signer.PublicKey())
	vote := pro.Vote(d.resources.Signer, quota/float64(d.resources.RoleStore.GetWindowSize()))
	d.onReceiveVote(vote)
	d.updateQCHigh(pro.QuorumCert())
}

func (d *driver) onNewProposal(pro *core.Proposal) {
	proposer := uint32(d.resources.RoleStore.GetValidatorIndex(pro.Proposer()))
	var ltreset, vtreset bool
	if d.isNormalApproval(pro.View(), proposer) {
		ltreset = true
		logger.I().Debugw("refresh leader",
			"view", pro.View(),
			"leader", proposer)
	}
	if d.isNewViewApproval(pro.View(), proposer) {
		ltreset = true
		vtreset = true
		d.approveViewLeader(pro.View(), proposer)
	}
	if ltreset {
		drainStopTimer(d.leaderTimer)
		d.leaderTimer.Reset(d.config.LeaderTimeout)
	}
	if vtreset {
		drainStopTimer(d.viewTimer)
		d.viewTimer.Reset(d.config.ViewWidth)
	}
}

func (d *driver) isNormalApproval(view uint32, proposer uint32) bool {
	curView := d.status.getView()
	leaderIdx := d.status.getLeaderIndex()
	pending := d.status.getViewChange()
	return pending == 0 && view == curView && proposer == leaderIdx
}

func (d *driver) isNewViewApproval(view uint32, proposer uint32) bool {
	curView := d.status.getView()
	if view > curView {
		return true
	} else if view == curView {
		leaderIdx := d.status.getLeaderIndex()
		pending := d.status.getViewChange()
		return pending == 0 && proposer != leaderIdx ||
			pending == 1 && proposer == leaderIdx
	}
	return false
}

func (d *driver) approveViewLeader(view uint32, proposer uint32) {
	d.status.setViewChange(0)
	d.status.setView(view)
	d.status.setLeaderIndex(proposer)
	d.status.setViewStart()
	d.leaderTimeoutCount = 0
	logger.I().Infow("approved leader",
		"view", view,
		"leader", proposer)
}

func (d *driver) sleepTime(delta time.Duration) {
	t := time.NewTimer(delta)
	defer t.Stop()
	select {
	case <-d.stopCh:
		return
	case <-t.C:
	}
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

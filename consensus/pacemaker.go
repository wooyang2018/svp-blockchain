// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"time"

	"github.com/wooyang2018/svp-blockchain/logger"
)

type pacemaker struct {
	resources *Resources
	config    Config
	state     *state
	status    *status
	driver    *driver

	checkDelay time.Duration // latency to check tx num in pool
	stopCh     chan struct{}
}

func (pm *pacemaker) start() {
	if pm.stopCh != nil {
		return
	}
	pm.stopCh = make(chan struct{})
	go pm.run()
	if PreserveTxFlag {
		time.Sleep(2 * time.Second)
		pm.driver.proposeCh <- struct{}{}
	}
	logger.I().Info("started pacemaker")
}

func (pm *pacemaker) stop() {
	if pm.stopCh == nil {
		return // not started yet
	}
	select {
	case <-pm.stopCh: // already stopped
		return
	default:
	}
	close(pm.stopCh)
	logger.I().Info("stopped pacemaker")
	pm.stopCh = nil
}

func (pm *pacemaker) run() {
	for {
		select {
		case <-pm.stopCh:
			return
		case <-pm.driver.proposeCh:
			pm.delayProposeWhenNoTxs()
			pm.newProposal()
		}
	}
}

func (pm *pacemaker) newProposal() {
	pm.driver.mtxUpdate.Lock()
	defer pm.driver.mtxUpdate.Unlock()

	if pm.status.getViewChange() != 0 {
		logger.I().Warn("can not create proposal when view change")
		return
	}
	if !pm.driver.isLeader(pm.resources.Signer.PublicKey()) {
		return
	}

	blk := pm.driver.createProposal(pm.status.getView(),
		pm.status.getBLeaf(), pm.status.getQCHigh())
	if err := pm.resources.MsgSvc.BroadcastProposal(blk); err != nil {
		logger.I().Errorf("broadcast proposal failed, %+v", err)
	}

	pm.driver.updateQCHigh(blk.QuorumCert())

	var quota uint64 = 1
	if !TwoPhaseBFTFlag {
		var err error
		if quota, err = pm.status.getVoteQuota(); err != nil {
			return
		}
	}
	vote := blk.Vote(pm.resources.Signer, quota)
	pm.driver.onReceiveVote(vote)
}

func (pm *pacemaker) delayProposeWhenNoTxs() {
	timer := time.NewTimer(pm.config.TxWaitTime)
	defer timer.Stop()
	for pm.resources.TxPool.GetStatus().Total == 0 {
		select {
		case <-timer.C:
			return
		case <-time.After(pm.checkDelay):
		}
	}
}

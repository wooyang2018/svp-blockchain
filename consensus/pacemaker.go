// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"time"

	"github.com/wooyang2018/posv-blockchain/logger"
)

type pacemaker struct {
	resources *Resources
	state     *state
	driver    *driver
	stopCh    chan struct{}
}

func (pm *pacemaker) start() {
	if pm.stopCh != nil {
		return
	}
	pm.stopCh = make(chan struct{})
	go pm.run()
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
	qc := pm.driver.SubscribeQC()
	defer qc.Unsubscribe()
	time.Sleep(time.Second)
	pm.newProposal()
	for {
		select {
		case <-pm.stopCh:
			return
		case <-qc.Events():
			pm.newProposal()
		}
	}
}

func (pm *pacemaker) newProposal() {
	pm.driver.mtxUpdate.Lock()
	defer pm.driver.mtxUpdate.Unlock()

	if !pm.driver.isLeader(pm.resources.Signer.PublicKey()) {
		return
	}

	pro := pm.driver.OnPropose()
	logger.I().Debugw("proposed proposal", "view", pro.View(),
		"height", pro.Block().Height(), "qc", pm.driver.qcRefHeight(pro.QuorumCert()), "txs", len(pro.Block().Transactions()))
	vote := pro.Vote(pm.resources.Signer)
	pm.driver.OnReceiveVote(vote)
	pm.driver.UpdateQCHigh(pro.QuorumCert())
}

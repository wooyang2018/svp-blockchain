// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"github.com/wooyang2018/posv-blockchain/logger"
)

type pacemaker struct {
	resources *Resources
	config    Config

	state *state
	posv  *posv

	stopCh chan struct{}
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
	subQC := pm.posv.SubscribeNewQCHigh()
	defer subQC.Unsubscribe()

	for {
		pm.newBlock()

		select {
		case <-pm.stopCh:
			return
		case <-subQC.Events():
		}
	}
}

func (pm *pacemaker) newBlock() {
	pm.state.mtxUpdate.Lock()
	defer pm.state.mtxUpdate.Unlock()

	select {
	case <-pm.stopCh:
		return
	default:
	}

	if !pm.state.isThisNodeLeader() {
		return
	}

	blk := pm.posv.OnPropose()
	logger.I().Debugw("proposed block", "height", blk.Height(), "qc", qcRefHeight(blk.Justify()), "txs", len(blk.Transactions()))
	vote := blk.(*innerBlock).block.Vote(pm.resources.Signer)
	pm.posv.OnReceiveVote(newVote(vote, pm.state))
	pm.posv.Update(blk)
}

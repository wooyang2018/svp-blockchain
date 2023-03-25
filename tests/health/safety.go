// Copyright (C) 2021 Aung Maw
// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package health

import (
	"fmt"
	"time"

	"github.com/wooyang2018/ppov-blockchain/consensus"
)

func (hc *checker) checkLiveness() error {
	status, err := hc.shouldGetStatus()
	if err != nil {
		return err
	}
	lastHeight := hc.getMaximumBexec(status)
	time.Sleep(hc.getLivenessWaitTime())
	select {
	case <-hc.interrupt:
		return nil
	default:
	}
	prevStatus := status
	status, err = hc.shouldGetStatus()
	if err != nil {
		return err
	}
	if err := hc.shouldCommitNewBlocks(status, lastHeight); err != nil {
		return err
	}
	return hc.shouldCommitTxs(prevStatus, status)
}

func (hc *checker) getMaximumBexec(status map[int]*consensus.Status) uint64 {
	var bexec uint64 = 0
	for _, s := range status {
		if s.BExec > bexec {
			bexec = s.BExec
		}
	}
	return bexec
}

func (hc *checker) getLivenessWaitTime() time.Duration {
	d := 20 * time.Second
	if hc.majority {
		d += time.Duration(hc.getFaultyCount()) * hc.LeaderTimeout()
	}
	return d
}

func (hc *checker) shouldCommitNewBlocks(
	sMap map[int]*consensus.Status, lastHeight uint64,
) error {
	validCount := 0
	blkCount := 0
	for _, status := range sMap {
		if status.BExec > lastHeight {
			if blkCount == 0 {
				blkCount = int(status.BExec - lastHeight)
			}
			validCount++
		}
	}
	if validCount < hc.minimumHealthyNode() {
		return fmt.Errorf("%d nodes are not committing new blocks",
			hc.cluster.NodeCount()-validCount)
	}
	fmt.Printf(" + Committed blocks in %s = %d\n", hc.getLivenessWaitTime(), blkCount)
	return nil
}

func (hc *checker) shouldCommitTxs(
	prevStatus, status map[int]*consensus.Status,
) error {
	validCount := 0
	txCount := 0
	for i, s := range status {
		if prevStatus == nil && s.CommittedTxCount > 0 {
			validCount++
		} else if s.CommittedTxCount > prevStatus[i].CommittedTxCount {
			if txCount == 0 {
				txCount = s.CommittedTxCount - prevStatus[i].CommittedTxCount
			}
			validCount++
		}
	}
	if validCount < hc.minimumHealthyNode() {
		return fmt.Errorf("%d nodes are not committing new txs",
			hc.cluster.NodeCount()-validCount)
	}
	fmt.Printf(" + Committed txs in %s = %d\n", hc.getLivenessWaitTime(), txCount)
	return nil
}

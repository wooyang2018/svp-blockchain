// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package health

import (
	"errors"
	"fmt"
	"time"

	"github.com/wooyang2018/posv-blockchain/consensus"
)

func (hc *checker) checkRotation() error {
	timeout := time.NewTimer(hc.getRotationTimeout())
	defer timeout.Stop()

	lastView := make(map[int]*consensus.Status)
	changedView := make(map[int]*consensus.Status)
	for {
		var err error
		if err = hc.updateViewChangeStatus(lastView, changedView); err != nil {
			return err
		}
		if len(changedView) >= len(lastView) {
			if err = hc.shouldEqualLeader(changedView); err == nil {
				return nil
			}
		}
		select {
		case <-hc.interrupt:
			return nil

		case <-timeout.C:
			return errors.New("cluster failed to rotate leader")

		case <-time.After(2 * time.Second):
		}
	}
}

func (hc *checker) getRotationTimeout() time.Duration {
	d := hc.ViewWidth() + 20*time.Second
	if hc.majority {
		d += time.Duration(hc.getFaultyCount()) * hc.LeaderTimeout()
	}
	return d
}

func (hc *checker) updateViewChangeStatus(last, changed map[int]*consensus.Status) error {
	sResp, err := hc.shouldGetStatus()
	if err != nil {
		return err
	}
	for i, status := range sResp {
		if hc.hasViewChanged(status, last[i]) {
			changed[i] = status
			last[i] = status
		}
		if last[i] == nil {
			last[i] = status // first time
		}
	}
	return nil
}

func (hc *checker) hasViewChanged(status, last *consensus.Status) bool {
	if last == nil {
		return false // last view is not loaded yet for the first time
	}
	if status.ViewChange == 1 {
		return false // view change is pending but not confirmed yet
	}
	if last.ViewChange == 1 { // previously pending, now not pending
		return true // it's confirmed view change
	}
	return status.LeaderIndex != last.LeaderIndex
}

func (hc *checker) shouldEqualLeader(changedView map[int]*consensus.Status) error {
	equalCount := make(map[uint32]int)
	for _, status := range changedView {
		equalCount[status.LeaderIndex]++
	}
	for i, count := range equalCount {
		if count >= hc.minimumHealthyNode() {
			fmt.Printf(" + Leader changed to %d\n", i)
			return nil
		}
	}
	return fmt.Errorf("View status map[leader]count: %v\n", equalCount)
}

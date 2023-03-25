// Copyright (C) 2021 Aung Maw
// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package experiments

import (
	"fmt"
	"time"

	"github.com/wooyang2018/ppov-blockchain/core"
	"github.com/wooyang2018/ppov-blockchain/tests/cluster"
	"github.com/wooyang2018/ppov-blockchain/tests/health"
	"github.com/wooyang2018/ppov-blockchain/tests/testutil"
)

type MajorityKeepRunning struct{}

func (expm *MajorityKeepRunning) Name() string {
	return "majority_keep_running"
}

// Keep majority (2f+1) validators running while stopping the rest
// The blockchain should keep remain healthy
// When the stopped nodes up again, they should sync the history
func (expm *MajorityKeepRunning) Run(cls *cluster.Cluster) error {
	total := cls.NodeCount()
	faulty := testutil.PickUniqueRandoms(total, total-core.MajorityCount(total))
	for _, i := range faulty {
		cls.GetNode(i).Stop()
	}
	fmt.Printf("Stopped %d out of %d nodes: %v\n", len(faulty), total, faulty)

	testutil.Sleep(10 * time.Second)
	if err := health.CheckMajorityNodes(cls); err != nil {
		return err
	}
	for _, fi := range faulty {
		if err := cls.GetNode(fi).Start(); err != nil {
			return err
		}
	}
	fmt.Printf("Started nodes: %v\n", faulty)
	// stopped nodes should sync with the majority after some duration
	testutil.Sleep(30 * time.Second)
	return nil
}

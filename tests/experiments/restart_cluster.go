// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package experiments

import (
	"fmt"
	"time"

	"github.com/wooyang2018/svp-blockchain/tests/cluster"
	"github.com/wooyang2018/svp-blockchain/tests/testutil"
)

type RestartCluster struct{}

func (expm *RestartCluster) Name() string {
	return "restart_cluster"
}

func (expm *RestartCluster) Run(cls *cluster.Cluster) error {
	//make sure no pending txs in pool
	testutil.LoadGen.Pause()
	testutil.Sleep(20 * time.Second)

	cls.Stop()
	fmt.Println("Stopped cluster")
	testutil.Sleep(10 * time.Second)

	testutil.LoadGen.UnPause()
	if err := cls.Start(); err != nil {
		return err
	}
	fmt.Println("Restarted cluster")
	testutil.Sleep(40 * time.Second)
	return nil
}

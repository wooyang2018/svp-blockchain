// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package testutil

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/tests/cluster"
)

type LoadClient interface {
	SetupOnCluster(cls *cluster.Cluster) error
	SubmitTx() (int, *core.Transaction, error)
	BatchSubmitTx(num int) (int, *core.TxList, error)
	SubmitTxAndWait() (int, error)
}

// Sleep print duration and call time.Sleep
func Sleep(d time.Duration) {
	fmt.Printf("Wait for %s\n", d)
	time.Sleep(d)
}

func PickUniqueRandoms(total, count int) []int {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	unique := make(map[int]struct{}, count)
	for len(unique) < count {
		unique[r.Intn(total)] = struct{}{}
	}
	ret := make([]int, 0, count)
	for v := range unique {
		ret = append(ret, v)
	}
	return ret
}

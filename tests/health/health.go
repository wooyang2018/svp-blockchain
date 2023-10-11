// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package health

import (
	"fmt"
	"sync"
	"time"

	"github.com/wooyang2018/svp-blockchain/consensus"
	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/tests/cluster"
	"github.com/wooyang2018/svp-blockchain/tests/testutil"
)

func CheckAllNodes(cls *cluster.Cluster) error {
	fmt.Println("Health check all nodes")
	hc := &checker{
		cluster:  cls,
		majority: false,
	}
	return hc.run()
}

func CheckMajorityNodes(cls *cluster.Cluster) error {
	fmt.Println("Health check majority nodes")
	hc := &checker{
		cluster:  cls,
		majority: true,
	}
	return hc.run()
}

type checker struct {
	cluster   *cluster.Cluster
	majority  bool // should (majority or all) nodes healthy
	interrupt chan struct{}
	mtxInter  sync.Mutex
	err       error
}

func (hc *checker) run() error {
	hc.interrupt = make(chan struct{})
	wg := new(sync.WaitGroup)
	wg.Add(2)
	go hc.runChecker(hc.checkSafety, wg)
	go hc.runChecker(hc.checkLiveness, wg)
	if hc.cluster.CheckRotation {
		wg.Add(1)
		go hc.runChecker(hc.checkRotation, wg)
	}
	wg.Wait()
	return hc.err
}

func (hc *checker) runChecker(checker func() error, wg *sync.WaitGroup) {
	defer wg.Done()
	if err := checker(); err != nil {
		hc.err = err
		hc.mtxInter.Lock()
		defer hc.mtxInter.Unlock()
		select {
		case <-hc.interrupt:
			return
		default:
		}
		close(hc.interrupt)
	}
}

func (hc *checker) shouldGetStatus() (map[int]*consensus.Status, error) {
	ret := testutil.GetStatusAll(hc.cluster)
	min := hc.minimumHealthyNode()
	if len(ret) < min {
		return nil, fmt.Errorf("failed to get status from %d nodes", min-len(ret))
	}
	return ret, nil
}

func (hc *checker) minimumHealthyNode() int {
	min := hc.cluster.NodeCount()
	if hc.majority {
		min = core.MajorityCount(hc.cluster.NodeCount())
	}
	return min
}

func (hc *checker) LeaderTimeout() time.Duration {
	config := hc.cluster.NodeConfig()
	return config.ConsensusConfig.LeaderTimeout
}

func (hc *checker) ViewWidth() time.Duration {
	config := hc.cluster.NodeConfig()
	return config.ConsensusConfig.ViewWidth
}

func (hc *checker) getFaultyCount() int {
	return hc.cluster.NodeCount() - core.MajorityCount(hc.cluster.NodeCount())
}

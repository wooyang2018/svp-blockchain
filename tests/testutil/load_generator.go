// Copyright (C) 2021 Aung Maw
// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package testutil

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/wooyang2018/ppov-blockchain/consensus"
	"github.com/wooyang2018/ppov-blockchain/tests/cluster"
)

type LoadGenerator struct {
	txPerSec int
	client   LoadClient

	totalSubmitted int64
}

func NewLoadGenerator(tps int, client LoadClient) *LoadGenerator {
	return &LoadGenerator{
		txPerSec: tps,
		client:   client,
	}
}

func (lg *LoadGenerator) SetupOnCluster(cls *cluster.Cluster) error {
	return lg.client.SetupOnCluster(cls)
}

func (lg *LoadGenerator) Run(ctx context.Context) {
	jobPerTick := 20
	delay := time.Second / time.Duration(lg.txPerSec/jobPerTick)
	ticker := time.NewTicker(delay)
	defer ticker.Stop()

	jobCh := make(chan struct{}, lg.txPerSec)
	defer close(jobCh)

	for i := 0; i < lg.txPerSec; i++ {
		go lg.loadWorker(jobCh)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for i := 0; i < jobPerTick; i++ {
				jobCh <- struct{}{}
			}
		}
	}
}

func (lg *LoadGenerator) BatchRun(ctx context.Context) {
	jobPerTick := 30000
	delay := time.Second / time.Duration(lg.txPerSec/jobPerTick)
	ticker := time.NewTicker(delay)
	defer ticker.Stop()
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, txs, err := lg.client.BatchSubmitTx(jobPerTick); err == nil {
				lg.increaseSubmitted(int64(len(*txs)))
			}
		case <-timer.C:
			if !consensus.ExecuteTxFlag {
				return
			}
		}
	}
}

func (lg *LoadGenerator) loadWorker(jobs <-chan struct{}) {
	for range jobs {
		if _, _, err := lg.client.SubmitTx(); err == nil {
			lg.increaseSubmitted(1)
		}
	}
}

func (lg *LoadGenerator) increaseSubmitted(delta int64) {
	atomic.AddInt64(&lg.totalSubmitted, delta)
}

func (lg *LoadGenerator) ResetTotalSubmitted() int {
	return int(atomic.SwapInt64(&lg.totalSubmitted, 0))
}

func (lg *LoadGenerator) GetTxPerSec() int {
	return lg.txPerSec
}

func (lg *LoadGenerator) GetClient() LoadClient {
	return lg.client
}

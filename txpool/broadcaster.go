// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package txpool

import (
	"time"

	"github.com/wooyang2018/svp-blockchain/core"
)

type broadcaster struct {
	msgSvc MsgService

	queue     chan *core.Transaction
	txBatch   []*core.Transaction
	batchSize int

	timeout time.Duration
	timer   *time.Timer
}

func newBroadcaster(msgSvc MsgService) *broadcaster {
	b := &broadcaster{
		msgSvc:    msgSvc,
		queue:     make(chan *core.Transaction, 5000),
		batchSize: 1000,
		timeout:   5 * time.Millisecond,
	}
	b.txBatch = make([]*core.Transaction, 0, b.batchSize)
	b.timer = time.NewTimer(b.timeout)

	return b
}

func (b *broadcaster) run() {
	for {
		select {
		case <-b.timer.C:
			if len(b.txBatch) > 0 {
				b.broadcastBatch()
			}
			b.timer.Reset(b.timeout)

		case tx := <-b.queue:
			b.txBatch = append(b.txBatch, tx)
			if len(b.txBatch) >= b.batchSize {
				b.broadcastBatch()
			}
		}
	}
}

func (b *broadcaster) broadcastBatch() {
	b.msgSvc.BroadcastTxList((*core.TxList)(&b.txBatch))
	b.txBatch = make([]*core.Transaction, 0, b.batchSize)
}

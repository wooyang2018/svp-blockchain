// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"encoding/csv"
	"os"
	"strconv"
	"time"
)

type tester struct {
	writer  *csv.Writer
	preTime int64
}

func newTester(file *os.File) *tester {
	t := &tester{}
	if file != nil {
		t.writer = csv.NewWriter(file)
		t.writer.Write([]string{
			"Height",
			"StartTime",
			"CommitTime",
			"EndTime",
			"Latency",
			"TxCount",
			"System Throughput",
			"Consensus Throughput",
		})
	}
	return t
}

func (t *tester) close() {
	t.writer.Flush()
}

func (t *tester) saveItem(height uint64, t0, t1, t2 int64, txs int) {
	if t.writer == nil {
		return
	}
	if t.preTime == 0 {
		t.preTime = t0
	}

	sysElapsed := t2 - t.preTime
	consElapsed := t1 - t.preTime
	sysTPS := float64(txs) * 1e9 / float64(sysElapsed)
	consTPS := float64(txs) * 1e9 / float64(consElapsed)
	t.preTime = t2

	t.writer.Write([]string{
		strconv.FormatUint(height, 10),
		strconv.FormatInt(t0, 10),
		strconv.FormatInt(t1, 10),
		strconv.FormatInt(t2, 10),
		strconv.FormatInt(time.Duration(t1-t0).Milliseconds(), 10),
		strconv.Itoa(txs),
		strconv.FormatFloat(sysTPS, 'f', 2, 64),
		strconv.FormatFloat(consTPS, 'f', 2, 64),
	})
}

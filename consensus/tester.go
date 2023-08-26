// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"encoding/csv"
	"os"
	"strconv"
	"time"

	"github.com/wooyang2018/svp-blockchain/logger"
)

type tester struct {
	writer      *csv.Writer
	preTime     int64
	sysElapsed  int64
	consElapsed int64
	txCount     int
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

func (t *tester) saveItem(height uint64, t0, t1, t2 int64, txs int) {
	if t.writer == nil {
		return
	}
	if t.preTime == 0 {
		t.preTime = t0
	}

	t.sysElapsed = t.sysElapsed + t2 - t.preTime
	t.consElapsed = t.consElapsed + t1 - t.preTime
	t.txCount = t.txCount + txs
	t.preTime = t2
	content := []string{
		strconv.FormatUint(height, 10),
		strconv.FormatInt(t0, 10),
		strconv.FormatInt(t1, 10),
		strconv.FormatInt(t2, 10),
		strconv.FormatInt(time.Duration(t1-t0).Milliseconds(), 10),
		strconv.Itoa(txs),
	}

	if height > 0 && height%10 == 0 {
		t.writer.Flush()
		sysTPS := float64(t.txCount) * 1e9 / float64(t.sysElapsed)
		sysStr := strconv.FormatFloat(sysTPS, 'f', 2, 64)
		consTPS := float64(t.txCount) * 1e9 / float64(t.consElapsed)
		consStr := strconv.FormatFloat(consTPS, 'f', 2, 64)
		logger.I().Debugw("benchmark test",
			"height", height,
			"txs", t.txCount,
			"elapsed", time.Duration(t.sysElapsed),
			"tps", sysStr)
		content = append(append(content, sysStr), consStr)
		t.txCount = 0
		t.sysElapsed = 0
		t.consElapsed = 0
	}

	t.writer.Write(content)
}

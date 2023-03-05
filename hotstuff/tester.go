package hotstuff

import (
	"encoding/csv"
	"os"
	"strconv"
	"time"

	"github.com/wooyang2018/ppov-blockchain/logger"
)

type tester struct {
	writer  *csv.Writer
	pretime int64
	elapsed int64
	txCount int
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
			"TxCount",
		})
	}
	return t
}

func (t *tester) saveItem(height uint64, t0, t1, t2 int64, txs int) {
	if t.writer == nil {
		return
	}
	if t.pretime == 0 {
		t.pretime = t1
	}
	t.elapsed = t.elapsed + t1 - t.pretime
	t.txCount = t.txCount + txs
	t.pretime = t2
	t.writer.Write([]string{
		strconv.FormatUint(height, 10),
		strconv.FormatInt(t0, 10),
		strconv.FormatInt(t1, 10),
		strconv.FormatInt(t2, 10),
		strconv.Itoa(txs),
	})
	if height > 0 && height%200 == 0 {
		t.writer.Flush()
		tps := float32(t.txCount) / float32(t.elapsed) * 1e9
		logger.I().Debugw("benchmark test", "height", height, "elapsed", time.Duration(t.elapsed), "txs", t.txCount, "tps", tps)
		t.txCount = 0
		t.elapsed = 0
	}
}

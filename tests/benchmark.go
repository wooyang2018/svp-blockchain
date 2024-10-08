// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/wooyang2018/svp-blockchain/consensus"
	"github.com/wooyang2018/svp-blockchain/tests/cluster"
	"github.com/wooyang2018/svp-blockchain/tests/health"
	"github.com/wooyang2018/svp-blockchain/tests/testutil"
	"github.com/wooyang2018/svp-blockchain/txpool"
)

type Measurement struct {
	Timestamp   int64
	TxSubmitted int
	TxCommitted int

	Load       float32 // actual tx sent per sec
	Throughput float32 // tx committed per sec
	Latency    time.Duration

	ConsensusStatus map[int]*consensus.Status
	TxPoolStatus    map[int]*txpool.Status
}

type Benchmark struct {
	workDir       string
	resultDir     string
	benchmarkName string
	duration      time.Duration
	interval      time.Duration

	cfactory   *cluster.RemoteFactory
	loadClient testutil.LoadClient
	loadGen    *testutil.LoadGenerator
	cluster    *cluster.Cluster
	err        error

	measurements      []*Measurement
	lastTxCommittedN0 int
	lastMeasuredTime  time.Time
}

func (bm *Benchmark) Run() {
	killed := make(chan os.Signal, 1)
	signal.Notify(killed, os.Interrupt, syscall.SIGTERM)
	for _, tps := range BenchLoads {
		if err := bm.runWithLoad(tps); err != nil {
			fmt.Println(err)
			return
		}
		select {
		case s := <-killed:
			fmt.Println("\nGot signal:", s)
			return
		default:
			time.Sleep(10 * time.Second)
		}
	}
}

func (bm *Benchmark) runWithLoad(tps int) error {
	bm.loadGen = testutil.NewLoadGenerator(bm.loadClient, tps, LoadJobPerTick)
	bm.benchmarkName = fmt.Sprintf("bench_n_%d_w_%d_load_%d",
		bm.cfactory.GetParams().NodeCount, bm.cfactory.GetParams().WindowSize, tps)
	if !EmptyChainCode && EVMChainCode {
		bm.benchmarkName += "_evm"
	} else if !EmptyChainCode && PCoinBinCC {
		bm.benchmarkName += "_bincc"
	}
	fmt.Printf("Running benchmark %s\n", bm.benchmarkName)

	bm.resultDir = path.Join(bm.workDir, bm.benchmarkName)
	os.Mkdir(bm.workDir, 0755)
	os.Mkdir(bm.resultDir, 0755)
	bm.measurements = make([]*Measurement, 0)

	done := make(chan struct{})
	loadCtx, stopLoad := context.WithCancel(context.Background())
	go bm.runAsync(loadCtx, done)

	killed := make(chan os.Signal, 1)
	signal.Notify(killed, os.Interrupt, syscall.SIGTERM)
	select {
	case s := <-killed:
		fmt.Println("\nGot signal:", s)
		bm.err = errors.New("interrupted")
	case <-done:
	}

	stopLoad()
	if bm.cluster != nil {
		fmt.Println("Stopping cluster")
		bm.cluster.Stop()
		fmt.Println("Stopped cluster")

		bm.saveResults()
		if RemoteRunRequired {
			bm.stopDstat()
			fmt.Println("Downloaded dstat records")
		}
		bm.downloadFiles()
		bm.removeDB()
		fmt.Print("Removed DB, Done\n\n")
	}
	return bm.err
}

func (bm *Benchmark) runAsync(loadCtx context.Context, done chan struct{}) {
	defer close(done)

	fmt.Println("Setting up a new cluster")
	bm.cluster, bm.err = bm.cfactory.SetupCluster(bm.benchmarkName)
	if bm.err != nil {
		return
	}
	bm.cluster.EmptyChainCode = EmptyChainCode
	bm.cluster.CheckRotation = CheckRotation

	if RemoteRunRequired {
		bm.startDstat()
	}

	fmt.Println("Starting cluster")
	if bm.err = bm.cluster.Start(); bm.err != nil {
		return
	}
	fmt.Println("Started cluster")
	testutil.Sleep(20 * time.Second)

	fmt.Println("Setting up load generator")
	if bm.err = bm.loadGen.SetupOnCluster(bm.cluster); bm.err != nil {
		return
	}
	if LoadBatchSubmit {
		go bm.loadGen.BatchRun(loadCtx)
	} else {
		go bm.loadGen.Run(loadCtx)
	}
	testutil.Sleep(20 * time.Second)

	if bm.err = health.CheckAllNodes(bm.cluster); bm.err != nil {
		fmt.Printf("health check failed before benchmark, %+v\n", bm.err)
		bm.cluster.Stop()
		return
	}

	if bm.err = bm.measure(); bm.err != nil {
		return
	}
}

func (bm *Benchmark) startDstat() {
	var wg sync.WaitGroup
	for i := 0; i < bm.cluster.NodeCount(); i++ {
		node := bm.cluster.GetNode(i).(*cluster.RemoteNode)
		wg.Add(1)
		go func() {
			defer wg.Done()
			node.StartDstat()
		}()
	}
	wg.Wait()
}

func (bm *Benchmark) stopDstat() {
	var wg sync.WaitGroup
	for i := 0; i < bm.cluster.NodeCount(); i++ {
		node := bm.cluster.GetNode(i).(*cluster.RemoteNode)
		wg.Add(1)
		go func() {
			defer wg.Done()
			node.StopDstat()
		}()
	}
	wg.Wait()
}

func (bm *Benchmark) downloadFiles() {
	var wg sync.WaitGroup
	for i := 0; i < bm.cluster.NodeCount(); i++ {
		if i >= 4 && i < bm.cluster.NodeCount()-1 {
			continue
		}
		node := bm.cluster.GetNode(i).(*cluster.RemoteNode)
		wg.Add(2)

		filePath1 := path.Join(bm.resultDir, fmt.Sprintf("consensus_%d.csv", i))
		go func() {
			defer wg.Done()
			node.DownloadFile(filePath1, "consensus.csv")
		}()

		filePath2 := path.Join(bm.resultDir, fmt.Sprintf("log_%d.txt", i))
		go func() {
			defer wg.Done()
			node.DownloadFile(filePath2, "log.txt")
		}()

		if RemoteRunRequired {
			wg.Add(1)
			filePath3 := path.Join(bm.resultDir, fmt.Sprintf("dstat_%d.txt", i))
			go func() {
				defer wg.Done()
				node.DownloadFile(filePath3, "dstat.txt")
			}()
		}
	}
	wg.Wait()
}

func (bm *Benchmark) removeDB() {
	var wg sync.WaitGroup
	for i := 0; i < bm.cluster.NodeCount(); i++ {
		node := bm.cluster.GetNode(i).(*cluster.RemoteNode)
		wg.Add(1)
		go func() {
			defer wg.Done()
			node.RemoveDB()
		}()
	}
	wg.Wait()
}

func (bm *Benchmark) measure() error {
	timer := time.NewTimer(bm.duration)
	defer timer.Stop()

	bm.loadGen.ResetTotalSubmitted()
	consStatus := testutil.GetStatusAll(bm.cluster)
	bm.lastTxCommittedN0 = consStatus[0].CommittedTxCount
	bm.lastMeasuredTime = time.Now()
	fmt.Printf("\nStarted performance measurements\n")

	ticker := time.NewTicker(bm.interval)
	defer ticker.Stop()
	for range ticker.C {
		if err := bm.onTick(); err != nil {
			return err
		}
		select {
		case <-timer.C:
			return nil
		default:
		}
	}
	return nil
}

func (bm *Benchmark) onTick() error {
	meas := new(Measurement)
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		meas.ConsensusStatus = testutil.GetStatusAll(bm.cluster)
	}()
	go func() {
		defer wg.Done()
		meas.TxPoolStatus = testutil.GetTxPoolStatusAll(bm.cluster)
		if len(meas.TxPoolStatus) >= 1 && meas.TxPoolStatus[0] != nil {
			if meas.TxPoolStatus[0].Queue >= 30000 {
				bm.loadGen.Pause()
			} else if meas.TxPoolStatus[0].Queue <= 10000 {
				bm.loadGen.UnPause()
			}
		}
	}()
	if consensus.ExecuteTxFlag {
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			bm.loadGen.GetClient().SubmitTxAndWait()
			meas.Latency = time.Since(start)
		}()
	}
	wg.Wait()

	meas.Timestamp = time.Now().Unix()
	meas.TxSubmitted = bm.loadGen.ResetTotalSubmitted()
	if len(meas.ConsensusStatus) < bm.cluster.NodeCount() {
		return fmt.Errorf("failed to get consensus status from %d nodes",
			bm.cluster.NodeCount()-len(meas.ConsensusStatus))
	}
	if len(meas.TxPoolStatus) < bm.cluster.NodeCount() {
		return fmt.Errorf("failed to get txpool status from %d nodes",
			bm.cluster.NodeCount()-len(meas.TxPoolStatus))
	}
	elapsed := time.Since(bm.lastMeasuredTime)
	bm.lastMeasuredTime = time.Now()
	meas.TxCommitted = meas.ConsensusStatus[0].CommittedTxCount - bm.lastTxCommittedN0
	bm.lastTxCommittedN0 = meas.ConsensusStatus[0].CommittedTxCount
	meas.Throughput = float32(meas.TxCommitted) / float32(elapsed.Seconds())
	meas.Load = float32(meas.TxSubmitted) / float32(elapsed.Seconds())
	bm.measurements = append(bm.measurements, meas)

	log.Printf("  Load: %6.1f  |  Throughput: %6.1f  |  Latency: %s\n",
		meas.Load, meas.Throughput, meas.Latency.String())
	return nil
}

func (bm *Benchmark) saveResults() error {
	if len(bm.measurements) == 0 {
		return errors.New("no measurements to save")
	}
	fmt.Println("\nSaving results")
	if err := bm.savePerformance(); err != nil {
		return err
	}
	for i := 0; i < bm.cluster.NodeCount(); i++ {
		if i < 4 || i == bm.cluster.NodeCount()-1 {
			if err := bm.saveStatusOneNode(i); err != nil {
				return err
			}
		}
	}
	fmt.Printf("\nSaved Results in %s\n", bm.resultDir)
	return nil
}

func (bm *Benchmark) savePerformance() error {
	f, err := os.Create(path.Join(bm.resultDir, "performance.csv"))
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	w.Write([]string{
		"Timestamp",
		"TxSubmitted",
		"TxCommitted",
		"Load",
		"Throughput",
		"Latency",
	})
	var loadTotal, tpTotal float32
	var ltTotal time.Duration
	for _, m := range bm.measurements {
		loadTotal += m.Load
		tpTotal += m.Throughput
		ltTotal += m.Latency

		w.Write([]string{
			strconv.Itoa(int(m.Timestamp)),
			strconv.Itoa(int(m.TxSubmitted)),
			strconv.Itoa(int(m.TxCommitted)),
			fmt.Sprintf("%.2f", m.Load),
			fmt.Sprintf("%.2f", m.Throughput),
			m.Latency.String(),
		})
	}

	loadAvg := loadTotal / float32(len(bm.measurements))
	tpAvg := tpTotal / float32(len(bm.measurements))
	ltAvg := ltTotal / time.Duration(len(bm.measurements))
	fmt.Println("\nAverage:")
	log.Printf("  Load: %6.1f  |  Throughput: %6.1f  |  Latency: %s\n",
		loadAvg, tpAvg, ltAvg.String())

	w.Write([]string{
		strconv.Itoa(int(time.Now().Unix())), "", "",
		fmt.Sprintf("%.2f", loadAvg),
		fmt.Sprintf("%.2f", tpAvg),
		ltAvg.String(),
	})
	w.Flush()
	return nil
}

func (bm *Benchmark) saveStatusOneNode(i int) error {
	f, err := os.Create(path.Join(bm.resultDir, fmt.Sprintf("status_%d.csv", i)))
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	w.Write([]string{
		"Timestamp",
		"BlockPoolSize",
		"QCPoolSize",
		"LeaderIndex",
		"ViewStart",
		"QCHigh",
		"TxPoolTotal",
		"TxPoolPending",
		"TxPoolQueue",
	})
	for _, m := range bm.measurements {
		w.Write([]string{
			strconv.Itoa(int(m.Timestamp)),
			strconv.Itoa(m.ConsensusStatus[i].BlockPoolSize),
			strconv.Itoa(m.ConsensusStatus[i].QCPoolSize),
			strconv.Itoa(int(m.ConsensusStatus[i].LeaderIndex)),
			strconv.Itoa(int(m.ConsensusStatus[i].ViewStart)),
			strconv.Itoa(int(m.ConsensusStatus[i].QCHigh)),
			strconv.Itoa(m.TxPoolStatus[i].Total),
			strconv.Itoa(m.TxPoolStatus[i].Pending),
			strconv.Itoa(m.TxPoolStatus[i].Queue),
		})
	}
	w.Flush()
	return nil
}

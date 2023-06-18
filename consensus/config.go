// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import "time"

const ExecuteTxFlag = true   //set to false when benchmark test
const PreserveTxFlag = false //set to true when benchmark test

const (
	FlagDebug   = "debug"
	FlagDataDir = "dataDir"

	FlagPort    = "port"
	FlagAPIPort = "apiPort"

	FlagBroadcastTx = "broadcastTx"

	// storage
	FlagMerkleBranchFactor = "storage-merkleBranchFactor"

	// execution
	FlagTxExecTimeout       = "execution-txExecTimeout"
	FlagExecConcurrentLimit = "execution-concurrentLimit"

	// consensus
	FlagChainID       = "consensus-chainID"
	FlagBlockTxLimit  = "consensus-blockTxLimit"
	FlagViewWidth     = "consensus-viewWidth"
	FlagLeaderTimeout = "consensus-leaderTimeout"
	FlagDelta         = "consensus-delta"
	FlagBenchmarkPath = "consensus-benchmarkPath"
)

type Config struct {
	ChainID int64

	// maximum tx count in a block
	BlockTxLimit int

	// view duration for a leader
	ViewWidth time.Duration

	// leader must create next qc within this duration
	LeaderTimeout time.Duration

	// upper bound of message latency in a synchronous network
	Delta time.Duration

	// path to save the benchmark log of the consensus algorithm (it will not be saved if blank)
	BenchmarkPath string
}

var DefaultConfig = Config{
	BlockTxLimit:  200,
	ViewWidth:     40 * time.Second,
	LeaderTimeout: 15 * time.Second,
	Delta:         3 * time.Second,
	BenchmarkPath: "",
}

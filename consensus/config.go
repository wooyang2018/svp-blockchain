// Copyright (C) 2021 Aung Maw
// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import "time"

const ExecuteTxFlag = true   //set to false when benchmark test
const PreserveTxFlag = false //set to true when benchmark test

type Config struct {
	ChainID int64

	// maximum tx count in a block
	BlockTxLimit int

	// block creation delay if no transactions in the pool
	TxWaitTime time.Duration

	// for leader, delay to propose next block if she cannot create qc")
	BeatTimeout time.Duration

	// minimum delay between each block (i.e, it can define maximum block rate)
	BlockDelay time.Duration

	// view duration for a leader
	ViewWidth time.Duration

	// leader must create next qc within this duration
	LeaderTimeout time.Duration

	// path to save the benchmark log of the consensus algorithm (it will not be saved if blank)
	BenchmarkPath string
}

var DefaultConfig = Config{
	BlockTxLimit:  10000,
	TxWaitTime:    1 * time.Second,
	BeatTimeout:   1000 * time.Millisecond,
	BlockDelay:    100 * time.Millisecond, // maximum block rate = 10 blk per sec
	ViewWidth:     60 * time.Second,
	LeaderTimeout: 20 * time.Second,
	BenchmarkPath: "",
}

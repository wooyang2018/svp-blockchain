// Copyright (C) 2021 Aung Maw
// Licensed under the GNU General Public License v3.0

package consensus

import "time"

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
}

var DefaultConfig = Config{
	BlockTxLimit:  400,
	TxWaitTime:    1 * time.Second,
	BeatTimeout:   1500 * time.Millisecond,
	BlockDelay:    100 * time.Millisecond, // maximum block rate = 10 blk per sec
	ViewWidth:     60 * time.Second,
	LeaderTimeout: 20 * time.Second,
}

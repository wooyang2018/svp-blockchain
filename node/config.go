// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package node

import (
	"github.com/wooyang2018/svp-blockchain/consensus"
	"github.com/wooyang2018/svp-blockchain/execution"
	"github.com/wooyang2018/svp-blockchain/storage"
)

const MaxProcsNum = 8 // set corresponding num of CPUs when benchmark test
const MinGraceHeight = 4

type Config struct {
	Debug       bool
	DataDir     string
	PointPort   int
	TopicPort   int
	APIPort     int
	BroadcastTx bool

	StorageConfig   storage.Config
	ExecutionConfig execution.Config
	ConsensusConfig consensus.Config
}

var DefaultConfig = Config{
	PointPort:       15150,
	TopicPort:       16160,
	APIPort:         9090,
	BroadcastTx:     true,
	StorageConfig:   storage.DefaultConfig,
	ExecutionConfig: execution.DefaultConfig,
	ConsensusConfig: consensus.DefaultConfig,
}

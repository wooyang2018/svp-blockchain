// Copyright (C) 2021 Aung Maw
// Licensed under the GNU General Public License v3.0

package node

import (
	"github.com/wooyang2018/ppov-blockchain/consensus"
	"github.com/wooyang2018/ppov-blockchain/execution"
	"github.com/wooyang2018/ppov-blockchain/storage"
)

type Config struct {
	Debug       bool
	Datadir     string
	Port        int
	APIPort     int
	BroadcastTx bool

	StorageConfig   storage.Config
	ExecutionConfig execution.Config
	ConsensusConfig consensus.Config
}

var DefaultConfig = Config{
	Port:            15150,
	APIPort:         9040,
	BroadcastTx:     true,
	StorageConfig:   storage.DefaultConfig,
	ExecutionConfig: execution.DefaultConfig,
	ConsensusConfig: consensus.DefaultConfig,
}

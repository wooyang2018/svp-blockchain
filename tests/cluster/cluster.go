// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package cluster

import (
	"time"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/node"
)

var Node0DataDir string
var Node0Key *core.PrivateKey

type Node interface {
	Start() error
	Stop()
	EffectDelay(d time.Duration) error
	EffectLoss(percent float64) error
	RemoveEffect()
	IsRunning() bool
	GetEndpoint() string
	PrintCmd() string
	NodeConfig() node.Config
}

type ClusterFactory interface {
	SetupCluster(name string) (*Cluster, error)
	TemplateDir() string
	Bootstrap() error
}

type Cluster struct {
	nodeConfig     node.Config
	nodes          []Node
	CheckRotation  bool
	EmptyChainCode bool
}

func (cls *Cluster) NodeConfig() node.Config {
	return cls.nodeConfig
}

func (cls *Cluster) Start() error {
	for _, node := range cls.nodes {
		if err := node.Start(); err != nil {
			return err
		}
	}
	return nil
}

func (cls *Cluster) Stop() {
	for _, node := range cls.nodes {
		node.Stop()
	}
}

func (cls *Cluster) RemoveEffects() {
	for _, node := range cls.nodes {
		node.RemoveEffect()
	}
}

func (cls *Cluster) NodeCount() int {
	return len(cls.nodes)
}

func (cls *Cluster) GetNode(idx int) Node {
	if idx >= len(cls.nodes) || idx < 0 {
		return nil
	}
	return cls.nodes[idx]
}

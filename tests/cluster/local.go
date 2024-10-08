// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package cluster

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/multiformats/go-multiaddr"

	"github.com/wooyang2018/svp-blockchain/node"
)

type LocalFactoryParams struct {
	BinPath     string
	WorkDir     string
	NodeCount   int
	StakeQuota  int
	WindowSize  int
	SetupDocker bool

	NodeConfig node.Config
}

type LocalFactory struct {
	params      LocalFactoryParams
	templateDir string
}

var _ ClusterFactory = (*LocalFactory)(nil)

func NewLocalFactory(params LocalFactoryParams) *LocalFactory {
	os.Mkdir(params.WorkDir, 0755)
	ftry := &LocalFactory{
		params: params,
	}
	if ftry.params.SetupDocker {
		ftry.templateDir = path.Join(ftry.params.WorkDir, "docker_template")
	} else {
		ftry.templateDir = path.Join(ftry.params.WorkDir, "cluster_template")
	}
	return ftry
}

func (ftry *LocalFactory) TemplateDir() string {
	return ftry.templateDir
}

func (ftry *LocalFactory) Bootstrap() (err error) {
	var pointAddrs, topicAddrs []multiaddr.Multiaddr
	if ftry.params.SetupDocker {
		pointAddrs, topicAddrs, err = ftry.MakeDockerAddrs()
	} else {
		pointAddrs, topicAddrs, err = ftry.MakeLocalAddrs()
	}
	if err != nil {
		return err
	}

	keys := MakeRandomKeys(ftry.params.NodeCount)
	quotas := MakeRandomQuotas(ftry.params.NodeCount, ftry.params.StakeQuota)
	genesis := &node.Genesis{
		Validators:  make([]string, len(keys)),
		StakeQuotas: make([]uint64, len(keys)),
		WindowSize:  ftry.params.WindowSize,
	}
	for i, v := range keys {
		genesis.Validators[i] = v.PublicKey().String()
		genesis.StakeQuotas[i] = quotas[i]
	}
	peers := MakePeers(keys, pointAddrs, topicAddrs)
	return SetupTemplateDir(ftry.templateDir, keys, genesis, peers)
}

func (ftry *LocalFactory) MakeDockerAddrs() ([]multiaddr.Multiaddr, []multiaddr.Multiaddr, error) {
	n := ftry.params.NodeCount
	pointAddrs, topicAddrs := make([]multiaddr.Multiaddr, n), make([]multiaddr.Multiaddr, n)
	for i := 0; i < n; i++ {
		pointAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/dns4/node%d/tcp/%d",
			i, ftry.params.NodeConfig.PointPort))
		topicAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/dns4/node%d/tcp/%d",
			i, ftry.params.NodeConfig.TopicPort))
		if err != nil {
			return nil, nil, err
		}
		pointAddrs[i] = pointAddr
		topicAddrs[i] = topicAddr
	}
	return pointAddrs, topicAddrs, nil
}

func (ftry *LocalFactory) MakeLocalAddrs() ([]multiaddr.Multiaddr, []multiaddr.Multiaddr, error) {
	n := ftry.params.NodeCount
	pointAddrs, topicAddrs := make([]multiaddr.Multiaddr, n), make([]multiaddr.Multiaddr, n)
	for i := 0; i < n; i++ {
		pointAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d",
			ftry.params.NodeConfig.PointPort+i))
		topicAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d",
			ftry.params.NodeConfig.TopicPort+i))
		if err != nil {
			return nil, nil, err
		}
		pointAddrs[i] = pointAddr
		topicAddrs[i] = topicAddr
	}
	return pointAddrs, topicAddrs, nil
}

func (ftry *LocalFactory) SetupCluster(name string) (*Cluster, error) {
	clusterDir := path.Join(ftry.params.WorkDir, name)
	if ftry.templateDir != clusterDir {
		if err := os.RemoveAll(clusterDir); err != nil {
			return nil, err
		}
		if err := exec.Command("cp", "-r", ftry.templateDir, clusterDir).Run(); err != nil {
			return nil, err
		}
	}

	// create localNodes
	nodes := make([]Node, ftry.params.NodeCount)
	for i := 0; i < ftry.params.NodeCount; i++ {
		node := &LocalNode{
			binPath: ftry.params.BinPath,
			config:  ftry.params.NodeConfig,
		}
		if ftry.params.SetupDocker {
			node.config.DataDir = "/app"
			node.binPath = path.Join(node.config.DataDir, "chain")
		} else {
			node.config.PointPort = node.config.PointPort + i
			node.config.TopicPort = node.config.TopicPort + i
			node.config.APIPort = node.config.APIPort + i
			node.config.DataDir = path.Join(clusterDir, strconv.Itoa(i))
		}
		node.config.ConsensusConfig.BenchmarkPath = path.Join(node.config.DataDir, "consensus.csv")
		nodes[i] = node
		if i == 0 {
			Node0DataDir = node.config.DataDir
		}
	}

	return &Cluster{
		nodes:      nodes,
		nodeConfig: ftry.params.NodeConfig,
	}, nil
}

type LocalNode struct {
	binPath string
	config  node.Config

	running bool
	mtxRun  sync.RWMutex

	cmd     *exec.Cmd
	logFile *os.File
}

var _ Node = (*LocalNode)(nil)

func (node *LocalNode) Start() error {
	if node.IsRunning() {
		return nil
	}
	f, err := os.OpenFile(path.Join(node.config.DataDir, "log.txt"),
		os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	node.logFile = f
	node.cmd = exec.Command(node.binPath)
	AddCmdFlags(node.cmd, &node.config)
	node.cmd.Stderr = node.logFile
	node.cmd.Stdout = node.logFile
	node.setRunning(true)
	return node.cmd.Start()
}

func (node *LocalNode) Stop() {
	if !node.IsRunning() {
		return
	}
	node.setRunning(false)
	node.cmd.Process.Signal(syscall.SIGINT)
	node.cmd.Process.Wait()
	node.logFile.Close()
}

func (node *LocalNode) EffectDelay(d time.Duration) error {
	// no network delay for local node
	return nil
}

func (node *LocalNode) EffectLoss(percent float64) error {
	// no network loss for local node
	return nil
}

func (node *LocalNode) RemoveEffect() {
	// no network effects for local node
}

func (node *LocalNode) IsRunning() bool {
	node.mtxRun.RLock()
	defer node.mtxRun.RUnlock()
	return node.running
}

func (node *LocalNode) setRunning(val bool) {
	node.mtxRun.Lock()
	defer node.mtxRun.Unlock()
	node.running = val
}

func (node *LocalNode) GetEndpoint() string {
	return fmt.Sprintf("http://127.0.0.1:%d", node.config.APIPort)
}

func (node *LocalNode) PrintCmd() string {
	cmd := exec.Command(node.binPath)
	AddCmdFlags(cmd, &node.config)
	return cmd.String()
}

func (node *LocalNode) NodeConfig() node.Config {
	return node.config
}

// Copyright (C) 2021 Aung Maw
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

	"github.com/wooyang2018/ppov-blockchain/node"
)

type LocalFactoryParams struct {
	BinPath   string
	WorkDir   string
	NodeCount int

	NodeConfig node.Config
}

type LocalFactory struct {
	params      LocalFactoryParams
	templateDir string
}

var _ ClusterFactory = (*LocalFactory)(nil)

func NewLocalFactory(params LocalFactoryParams) (*LocalFactory, error) {
	os.Mkdir(params.WorkDir, 0755)
	ftry := &LocalFactory{
		params: params,
	}
	if err := ftry.setup(); err != nil {
		return nil, err
	}
	return ftry, nil
}

func (ftry *LocalFactory) setup() error {
	ftry.templateDir = path.Join(ftry.params.WorkDir, "cluster_template")
	addrs, err := ftry.makeAddrs()
	if err != nil {
		return err
	}
	keys := MakeRandomKeys(ftry.params.NodeCount)
	peers := MakePeers(keys, addrs)
	return SetupTemplateDir(ftry.templateDir, keys, peers)
}

func (ftry *LocalFactory) makeAddrs() ([]multiaddr.Multiaddr, error) {
	addrs := make([]multiaddr.Multiaddr, ftry.params.NodeCount)
	for i := range addrs {
		addr, err := multiaddr.NewMultiaddr(
			fmt.Sprintf("/ip4/127.0.0.1/tcp/%d",
				ftry.params.NodeConfig.Port+i))
		if err != nil {
			return nil, err
		}
		addrs[i] = addr
	}
	return addrs, nil
}

func (ftry *LocalFactory) SetupCluster(name string) (*Cluster, error) {
	clusterDir := path.Join(ftry.params.WorkDir, name)
	if ftry.templateDir != clusterDir {
		err := os.RemoveAll(clusterDir) // no error if path not exist
		if err != nil {
			return nil, err
		}
		err = exec.Command("cp", "-r", ftry.templateDir, clusterDir).Run()
		if err != nil {
			return nil, err
		}
	}

	nodes := make([]Node, ftry.params.NodeCount)
	// create localNodes
	for i := 0; i < ftry.params.NodeCount; i++ {
		node := &LocalNode{
			binPath: ftry.params.BinPath,
			config:  ftry.params.NodeConfig,
		}
		node.config.Datadir = path.Join(clusterDir, strconv.Itoa(i))
		node.config.Port = node.config.Port + i
		node.config.APIPort = node.config.APIPort + i
		nodes[i] = node
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
	f, err := os.OpenFile(path.Join(node.config.Datadir, "log.txt"),
		os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	node.logFile = f
	node.cmd = exec.Command(node.binPath)
	AddPPoVFlags(node.cmd, &node.config)
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
	syscall.Kill(node.cmd.Process.Pid, syscall.SIGTERM)
	node.logFile.Close()
}

func (node *LocalNode) EffectDelay(d time.Duration) error {
	// no network delay for local node
	return nil
}

func (node *LocalNode) EffectLoss(percent float32) error {
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
	AddPPoVFlags(cmd, &node.config)
	return cmd.String()
}

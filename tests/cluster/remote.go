// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package cluster

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/multiformats/go-multiaddr"

	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/node"
)

type RemoteFactoryParams struct {
	BinPath    string
	WorkDir    string
	NodeCount  int
	StakeQuota int
	WindowSize int

	NodeConfig node.Config

	KeySSH          string
	HostsPath       string // file path to host ip addresses
	SetupRequired   bool
	InstallRequired bool
}

type RemoteFactory struct {
	params      RemoteFactoryParams
	templateDir string
	hosts       []string
	loginNames  []string
	netDevices  []string
	workDirs    []string
}

var _ ClusterFactory = (*RemoteFactory)(nil)

func NewRemoteFactory(params RemoteFactoryParams) *RemoteFactory {
	os.Mkdir(params.WorkDir, 0755)
	ftry := &RemoteFactory{
		params: params,
	}
	ftry.templateDir = path.Join(ftry.params.WorkDir, "cluster_template")
	err := ftry.ReadHosts(ftry.params.HostsPath, ftry.params.NodeCount)
	common.Check2(err)
	return ftry
}

func (ftry *RemoteFactory) TemplateDir() string {
	return ftry.templateDir
}

func (ftry *RemoteFactory) ReadHosts(hostsPath string, nodeCount int) error {
	raw, err := os.ReadFile(hostsPath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(raw), "\n")
	if len(lines) < nodeCount {
		return fmt.Errorf("not enough hosts, expected %d, got %d", nodeCount, len(lines))
	}

	hosts := make([]string, nodeCount)
	loginNames := make([]string, nodeCount)
	netDevices := make([]string, nodeCount)
	workDirs := make([]string, nodeCount)
	for i := 0; i < nodeCount; i++ {
		words := strings.Split(lines[i], "\t")
		hosts[i] = words[0]
		loginNames[i] = words[1]
		netDevices[i] = words[2]
		workDirs[i] = words[3]
	}

	ftry.hosts = hosts
	ftry.loginNames = loginNames
	ftry.netDevices = netDevices
	ftry.workDirs = workDirs
	return nil
}

func (ftry *RemoteFactory) GetParams() RemoteFactoryParams {
	return ftry.params
}

func (ftry *RemoteFactory) Bootstrap() error {
	pointAddrs, topicAddrs, err := ftry.makeAddrs()
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

	if err = SetupTemplateDir(ftry.templateDir, keys, genesis, peers); err != nil {
		return err
	}
	if err = ftry.setupRemoteServers(); err != nil {
		return err
	}
	if err = ftry.sendChain(); err != nil {
		return err
	}
	return ftry.sendTemplate()
}

func (ftry *RemoteFactory) makeAddrs() ([]multiaddr.Multiaddr, []multiaddr.Multiaddr, error) {
	n := ftry.params.NodeCount
	pointAddrs, topicAddrs := make([]multiaddr.Multiaddr, n), make([]multiaddr.Multiaddr, n)
	// create validator infos (pubkey + pointAddr + topicAddr)
	for i := 0; i < n; i++ {
		pointAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d",
			ftry.hosts[i], ftry.params.NodeConfig.PointPort+i))
		topicAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d",
			ftry.hosts[i], ftry.params.NodeConfig.TopicPort+i))
		if err != nil {
			return nil, nil, err
		}
		pointAddrs[i] = pointAddr
		topicAddrs[i] = topicAddr
	}
	return pointAddrs, topicAddrs, nil
}

func (ftry *RemoteFactory) setupRemoteServers() error {
	for i := 0; i < ftry.params.NodeCount; i++ {
		if ftry.params.InstallRequired {
			cmd := exec.Command("ssh",
				"-i", ftry.params.KeySSH,
				fmt.Sprintf("%s@%s", ftry.loginNames[i], ftry.hosts[i]),
				"sudo", "apt", "update", ";",
				"sudo", "apt", "install", "-y", "dstat", ";",
			)
			if err := RunCommand(cmd); err != nil {
				return err
			}
		}
		// also kills remaining effect and nodes to make sure clean environment
		cmd := exec.Command("ssh",
			"-i", ftry.params.KeySSH,
			fmt.Sprintf("%s@%s", ftry.loginNames[i], ftry.hosts[i]),
			"sudo", "tc", "qdisc", "del", "dev", ftry.netDevices[i], "root", ";",
			"sudo", "killall", "chain", ";",
			"sudo", "killall", "dstat", ";",
			"mkdir", "-p", ftry.workDirs[i], ";",
			"cd", ftry.workDirs[i], ";",
			"rm", "-rf", "template",
		)
		if err := RunCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (ftry *RemoteFactory) sendChain() error {
	for i := 0; i < ftry.params.NodeCount; i++ {
		cmd := exec.Command("scp",
			"-i", ftry.params.KeySSH,
			ftry.params.BinPath,
			fmt.Sprintf("%s@%s:%s", ftry.loginNames[i], ftry.hosts[i], ftry.workDirs[i]),
		)
		if err := RunCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (ftry *RemoteFactory) sendTemplate() error {
	for i := 0; i < ftry.params.NodeCount; i++ {
		cmd := exec.Command("scp",
			"-i", ftry.params.KeySSH,
			"-r", path.Join(ftry.templateDir, strconv.Itoa(i)),
			fmt.Sprintf("%s@%s:%s", ftry.loginNames[i], ftry.hosts[i],
				path.Join(ftry.workDirs[i], "/template")),
		)
		if err := RunCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func (ftry *RemoteFactory) SetupCluster(name string) (*Cluster, error) {
	if err := ftry.setupClusterDir(name); err != nil {
		return nil, err
	}
	cls := &Cluster{
		nodes:      make([]Node, ftry.params.NodeCount),
		nodeConfig: ftry.params.NodeConfig,
	}
	for i := 0; i < ftry.params.NodeCount; i++ {
		node := &RemoteNode{
			binPath:       path.Join(ftry.workDirs[i], "chain"),
			config:        ftry.params.NodeConfig,
			loginName:     ftry.loginNames[i],
			keySSH:        ftry.params.KeySSH,
			networkDevice: ftry.netDevices[i],
			host:          ftry.hosts[i],
		}
		node.config.DataDir = path.Join(ftry.workDirs[i], name)
		node.config.ConsensusConfig.BenchmarkPath = path.Join(node.config.DataDir, "consensus.csv")
		node.config.PointPort = node.config.PointPort + i
		node.config.TopicPort = node.config.TopicPort + i
		node.config.APIPort = node.config.APIPort + i
		node.RemoveEffect()
		node.StopDstat()
		cls.nodes[i] = node
		if i == 0 {
			Node0DataDir = node.config.DataDir
		}
	}
	cls.Stop()
	time.Sleep(5 * time.Second)
	return cls, nil
}

func (ftry *RemoteFactory) setupClusterDir(name string) error {
	for i := 0; i < ftry.params.NodeCount; i++ {
		cmd := exec.Command("ssh",
			"-i", ftry.params.KeySSH,
			fmt.Sprintf("%s@%s", ftry.loginNames[i], ftry.hosts[i]),
			"cd", ftry.workDirs[i], ";",
			"rm", "-r", name, ";",
			"cp", "-r", "template", name,
		)
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

type RemoteNode struct {
	binPath string
	config  node.Config

	running bool
	mtxRun  sync.RWMutex

	loginName     string
	host          string
	keySSH        string
	networkDevice string
}

var _ Node = (*RemoteNode)(nil)

func (node *RemoteNode) Start() error {
	node.setRunning(true)
	cmd := exec.Command("ssh",
		"-i", node.keySSH,
		fmt.Sprintf("%s@%s", node.loginName, node.host),
		"nohup", node.binPath,
	)
	AddCmdFlags(cmd, &node.config)
	cmd.Args = append(cmd.Args,
		">>", path.Join(node.config.DataDir, "log.txt"), "2>&1", "&",
	)
	return cmd.Run()
}

func (node *RemoteNode) Stop() {
	node.setRunning(false)
	cmd := exec.Command("ssh",
		"-i", node.keySSH,
		fmt.Sprintf("%s@%s", node.loginName, node.host),
		"sudo", "killall", node.binPath,
	)
	cmd.Run()
}

func (node *RemoteNode) EffectDelay(d time.Duration) error {
	cmd := exec.Command("ssh",
		"-i", node.keySSH,
		fmt.Sprintf("%s@%s", node.loginName, node.host),
		"sudo", "tc", "qdisc", "add", "dev", node.networkDevice, "root",
		"netem", "delay", d.String(),
	)
	return cmd.Run()
}

func (node *RemoteNode) EffectLoss(percent float64) error {
	cmd := exec.Command("ssh",
		"-i", node.keySSH,
		fmt.Sprintf("%s@%s", node.loginName, node.host),
		"sudo", "tc", "qdisc", "add", "dev", node.networkDevice, "root",
		"netem", "loss", fmt.Sprintf("%f%%", percent),
	)
	return cmd.Run()
}

func (node *RemoteNode) RemoveEffect() {
	cmd := exec.Command("ssh",
		"-i", node.keySSH,
		fmt.Sprintf("%s@%s", node.loginName, node.host),
		"sudo", "tc", "qdisc", "del", "dev", node.networkDevice, "root",
	)
	cmd.Run()
}

func (node *RemoteNode) InstallDstat() {
	cmd := exec.Command("ssh",
		"-i", node.keySSH,
		fmt.Sprintf("%s@%s", node.loginName, node.host),
		"sudo", "apt", "update", ";",
		"sudo", "apt", "install", "-y", "dstat",
	)
	fmt.Printf("%s\n", strings.Join(cmd.Args, " "))
	cmd.Run()
}

func (node *RemoteNode) StartDstat() {
	cmd := exec.Command("ssh",
		"-i", node.keySSH,
		fmt.Sprintf("%s@%s", node.loginName, node.host),
		"nohup", "dstat", "-Tcmdns",
		">", path.Join(node.config.DataDir, "dstat.txt"), "2>&1", "&",
	)
	cmd.Run()
}

func (node *RemoteNode) StopDstat() {
	cmd := exec.Command("ssh",
		"-i", node.keySSH,
		fmt.Sprintf("%s@%s", node.loginName, node.host),
		"sudo", "killall", "dstat",
	)
	cmd.Run()
}

func (node *RemoteNode) DownloadFile(localPath string, fileName string) {
	cmd := exec.Command("scp",
		"-i", node.keySSH,
		fmt.Sprintf("%s@%s:%s", node.loginName, node.host,
			path.Join(node.config.DataDir, fileName)),
		localPath,
	)
	cmd.Run()
}

func (node *RemoteNode) RemoveDB() {
	cmd := exec.Command("ssh",
		"-i", node.keySSH,
		fmt.Sprintf("%s@%s", node.loginName, node.host),
		"rm", "-rf", path.Join(node.config.DataDir, "db"),
	)
	cmd.Run()
}

func (node *RemoteNode) IsRunning() bool {
	node.mtxRun.RLock()
	defer node.mtxRun.RUnlock()
	return node.running
}

func (node *RemoteNode) setRunning(val bool) {
	node.mtxRun.Lock()
	defer node.mtxRun.Unlock()
	node.running = val
}

func (node *RemoteNode) GetEndpoint() string {
	return fmt.Sprintf("http://%s:%d", node.host, node.config.APIPort)
}

func (node *RemoteNode) PrintCmd() string {
	cmd := exec.Command(node.binPath)
	AddCmdFlags(cmd, &node.config)
	return cmd.String()
}

func (node *RemoteNode) NodeConfig() node.Config {
	return node.config
}

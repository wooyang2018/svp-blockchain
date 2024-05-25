// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package cluster

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/multiformats/go-multiaddr"
	"gopkg.in/yaml.v3"

	"github.com/wooyang2018/svp-blockchain/consensus"
	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/node"
)

type DockerCompose struct {
	Version  string             `yaml:"version"`
	Services map[string]Service `yaml:"services"`
}

type Service struct {
	Image   string   `yaml:"image"`
	Volumes []string `yaml:"volumes"`
	Ports   []string `yaml:"ports"`
	Command string   `yaml:"command"`
}

func NewDockerCompose(cls *Cluster) DockerCompose {
	dockerCompose := DockerCompose{
		Version:  "3",
		Services: make(map[string]Service),
	}
	for i := 0; i < cls.NodeCount(); i++ {
		curNode := cls.GetNode(i)
		service := Service{
			Image:   "ubuntu:20.04",
			Volumes: []string{fmt.Sprintf("../../../chain:%s", path.Join(curNode.NodeConfig().DataDir, "chain"))},
			Ports:   []string{fmt.Sprintf("%d:%d", curNode.NodeConfig().APIPort+i, curNode.NodeConfig().APIPort)},
			Command: curNode.PrintCmd(),
		}
		for _, v := range []string{node.GenesisFile, node.NodekeyFile, node.PeersFile} {
			filepath := path.Join(curNode.NodeConfig().DataDir, v)
			service.Volumes = append(service.Volumes, fmt.Sprintf("./%d/%s:%s", i, v, filepath))
		}
		name := fmt.Sprintf("node%d", i)
		dockerCompose.Services[name] = service
	}
	return dockerCompose
}

func WriteYamlFile(clusterDir string, data DockerCompose) error {
	file, err := os.Create(path.Join(clusterDir, "docker-compose.yaml"))
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)
	return encoder.Encode(data)
}

func WriteNodeKey(datadir string, key *core.PrivateKey) error {
	f, err := os.Create(path.Join(datadir, node.NodekeyFile))
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(key.Bytes())
	return err
}

func WriteGenesisFile(datadir string, genesis *node.Genesis) error {
	f, err := os.Create(path.Join(datadir, node.GenesisFile))
	if err != nil {
		return err
	}
	defer f.Close()
	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(genesis)
}

func WritePeersFile(datadir string, peers []node.Peer) error {
	f, err := os.Create(path.Join(datadir, node.PeersFile))
	if err != nil {
		return err
	}
	defer f.Close()
	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	return e.Encode(peers)
}

func MakeRandomKeys(count int) []*core.PrivateKey {
	keys := make([]*core.PrivateKey, count)
	for i := 0; i < count; i++ {
		keys[i] = core.GenerateKey(nil)
	}
	return keys
}

func MakeRandomQuotas(count int, quota int) []uint64 {
	quotas := make([]uint64, count)
	if consensus.TwoPhaseBFTFlag {
		for i := 0; i < count; i++ {
			quotas[i] = 1
		}
		return quotas
	}
	r := rand.New(rand.NewSource(time.Now().Unix()))
	quotas[count-1] = uint64(quota+1) / 2
	remain := quota - int(quotas[count-1])
	for i := 0; i < remain; i++ {
		quotas[r.Intn(count-1)]++
	}
	sort.SliceStable(quotas, func(i int, j int) bool {
		return quotas[i] < quotas[j]
	})
	return quotas
}

func MakePeers(keys []*core.PrivateKey, pointAddrs, topicAddrs []multiaddr.Multiaddr) []node.Peer {
	n := len(pointAddrs)
	vlds := make([]node.Peer, n)
	// create validator infos (pubkey + pointAddr + topicAddr)
	for i := 0; i < n; i++ {
		vlds[i] = node.Peer{
			PubKey:    keys[i].PublicKey().Bytes(),
			PointAddr: pointAddrs[i].String(),
			TopicAddr: topicAddrs[i].String(),
		}
	}
	return vlds
}

func SetupTemplateDir(dir string, keys []*core.PrivateKey, genesis *node.Genesis, vlds []node.Peer) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.Mkdir(dir, 0755); err != nil {
		return err
	}
	for i, key := range keys {
		d := path.Join(dir, strconv.Itoa(i))
		os.Mkdir(d, 0755)
		if err := WriteNodeKey(d, key); err != nil {
			return err
		}
		if err := WriteGenesisFile(d, genesis); err != nil {
			return err
		}
		if err := WritePeersFile(d, vlds); err != nil {
			return err
		}
	}
	return nil
}

func RunCommand(cmd *exec.Cmd) error {
	cmd.Stdout = os.Stdout
	fmt.Printf(" $ %s\n", strings.Join(cmd.Args, " "))
	return cmd.Run()
}

func AddCmdFlags(cmd *exec.Cmd, config *node.Config) {
	if config.Debug {
		cmd.Args = append(cmd.Args, "--"+consensus.FlagDebug)
	}
	if config.BroadcastTx {
		cmd.Args = append(cmd.Args, "--"+consensus.FlagBroadcastTx)
	}
	cmd.Args = append(cmd.Args, "--"+consensus.FlagDataDir, config.DataDir)
	cmd.Args = append(cmd.Args, "--"+consensus.FlagChainID,
		strconv.Itoa(int(config.ConsensusConfig.ChainID)))

	cmd.Args = append(cmd.Args, "--"+consensus.FlagPointPort, strconv.Itoa(config.PointPort))
	cmd.Args = append(cmd.Args, "--"+consensus.FlagTopicPort, strconv.Itoa(config.TopicPort))
	cmd.Args = append(cmd.Args, "--"+consensus.FlagAPIPort, strconv.Itoa(config.APIPort))

	cmd.Args = append(cmd.Args, "--"+consensus.FlagMerkleBranchFactor,
		strconv.Itoa(int(config.StorageConfig.MerkleBranchFactor)))

	cmd.Args = append(cmd.Args, "--"+consensus.FlagTxExecTimeout,
		config.ExecutionConfig.TxExecTimeout.String(),
	)
	cmd.Args = append(cmd.Args, "--"+consensus.FlagExecConcurrentLimit,
		strconv.Itoa(config.ExecutionConfig.ConcurrentLimit))

	cmd.Args = append(cmd.Args, "--"+consensus.FlagBlockTxLimit,
		strconv.Itoa(config.ConsensusConfig.BlockTxLimit))

	cmd.Args = append(cmd.Args, "--"+consensus.FlagTxWaitTime,
		config.ConsensusConfig.TxWaitTime.String())

	cmd.Args = append(cmd.Args, "--"+consensus.FlagViewWidth,
		config.ConsensusConfig.ViewWidth.String())

	cmd.Args = append(cmd.Args, "--"+consensus.FlagLeaderTimeout,
		config.ConsensusConfig.LeaderTimeout.String())

	cmd.Args = append(cmd.Args, "--"+consensus.FlagVoteStrategy,
		fmt.Sprintf("%d", config.ConsensusConfig.VoteStrategy))

	cmd.Args = append(cmd.Args, "--"+consensus.FlagBenchmarkPath,
		config.ConsensusConfig.BenchmarkPath)
}

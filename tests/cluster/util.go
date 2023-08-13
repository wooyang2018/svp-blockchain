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
	"github.com/wooyang2018/posv-blockchain/consensus"

	"github.com/wooyang2018/posv-blockchain/core"
	"github.com/wooyang2018/posv-blockchain/node"
)

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

func MakeRandomQuotas(count int, quota int) []uint32 {
	quotas := make([]uint32, count)
	if consensus.TwoPhaseBFTFlag {
		for i := 0; i < count; i++ {
			quotas[i] = 1
		}
		return quotas
	}
	r := rand.New(rand.NewSource(time.Now().Unix()))
	quotas[count-1] = uint32(quota+1) / 2
	remain := quota - int(quotas[count-1])
	for i := 0; i < remain; i++ {
		quotas[r.Intn(count-1)]++
	}
	sort.SliceStable(quotas, func(i int, j int) bool {
		return quotas[i] < quotas[j]
	})
	return quotas
}

func MakePeers(keys []*core.PrivateKey, addrs []multiaddr.Multiaddr) []node.Peer {
	vlds := make([]node.Peer, len(addrs))
	// create validator infos (pubkey + addr)
	for i, addr := range addrs {
		vlds[i] = node.Peer{
			PubKey: keys[i].PublicKey().Bytes(),
			Addr:   addr.String(),
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
	cmd.Args = append(cmd.Args, "-d", config.Datadir)
	cmd.Args = append(cmd.Args, "-p", strconv.Itoa(config.Port))
	cmd.Args = append(cmd.Args, "-P", strconv.Itoa(config.APIPort))

	if config.Debug {
		cmd.Args = append(cmd.Args, "--"+consensus.FlagDebug)
	}
	if config.BroadcastTx {
		cmd.Args = append(cmd.Args, "--"+consensus.FlagBroadcastTx)
	}

	cmd.Args = append(cmd.Args, "--"+consensus.FlagMerkleBranchFactor,
		strconv.Itoa(int(config.StorageConfig.MerkleBranchFactor)))

	cmd.Args = append(cmd.Args, "--"+consensus.FlagTxExecTimeout,
		config.ExecutionConfig.TxExecTimeout.String(),
	)
	cmd.Args = append(cmd.Args, "--"+consensus.FlagExecConcurrentLimit,
		strconv.Itoa(config.ExecutionConfig.ConcurrentLimit))

	cmd.Args = append(cmd.Args, "--"+consensus.FlagChainID,
		strconv.Itoa(int(config.ConsensusConfig.ChainID)))

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

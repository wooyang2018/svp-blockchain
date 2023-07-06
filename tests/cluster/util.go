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

func MakeRandomQuotas(count int, quota int) []float64 {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	quotas := make([]float64, count)
	sum := 0
	for i := 0; i <= count-2; i++ {
		temp := r.Intn(quota - sum - (count - 1 - i))
		sum += temp
		quotas[i] = float64(temp)
	}
	quotas[count-1] = float64(quota - sum)
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

func SetupTemplateDir(dir string, keys []*core.PrivateKey, quotas []float64, vlds []node.Peer) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.Mkdir(dir, 0755); err != nil {
		return err
	}
	//assert len(keys) == len(quotas)
	genesis := &node.Genesis{
		Validators: make([]string, len(keys)),
		Quotas:     make([]float64, len(keys)),
	}
	for i, v := range keys {
		genesis.Validators[i] = v.PublicKey().String()
		genesis.Quotas[i] = quotas[i]
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

	cmd.Args = append(cmd.Args, "--"+consensus.FlagDeltaTime,
		config.ConsensusConfig.DeltaTime.String())

	cmd.Args = append(cmd.Args, "--"+consensus.FlagBenchmarkPath,
		config.ConsensusConfig.BenchmarkPath)
}

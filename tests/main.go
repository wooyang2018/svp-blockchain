// Copyright (C) 2021 Aung Maw
// Licensed under the GNU General Public License v3.0

package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/wooyang2018/ppov-blockchain/node"
	"github.com/wooyang2018/ppov-blockchain/tests/cluster"
	"github.com/wooyang2018/ppov-blockchain/tests/experiments"
	"github.com/wooyang2018/ppov-blockchain/tests/testutil"
)

var (
	WorkDir   = "./workdir"
	NodeCount = 4

	LoadTxPerSec     = 100
	LoadMintAccounts = 100
	LoadDestAccounts = 10000 // increase dest accounts for benchmark

	PPoVCoinBinCC  = false // Deploy ppovcoin chaincode as bincc type (not embeded in ppov node)
	EmptyChainCode = true  // Deploy empty chaincode instead of ppovcoin

	// Run tests in remote linux cluster
	// if false it'll use local cluster (running multiple nodes on single local machine)
	RemoteLinuxCluster  = false
	RemoteSetupRequired = true
	RemoteLoginName     = "wooyoung"
	RemoteKeySSH        = "~/.ssh/id_rsa"
	RemoteHostsPath     = "hosts"
	RemoteNetworkDevice = "ens38"

	// run benchmark, otherwise run experiments
	RunBenchmark      = false
	BenchmarkDuration = 5 * time.Minute
	BenchLoads        = []int{1000, 2000, 3000, 4000, 4500, 5000, 5500, 6000, 7000}

	SetupClusterTemplate = false
)

func getNodeConfig() node.Config {
	config := node.DefaultConfig
	config.Debug = true
	config.BroadcastTx = false
	return config
}

func setupExperiments() []Experiment {
	expms := make([]Experiment, 0)
	if RemoteLinuxCluster {
		expms = append(expms, &experiments.NetworkDelay{
			Delay: 100 * time.Millisecond,
		})
		expms = append(expms, &experiments.NetworkPacketLoss{
			Percent: 10,
		})
	}
	expms = append(expms, &experiments.MajorityKeepRunning{})
	expms = append(expms, &experiments.CorrectExecution{})
	expms = append(expms, &experiments.RestartCluster{})
	return expms
}

func main() {
	printVars()
	os.Mkdir(WorkDir, 0755)
	buildPPoV()
	setupTransport()

	if RunBenchmark {
		runBenchmark()
	} else {
		var cfactory cluster.ClusterFactory
		if RemoteLinuxCluster {
			cfactory = makeRemoteClusterFactory()
		} else {
			cfactory = makeLocalClusterFactory()
		}
		if SetupClusterTemplate {
			cls, err := cfactory.SetupCluster("cluster_template")
			if err != nil {
				return
			}
			fmt.Println("\nThe cluster startup command is as follows:")
			for i := 0; i < cls.NodeCount(); i++ {
				fmt.Println(cls.GetNode(i).PrintCmd())
			}
			return
		}
		runExperiments(testutil.NewLoadGenerator(
			LoadTxPerSec, makeLoadClient(),
		), cfactory)
	}
}

func setupTransport() {
	// to make load test http client efficient
	transport := (http.DefaultTransport.(*http.Transport))
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 100
}

func runBenchmark() {
	if !RemoteLinuxCluster {
		fmt.Println("mush run benchmark on remote cluster")
		os.Exit(1)
		return
	}
	bm := &Benchmark{
		workDir:    path.Join(WorkDir, "benchmarks"),
		duration:   BenchmarkDuration,
		interval:   5 * time.Second,
		cfactory:   makeRemoteClusterFactory(),
		loadClient: makeLoadClient(),
	}
	bm.Run()
}

func runExperiments(loadGen *testutil.LoadGenerator, cfactory cluster.ClusterFactory) {
	r := &ExperimentRunner{
		experiments: setupExperiments(),
		cfactory:    cfactory,
		loadGen:     loadGen,
	}
	pass, fail := r.Run()
	fmt.Printf("\nTotal: %d  |  Pass: %d  |  Fail: %d\n", len(r.experiments), pass, fail)
}

func printVars() {
	fmt.Println()
	fmt.Println("NodeCount =", NodeCount)
	fmt.Println("LoadTxPerSec=", LoadTxPerSec)
	fmt.Println("RemoteCluster =", RemoteLinuxCluster)
	fmt.Println("RunBenchmark=", RunBenchmark)
	fmt.Println("SetupClusterTemplate=", SetupClusterTemplate)
	fmt.Println()
}

func buildPPoV() {
	cmd := exec.Command("go", "build", "../cmd/chain")
	if RemoteLinuxCluster {
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, "GOOS=linux")
		fmt.Printf(" $ export %s\n", "GOOS=linux")
	}
	fmt.Printf(" $ %s\n\n", strings.Join(cmd.Args, " "))
	check(cmd.Run())
}

func makeLoadClient() testutil.LoadClient {
	fmt.Println("Preparing load client")
	if EmptyChainCode {
		return testutil.NewEmptyClient()
	}
	var binccPath string
	if PPoVCoinBinCC {
		buildPPoVCoinBinCC()
		binccPath = "./ppovcoin"
	}
	return testutil.NewPPoVCoinClient(LoadMintAccounts, LoadDestAccounts, binccPath)
}

func buildPPoVCoinBinCC() {
	cmd := exec.Command("go", "build")
	cmd.Args = append(cmd.Args, "-ldflags", "-s -w")
	cmd.Args = append(cmd.Args, "../execution/bincc/ppovcoin")
	if RemoteLinuxCluster {
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, "GOOS=linux")
		fmt.Printf(" $ export %s\n", "GOOS=linux")
	}
	fmt.Printf(" $ %s\n\n", strings.Join(cmd.Args, " "))
	check(cmd.Run())
}

func makeLocalClusterFactory() *cluster.LocalFactory {
	ftry, err := cluster.NewLocalFactory(cluster.LocalFactoryParams{
		BinPath:    "./chain",
		WorkDir:    path.Join(WorkDir, "local-clusters"),
		NodeCount:  NodeCount,
		NodeConfig: getNodeConfig(),
	})
	check(err)
	return ftry
}

func makeRemoteClusterFactory() *cluster.RemoteFactory {
	ftry, err := cluster.NewRemoteFactory(cluster.RemoteFactoryParams{
		BinPath:       "./chain",
		WorkDir:       path.Join(WorkDir, "remote-clusters"),
		NodeCount:     NodeCount,
		NodeConfig:    getNodeConfig(),
		LoginName:     RemoteLoginName,
		KeySSH:        RemoteKeySSH,
		HostsPath:     RemoteHostsPath,
		SetupRequired: RemoteSetupRequired,
		NetworkDevice: RemoteNetworkDevice,
	})
	check(err)
	return ftry
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// Copyright (C) 2021 Aung Maw
// Copyright (C) 2023 Wooyang2018
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

	"github.com/wooyang2018/ppov-blockchain/consensus"
	"github.com/wooyang2018/ppov-blockchain/node"
	"github.com/wooyang2018/ppov-blockchain/tests/cluster"
	"github.com/wooyang2018/ppov-blockchain/tests/experiments"
	"github.com/wooyang2018/ppov-blockchain/tests/testutil"
)

var (
	WorkDir   = "./workdir"
	NodeCount = 4

	LoadTxPerSec    = 100
	LoadJobPerTick  = 1000
	LoadSubmitNodes = []int{0}
	LoadBatchSubmit = true //whether to enable batch transaction submission

	//chaincode priority: empty > ppovcoin bincc > ppovcoin
	EmptyChainCode = false // deploy empty chaincode instead of ppovcoin
	PPoVCoinBinCC  = false // deploy ppovcoin chaincode as bincc type (not embeded in ppov node)
	CheckRotation  = true
	BroadcastTx    = true

	// run tests in remote linux cluster
	RemoteLinuxCluster    = true // if false it'll use local cluster (running multiple nodes on single local machine)
	RemoteSetupRequired   = true
	RemoteInstallRequired = false // if false it will not try to install dstat on remote machine
	RemoteKeySSH          = "~/.ssh/id_rsa"
	RemoteHostsPath       = "hosts"

	// run benchmark, otherwise run experiments
	RunBenchmark  = true
	BenchDuration = 5 * time.Minute
	BenchLoads    = []int{5000, 10000, 15000}

	SetupClusterTemplate = false
)

func getNodeConfig() node.Config {
	config := node.DefaultConfig
	config.Debug = true
	config.BroadcastTx = BroadcastTx
	if !CheckRotation {
		config.ConsensusConfig.ViewWidth = 1 * time.Hour
		config.ConsensusConfig.LeaderTimeout = 1 * time.Hour
	}
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
	printAndCheckVars()
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
			cls.EmptyChainCode = EmptyChainCode
			cls.CheckRotation = CheckRotation
			fmt.Println("\nThe cluster startup command is as follows:")
			for i := 0; i < cls.NodeCount(); i++ {
				fmt.Println(cls.GetNode(i).PrintCmd())
			}
			return
		}
		runExperiments(testutil.NewLoadGenerator(makeLoadClient(),
			LoadTxPerSec, LoadJobPerTick), cfactory)
	}
}

func setupTransport() {
	// to make load test http client efficient
	transport := http.DefaultTransport.(*http.Transport)
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 100
}

func runBenchmark() {
	bm := &Benchmark{
		workDir:    path.Join(WorkDir, "benchmarks"),
		duration:   BenchDuration,
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

func printAndCheckVars() {
	fmt.Println("NodeCount =", NodeCount)
	fmt.Println("LoadJobPerTick =", LoadJobPerTick)
	fmt.Println("LoadSubmitNodes =", LoadSubmitNodes)
	fmt.Println("LoadBatchSubmit =", LoadBatchSubmit)
	fmt.Println("EmptyChainCode =", EmptyChainCode)
	fmt.Println("CheckRotation =", CheckRotation)
	fmt.Println("BroadcastTx =", BroadcastTx)
	fmt.Println("RemoteLinuxCluster =", RemoteLinuxCluster)
	fmt.Println("RunBenchmark =", RunBenchmark)
	fmt.Println("BenchLoads =", BenchLoads)
	fmt.Println("SetupClusterTemplate =", SetupClusterTemplate)
	fmt.Println("consensus.ExecuteTxFlag =", consensus.ExecuteTxFlag)
	fmt.Println()
	pass := true
	if !BroadcastTx && len(LoadSubmitNodes) != 1 {
		fmt.Println("!BroadcastTx ===> len(LoadSubmitNodes)=1")
		pass = false
	}
	if !RunBenchmark && len(LoadSubmitNodes) != 0 {
		fmt.Println("!RunBenchmark =?=> len(LoadSubmitNodes)=0")
	}
	if RunBenchmark && !LoadBatchSubmit {
		fmt.Println("RunBenchmark =?=> LoadBatchSubmit")
	}
	if !RunBenchmark && !CheckRotation {
		fmt.Println("!RunBenchmark ===> CheckRotation")
		pass = false
	}
	if !RunBenchmark && !BroadcastTx {
		fmt.Println("!RunBenchmark ===> BroadcastTx")
		pass = false
	}
	if !consensus.ExecuteTxFlag && BroadcastTx || !BroadcastTx && consensus.ExecuteTxFlag {
		fmt.Println("!consensus.ExecuteTxFlag <=?=> !BroadcastTx")
	}
	if consensus.ExecuteTxFlag && !BroadcastTx {
		fmt.Println("consensus.ExecuteTxFlag ===> BroadcastTx")
		pass = false
	}
	if RunBenchmark && !RemoteLinuxCluster {
		fmt.Println("RunBenchmark ===> RemoteLinuxCluster")
		pass = false
	}
	if SetupClusterTemplate && RunBenchmark {
		fmt.Println("SetupClusterTemplate ===> !RunBenchmark")
		pass = false
	}
	if !consensus.ExecuteTxFlag && RunBenchmark {
		fmt.Println("!consensus.ExecuteTxFlag ===> !RunBenchmark")
		pass = false
	}
	if !consensus.ExecuteTxFlag && len(BenchLoads) > 1 {
		fmt.Println("!consensus.ExecuteTxFlag =?=> len(BenchLoads)=1")
	}
	if !RunBenchmark && !consensus.ExecuteTxFlag {
		fmt.Println("!RunBenchmark ===> consensus.ExecuteTxFlag")
		pass = false
	}
	if pass {
		fmt.Println("test parameters passed verification")
	} else {
		os.Exit(1)
	}
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
		return testutil.NewEmptyClient(LoadSubmitNodes)
	}
	var binccPath string
	if PPoVCoinBinCC {
		buildPPoVCoinBinCC()
		binccPath = "./ppovcoin"
	}
	mintAccounts := 100
	destAccounts := 10000 // increase dest accounts for benchmark
	return testutil.NewPPoVCoinClient(LoadSubmitNodes, mintAccounts, destAccounts, binccPath)
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
		BinPath:         "./chain",
		WorkDir:         path.Join(WorkDir, "remote-clusters"),
		NodeCount:       NodeCount,
		NodeConfig:      getNodeConfig(),
		KeySSH:          RemoteKeySSH,
		HostsPath:       RemoteHostsPath,
		SetupRequired:   RemoteSetupRequired,
		InstallRequired: RemoteInstallRequired,
	})
	check(err)
	return ftry
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

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
	"runtime"
	"strings"
	"time"

	"github.com/wooyang2018/posv-blockchain/consensus"
	"github.com/wooyang2018/posv-blockchain/node"
	"github.com/wooyang2018/posv-blockchain/tests/cluster"
	"github.com/wooyang2018/posv-blockchain/tests/experiments"
	"github.com/wooyang2018/posv-blockchain/tests/testutil"
)

var (
	WorkDir    = "./workdir"
	NodeCount  = 7
	StakeQuota = 100
	WinSize    = 4

	LoadTxPerSec    = 10  //tps for client to submit tx during functional testing
	LoadJobPerTick  = 100 //num of tasks to be completed per tick
	LoadSubmitNodes = []int{}
	LoadBatchSubmit = true //whether to enable batch transaction submission

	//chaincode priority: empty > pcoin bincc > pcoin
	EmptyChainCode = false // deploy empty chaincode instead of pcoin
	PCoinBinCC     = true  // deploy pcoin chaincode as bincc type (not embeded in node)
	CheckRotation  = true
	BroadcastTx    = true

	// run tests in remote linux cluster
	RemoteLinuxCluster    = true // if false it'll use local cluster (running multiple nodes on single local machine)
	RemoteSetupRequired   = true
	RemoteInstallRequired = false // if false it will not try to install dstat on remote machine
	RemoteKeySSH          = "~/.ssh/id_rsa"
	RemoteHostsPath       = "hosts"
	RemoteNetworkDelay    = 500 * time.Millisecond
	RemoteNetworkLoss     = 10.0

	// run benchmark, otherwise run experiments
	RunBenchmark  = false
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
			Delay: RemoteNetworkDelay,
		})
		expms = append(expms, &experiments.NetworkPacketLoss{
			Percent: RemoteNetworkLoss,
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
	buildChain()
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
			if cls, err := cfactory.SetupCluster("cluster_template"); err == nil {
				cls.EmptyChainCode = EmptyChainCode
				cls.CheckRotation = CheckRotation
				fmt.Println("\nThe cluster startup command is as follows:")
				for i := 0; i < cls.NodeCount(); i++ {
					fmt.Println(cls.GetNode(i).PrintCmd())
				}
			} else {
				fmt.Println(err)
			}
			return
		}
		testutil.NewLoadGenerator(makeLoadClient(), LoadTxPerSec, LoadJobPerTick)
		runExperiments(cfactory)
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

func runExperiments(cfactory cluster.ClusterFactory) {
	r := &ExperimentRunner{
		experiments: setupExperiments(),
		cfactory:    cfactory,
	}
	pass, fail := r.Run()
	fmt.Printf("\nTotal: %d  |  Pass: %d  |  Fail: %d\n", len(r.experiments), pass, fail)
}

func printAndCheckVars() {
	if !SetupClusterTemplate && runtime.GOOS == "windows" {
		fmt.Println("cannot use windows to run experiments")
		os.Exit(1)
	}
	fmt.Println("NodeCount =", NodeCount)
	fmt.Println("StakeQuota =", StakeQuota)
	fmt.Println("LoadJobPerTick =", LoadJobPerTick)
	fmt.Println("LoadSubmitNodes =", LoadSubmitNodes)
	fmt.Println("LoadBatchSubmit =", LoadBatchSubmit)
	fmt.Println("EmptyChainCode =", EmptyChainCode)
	fmt.Println("CheckRotation =", CheckRotation)
	fmt.Println("BroadcastTx =", BroadcastTx)
	fmt.Println("RemoteLinuxCluster =", RemoteLinuxCluster)
	fmt.Println("RemoteNetworkDelay =", RemoteNetworkDelay)
	fmt.Println("RemoteNetworkLoss =", RemoteNetworkLoss)
	fmt.Println("RunBenchmark =", RunBenchmark)
	fmt.Println("BenchLoads =", BenchLoads)
	fmt.Println("ExecuteTxFlag =", consensus.ExecuteTxFlag)
	fmt.Println("PreserveTxFlag =", consensus.PreserveTxFlag)
	fmt.Println()
	pass := true
	if !BroadcastTx && len(LoadSubmitNodes) != 1 {
		fmt.Println("!BroadcastTx ===> len(LoadSubmitNodes)=1")
		pass = false
	}
	if !RunBenchmark && len(LoadSubmitNodes) != 0 {
		fmt.Println("!RunBenchmark ---> len(LoadSubmitNodes)=0")
	}
	if RunBenchmark && !LoadBatchSubmit {
		fmt.Println("RunBenchmark ---> LoadBatchSubmit")
	}
	if !consensus.ExecuteTxFlag && !EmptyChainCode {
		fmt.Println("!ExecuteTxFlag ===> EmptyChainCode")
		pass = false
	}
	if !RunBenchmark && !CheckRotation {
		fmt.Println("!RunBenchmark ===> CheckRotation")
		pass = false
	}
	if !RunBenchmark && !BroadcastTx {
		fmt.Println("!RunBenchmark ===> BroadcastTx")
		pass = false
	}
	if !consensus.ExecuteTxFlag && BroadcastTx {
		fmt.Println("!ExecuteTxFlag ---> !BroadcastTx")
	}
	if consensus.ExecuteTxFlag && !BroadcastTx {
		fmt.Println("ExecuteTxFlag ===> BroadcastTx")
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
	if !RunBenchmark && !consensus.ExecuteTxFlag {
		fmt.Println("!RunBenchmark ===> ExecuteTxFlag")
		pass = false
	}
	if !RunBenchmark && consensus.PreserveTxFlag {
		fmt.Println("!RunBenchmark ===> !PreserveTxFlag")
		pass = false
	}
	if consensus.ExecuteTxFlag && consensus.PreserveTxFlag {
		fmt.Println("ExecuteTxFlag ===> !PreserveTxFlag")
		pass = false
	}
	if pass {
		fmt.Println("passed testing parameters validation")
	} else {
		os.Exit(1)
	}
	fmt.Println()
}

func buildChain() {
	cmd := exec.Command("go", "build", "../cmd/chain")
	if RemoteLinuxCluster {
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, "GOOS=linux", "GOARCH=amd64")
		fmt.Printf(" $ export %s\n", "GOOS=linux GOARCH=amd64")
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
	if PCoinBinCC {
		buildPCoinBinCC()
		binccPath = "./pcoin"
	}
	mintAccounts := 100
	destAccounts := 10000 // increase dest accounts for benchmark
	return testutil.NewPCoinClient(LoadSubmitNodes, mintAccounts, destAccounts, binccPath)
}

func buildPCoinBinCC() {
	cmd := exec.Command("go", "build")
	cmd.Args = append(cmd.Args, "-ldflags", "-s -w")
	cmd.Args = append(cmd.Args, "../execution/bincc/pcoin")
	if RemoteLinuxCluster {
		cmd.Env = os.Environ()
		os.Setenv("GOOS", "linux")
		os.Setenv("GOARCH", "amd64")
		fmt.Printf(" $ export %s\n", "GOOS=linux GOARCH=amd64")
	}
	fmt.Printf(" $ %s\n\n", strings.Join(cmd.Args, " "))
	check(cmd.Run())
}

func makeLocalClusterFactory() *cluster.LocalFactory {
	ftry, err := cluster.NewLocalFactory(cluster.LocalFactoryParams{
		BinPath:    "./chain",
		WorkDir:    path.Join(WorkDir, "local-clusters"),
		NodeCount:  NodeCount,
		StakeQuota: StakeQuota,
		WinSize:    WinSize,
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
		StakeQuota:      StakeQuota,
		WinSize:         WinSize,
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

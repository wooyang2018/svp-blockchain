# SVP Blockchain

## Getting Started

You can run the cluster tests on local machine in a few seconds.

1. Install dependencies

```shell
sudo apt-get install build-essential
```

2. Download and install [`golang`](https://golang.org/doc/install)
3. Prepare the repo and run tests

```shell
cd tests && go run .
```

The test script will compile `cmd/chain` and set up a cluster of 7 nodes with different ports on the local machine.

Experiments from `tests/experiments` will be run and health checks will be performed throughput the tests.

***NOTE**: Network simulation experiments are only run on the remote linux cluster.*

## Deployment

### Local Mode

Modify the following global variables in the source code of `tests/main.go`.

```
NodeCount = 7 // your expected number of nodes
RemoteLinuxCluster = false
OnlySetupCluster = true
```

Then execute the following command to generate the configuration files and startup commands required to start each local node.

```shell
cd tests && go run .
```

The start command for each local node is displayed directly to the console as follows. Now, you can open 7 shell windows and start each node in turn.

### Remote Mode

Modify the following global variables in the source code of `tests/main.go`.

```
NodeCount = 7 // your expected number of nodes
RemoteLinuxCluster = true
RemoteKeySSH = "~/.ssh/id_rsa"
RemoteHostsPath = "hosts"
OnlySetupCluster = true
```

You will also need to modify the `tests/hosts` file based on your remote node information.

Each line corresponds to a node's IP address, username, NIC name and working directory and is separated by a tab.

Then execute the following command to generate the configuration files and startup commands required to start each remote node.

```shell
cd tests && go run .
```

In the above command, we will also use `scp` to transfer the relevant files to the remote node. So the private key configured by `RemoteKeySSH` is required to be able to log in to the machine properly.

The start command for each remote node is displayed directly to the console. Now you can log in to the remote machine and start each blockchain node.

## Native Contract

To implement a native contract, you only need to implement the following interface. You can refer to our built-in contract, `KVDB`, for specific examples.

```
type Chaincode interface {
	// Init is called when chaincode is deployed
	Init(ctx CallContext) error
	Invoke(ctx CallContext) error
	Query(ctx CallContext) ([]byte, error)
}
```

Next, we test the functionalities of the built-in KVDB contract. Here, we deploy the blockchain in local mode and set `OnlyRunCluster` to true in the `tests/main.go`.

```
NodeCount = 7 // your expected number of nodes
RemoteLinuxCluster = false
OnlySetupCluster = false
OnlyRunCluster = true
```

Then execute the following command to start the blockchain for continuous operation.

```shell
cd tests && go run .
```

Then open another shell and compile the client corresponding to the KVDB contract using the following command.

```shell
cd native/kvdb/client
go build
./client --help
```

Create an account and deploy the KVDB contract using the following command.

```shell
 ./client account
 ./client deploy
```

Then use the following command to set key-value pairs.

```shell
./client set --key=hello --value=world
```

Finally, query the value corresponding to the key.

```shell
./client get --key=hello
```

## Benchmark

Modify the following global variables in the source code of `tests/main.go`.

```
NodeCount = 7 // your expected number of nodes
WindowSize = 4 // your expected size of voting window 
RemoteLinuxCluster = true
RemoteKeySSH = "~/.ssh/id_rsa"
RemoteHostsPath = "hosts"
RunBenchmark = true 
OnlySetupCluster = false
OnlyRunCluster = false
```

And other variables are in the source code of `consensus\config.go`.

```
BlockTxLimit = 10000 // maximum tx count in a block
VoteStrategy = RandomVote // voting strategy adopted by validator
ExecuteTxFlag = false
PreserveTxFlag = true 
```

Then execute the following command to start the benchmark.

```
cd tests && go run .
```

The results of the client's perspective are displayed in the console. The results of the consensus protocol are displayed in the log file of the remote node.

***NOTE**: Benchmark experiments are only run on the remote linux cluster.*

## About the Project

### License

This project is licensed under the GPL-3.0 License.

### Contributing

When contributing to this repository, please first discuss the change you wish to make via issue, email, or any other method with the owners of this repository before making a change.

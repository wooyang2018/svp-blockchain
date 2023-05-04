# PPoV Blockchain

Our PPoV blockchain is based on [Juria](https://aungmawjj.github.io/juria-blockchain) which is a high-performance consortium blockchain with [Hotstuff](https://arxiv.org/abs/1803.05069) consensus mechanism and a transaction-based state machine.

Hotstuff provides a mechanism to rotate leader (block maker) efficiently among the validator nodes. Hence it is not required to have a single trusted leader in the network.

With the use of Hotstuff three-chain commit rule, Juria blockchain ensures that the same history of blocks is committed on all nodes despite network and machine failures.

## Getting Started

You can run the cluster tests on local machine in a few seconds.

1. Install dependencies

```bash
sudo apt-get install build-essential
```

2. Download and install [`go 1.18`](https://golang.org/doc/install)
3. Prepare the repo and run tests

```bash
cd tests
go run .
```

The test script will compile `cmd/chain` and set up a cluster of 4 nodes with different ports on the local machine. Experiments from `tests/experiments` will be run and health checks will be performed throughout the tests.

***NOTE**: Network simulation experiments are only run on the remote linux cluster.*

## Deployment

### Local Mode

Modify the following global variables in the source code of `tests/main.go`.

```go
NodeCount = 4 //your expected number of nodes
RemoteLinuxCluster = false
SetupClusterTemplate = true
```

Then execute the following command to generate the configuration files and startup commands required to start each local node.

```bash
cd tests && go run .
```

The start command for each local node is displayed directly to the console as follows.

```bash
./chain -d workdir/local-clusters/cluster_template/0 -p 15150 -P 9040 --debug --storage-merkleBranchFactor 8 --execution-txExecTimeout 10s --execution-concurrentLimit 20 --chainID 0 --consensus-batchTxLimit 5000 --consensus-blockBatchLimit 4 --consensus-voteBatchLimit 4 --consensus-txWaitTime 1s --consensus-batchWaitTime 3s --consensus-proposeTimeout 1.5s --consensus-batchTimeout 1s --consensus-blockDelay 100ms --consensus-viewWidth 1m0s --consensus-leaderTimeout 20s
...
```

Now, you can open 4 Shell windows and start each node in turn. 

### Remote Mode

Modify the following global variables in the source code of `tests/main.go`.

```go
NodeCount = 4 //your expected number of nodes
RemoteLinuxCluster = true
RemoteKeySSH = "~/.ssh/id_rsa"
RemoteHostsPath = "hosts"
SetupClusterTemplate = true
```

You will also need to modify the `tests/hosts` file based on your remote node information. Each line corresponds to a node's IP address, username, NIC name and working directory and is separated by a tab.

Then execute the following command to generate the configuration files and startup commands required to start each remote node.

```bash
cd tests && go run .
```

In the above command, we will also use `scp` to transfer the relevant files to the remote node. So the private key configured by `RemoteKeySSH` is required to be able to log in to the machine properly. 

The start command for each remote node is displayed directly to the console. Now you can log in to the remote machine and start each blockchain node.

## About the Project

### License

This project is licensed under the GPL-3.0 License.

### Contributing

When contributing to this repository, please first discuss the change you wish to make via issue, email, or any other method with the owners of this repository before making a change.

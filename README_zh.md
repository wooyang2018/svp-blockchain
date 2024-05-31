# SVP区块链

## 入门

你可以在几秒钟内在本地机器上运行集群测试。

1. 安装依赖项

```shell
sudo apt-get install build-essential
```

2. 下载并安装 [`golang`](https://golang.org/doc/install)
3. 准备代码库并运行测试

```shell
cd tests && go run.
```

测试脚本将编译 `cmd/chain` 并在本地机器上设置具有不同端口的节点的集群。

将运行 `tests/experiments` 中的实验，并在整个测试过程中进行健康检查。

***注意**：网络模拟实验仅在远程 Linux 集群上运行。*

## 部署

### Local模式

在 `tests/main.go` 的源代码中修改以下全局变量。

```
NodeCount = 7 // 你期望的节点数量
RemoteLinuxCluster = false
OnlySetupCluster = true
```

然后执行以下命令以生成启动所有本地节点所需的配置文件和启动命令。

```shell
cd tests && go run.
```

每个本地节点的启动命令将直接显示在控制台上。现在，你可以打开 7 个 shell 窗口并依次启动每个节点。

### Docker模式

在 `tests/main.go` 的源代码中修改以下全局变量。

```
NodeCount = 7 // 你期望的节点数量
RemoteLinuxCluster = false
OnlySetupDocker = true
```

然后执行以下命令以生成启动所有容器所需的配置文件和 `docker-compose.yaml`。

```shell
cd tests && go run.
```

现在`docker-compose`的启动命令将直接显示在控制台上。

### SEED模式

首先按照如下步骤安装`seed-emulator`，这是一个用于网络仿真的平台。

```
git clone https://github.com/seed-labs/seed-emulator.git
cd seed-emulator
python setup.py install
pip install -r requirements.txt
```

SEED模式基于Docker模式实现，所以请先配置Docker模式。然后修改 `tests/seedemu/main.py`中的`dimension`
变量，保证`NodeCount=2^dimension`，即节点数量相等。然后执行以下命令以生成启动`seed-emulator`需要的配置文件。

```
cd tests/seedemu/ 
python main.py 
cd output
docker-compose build 
docker-compose up
```

现在你可以进入到容器中查看区块链的相关日志。

```
docker exec -it ec89be7e63a8 /bin/bash
tail -f app/output.log
```

通过http://127.0.0.1:8080/map.html查看`seed-emulator`支持的Web UI。

### 远程模式

在 `tests/main.go` 的源代码中修改以下全局变量。

```
NodeCount = 7 // 你期望的节点数量
RemoteLinuxCluster = true
RemoteKeySSH = "~/.ssh/id_rsa"
RemoteHostsPath = "hosts"
OnlySetupCluster = true
```

你还需要根据你的远程节点信息修改 `tests/hosts` 文件。

每行对应一个节点的 IP 地址、用户名、NIC 名称和工作目录，并由制表符分隔。

然后执行以下命令以生成启动所有远程节点所需的配置文件和启动命令。

```shell
cd tests && go run.
```

在上述命令中，我们还将使用 `scp` 将相关文件传输到远程节点。因此，需要在 `RemoteKeySSH` 中配置能够正确登录到远程节点的私钥。

每个远程节点的启动命令将直接显示在控制台上。现在你可以登录到远程机器并启动每个区块链节点。

## 本地合约

要实现本地合约，你只需要实现以下接口。你可以参考我们的内置合约 `KVDB` 以获取具体示例。

```
type Chaincode interface {
    // 在部署链码时调用
    Init(ctx CallContext) error
    Invoke(ctx CallContext) error
    Query(ctx CallContext) ([]byte, error)
}
```

### KVDB合约

接下来，我们测试内置 `KVDB` 合约的功能。我们在Local模式下部署区块链，并在 `tests/main.go` 中设置 `OnlyRunCluster` 为 true。

```
NodeCount = 7 // 你期望的节点数量
RemoteLinuxCluster = false
OnlySetupCluster = false
OnlyRunCluster = true
```

然后执行以下命令以启动区块链进行后续操作。

```shell
cd tests && go run.
```

然后打开另一个 shell，并使用以下命令编译对应于 `KVDB` 合约的客户端。

```
cd native/kvdb/client
go build
./client --help
```

创建一个账户并使用以下命令部署 `KVDB` 合约。

```
./client account
./client deploy
```

然后使用以下命令设置键值对。

```
./client set --key=hello --value=world
```

最后，查询对应键的值。

```
./client get --key=hello
```

### XCoin合约

`XCoin` 合约为系统内置合约，由创世区块初始化，不允许用户部署。

打开一个 shell，并使用以下命令编译对应于 `XCoin` 合约的客户端。然后将`client`
命令移动到某个节点的工作目录下，使用其`nodekey`进行`Xcoin`合约的相关交互。

```
go build ./native/xcoin/client
mv client tests/workdir/local-clusters/keep_alive_running/0
cd tests/workdir/local-clusters/keep_alive_running/0
./client --help
```

根据公钥查询指定节点的账户余额。

```
./client --code xcoin balance --dest VwCSwqXzfG871UScbUdUt8hNZT6C/4OmCzHO2s7Hfag=
```

给目标节点转账指定的金额，不能超过节点自身的余额。

```
./client --code xcoin transfer --dest VgYxN5bVi5B2JXAJ5qWm/yhQ7rv/7sAeikNSu6C0IqM= --value 100
```

### TAddr合约

`TAddr` 合约为系统内置合约，由创世区块初始化，不允许用户部署。

打开一个 shell，并使用以下命令编译对应于 `TAddr` 合约的客户端。然后将`client`
命令移动到某个节点的工作目录下，使用其`nodekey`进行`Taddr`合约的相关交互。

```
go build ./native/taddr/client
mv client tests/workdir/local-clusters/keep_alive_running/0
cd tests/workdir/local-clusters/keep_alive_running/0
./client --help
```

根据节点公钥查询其以太坊短地址。

```
./client --code taddr query --addr VwCSwqXzfG871UScbUdUt8hNZT6C/4OmCzHO2s7Hfag=
```

根据以太坊短地址查询其节点公钥。

```
./client --code taddr query --addr 570092c2a5f37c6f3bd5449c6d4754b7c84d653e
```

## 基准测试

在 `tests/main.go` 的源代码中修改以下全局变量。

```
NodeCount = 7 // 你期望的节点数量
WindowSize = 4 // 你期望的投票窗口大小 
RemoteLinuxCluster = true
RemoteKeySSH = "~/.ssh/id_rsa"
RemoteHostsPath = "hosts"
RunBenchmark = true 
OnlySetupCluster = false
OnlyRunCluster = false
```

并且其他变量在 `consensus/config.go` 的源代码中。

```
BlockTxLimit = 10000 // 一个块中最大的交易数量
VoteStrategy = RandomVote // 验证者采用的投票策略
ExecuteTxFlag = false
PreserveTxFlag = true 
```

然后执行以下命令开始基准测试。

```
cd tests && go run.
```

客户端视角的结果将显示在控制台上。共识协议的结果将显示在远程节点的日志文件中。

***注意**：基准测试实验仅在远程 Linux 集群上运行。*

## 关于项目

### 许可证

本项目在 GPL-3.0 许可证下授权。

### 贡献

当向此存储库贡献时，请首先通过问题、电子邮件或与该存储库所有者的任何其他方法讨论你希望进行的更改，然后再进行更改。
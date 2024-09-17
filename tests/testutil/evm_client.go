// Copyright (C) 2024 XZLiang
// Licensed under the GNU General Public License v3.0

package testutil

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"path"
	"strconv"
	"time"

	"github.com/wooyang2018/svp-blockchain/consensus"
	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/execution/evm"
	"github.com/wooyang2018/svp-blockchain/native"
	"github.com/wooyang2018/svp-blockchain/tests/cluster"
)

type EVMClient struct {
	contractPath       string
	contractCodeID     []byte
	contractUploadNode int

	signer   *core.PrivateKey
	cluster  *cluster.Cluster
	codeAddr []byte
	nodes    []int
}

var _ LoadClient = (*EVMClient)(nil)

func NewEVMClient(nodes []int, contractPath string) *EVMClient {
	return &EVMClient{
		contractPath: contractPath,
		nodes:        nodes,
	}
}

func (client *EVMClient) SetupOnCluster(cls *cluster.Cluster) error {
	return client.setupOnCluster(cls)
}

func (client *EVMClient) SubmitTxAndWait() (int, error) {
	return SubmitTxAndWait(client.cluster, client.MakeInvokeTx())
}

func (client *EVMClient) SubmitTx() (int, *core.Transaction, error) {
	tx := client.MakeInvokeTx()
	nodeIdx, err := SubmitTx(client.cluster, client.nodes, tx)
	return nodeIdx, tx, err
}

func (client *EVMClient) BatchSubmitTx(num int) (int, *core.TxList, error) {
	jobCh := make(chan struct{}, num)
	defer close(jobCh)
	out := make(chan *core.Transaction, num)
	defer close(out)

	for i := 0; i < 100; i++ {
		go func(jobCh <-chan struct{}, out chan<- *core.Transaction) {
			for range jobCh {
				out <- client.MakeInvokeTx()
			}
		}(jobCh, out)
	}
	for i := 0; i < num; i++ {
		jobCh <- struct{}{}
	}
	txs := make([]*core.Transaction, num)
	for i := 0; i < num; i++ {
		txs[i] = <-out
	}

	txList := core.TxList(txs)
	nodeIdx, err := BatchSubmitTx(client.cluster, client.nodes, &txList)
	return nodeIdx, &txList, err
}

func (client *EVMClient) setupOnCluster(cls *cluster.Cluster) error {
	fmt.Println("Reading", path.Join(cluster.Node0DataDir, native.FileNodekey))
	client.signer = cluster.Node0Key
	client.cluster = cls
	if consensus.ExecuteTxFlag {
		if err := client.deploy(); err != nil {
			return err
		}
	}
	return nil
}

func (client *EVMClient) deploy() error {
	if client.contractPath != "" {
		i, codeID, err := uploadEVMChainCode(client.cluster, client.contractPath)
		if err != nil {
			return err
		}
		client.contractCodeID = codeID
		client.contractUploadNode = i
	}
	depTx := client.MakeDeploymentTx()
	if _, err := SubmitTxAndWait(client.cluster, depTx); err != nil {
		return fmt.Errorf("cannot deploy evm contract, %w", err)
	}
	client.codeAddr = depTx.Hash()
	return nil
}

func (client *EVMClient) MakeDeploymentTx() *core.Transaction {
	input := client.evmDeploymentInput()
	b, _ := json.Marshal(input)
	return core.NewTransaction().
		SetNonce(time.Now().UnixNano()).
		SetInput(b).
		Sign(client.signer)
}

func (client *EVMClient) evmDeploymentInput() *common.DeploymentInput {
	initInput := &evm.InitInput{Class: "Storage"}
	b, _ := json.Marshal(initInput)
	return &common.DeploymentInput{
		CodeInfo: common.CodeInfo{
			DriverType: common.DriverTypeEVM,
			CodeID:     client.contractCodeID,
		},
		InitInput: b,
		InstallData: []byte(fmt.Sprintf("%s/contract/%s",
			client.cluster.GetNode(client.contractUploadNode).GetEndpoint(),
			hex.EncodeToString(client.contractCodeID),
		)),
	}
}

func (client *EVMClient) MakeInvokeTx() *core.Transaction {
	input := &evm.Input{
		Method: "store",
		Params: []string{strconv.Itoa(rand.Int())},
		Types:  []string{"uint256"},
	}
	b, _ := json.Marshal(input)
	return core.NewTransaction().
		SetCodeAddr(client.codeAddr).
		SetNonce(time.Now().UnixNano()).
		SetInput(b).
		Sign(client.signer)
}

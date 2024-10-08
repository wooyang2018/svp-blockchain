// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package testutil

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"time"

	"github.com/wooyang2018/svp-blockchain/consensus"
	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/native"
	"github.com/wooyang2018/svp-blockchain/tests/cluster"
)

type EmptyClient struct {
	signer   *core.PrivateKey
	cluster  *cluster.Cluster
	codeAddr []byte
	nodes    []int
}

var _ LoadClient = (*EmptyClient)(nil)

func NewEmptyClient(nodes []int) *EmptyClient {
	return &EmptyClient{
		signer: core.GenerateKey(nil),
		nodes:  nodes,
	}
}

func (client *EmptyClient) SetupOnCluster(cls *cluster.Cluster) error {
	return client.setupOnCluster(cls)
}

func (client *EmptyClient) SubmitTxAndWait() (int, error) {
	return SubmitTxAndWait(client.cluster, client.MakeTx())
}

func (client *EmptyClient) SubmitTx() (int, *core.Transaction, error) {
	tx := client.MakeTx()
	nodeIdx, err := SubmitTx(client.cluster, client.nodes, tx)
	return nodeIdx, tx, err
}

func (client *EmptyClient) BatchSubmitTx(num int) (int, *core.TxList, error) {
	jobCh := make(chan struct{}, num)
	defer close(jobCh)
	out := make(chan *core.Transaction, num)
	defer close(out)

	for i := 0; i < 100; i++ {
		go func(jobCh <-chan struct{}, out chan<- *core.Transaction) {
			for range jobCh {
				out <- client.MakeTx()
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

func (client *EmptyClient) setupOnCluster(cls *cluster.Cluster) error {
	client.cluster = cls
	if consensus.ExecuteTxFlag {
		if err := client.deploy(); err != nil {
			return err
		}
	}
	return nil
}

func (client *EmptyClient) deploy() error {
	depTx := client.MakeDeploymentTx()
	if _, err := SubmitTxAndWait(client.cluster, depTx); err != nil {
		return fmt.Errorf("cannot deploy empty chaincode, %w", err)
	}
	client.codeAddr = depTx.Hash()
	return nil
}

func (client *EmptyClient) MakeDeploymentTx() *core.Transaction {
	input := client.nativeDeploymentInput()
	b, _ := json.Marshal(input)
	return core.NewTransaction().
		SetNonce(time.Now().UnixNano()).
		SetInput(b).
		Sign(client.signer)
}

func (client *EmptyClient) nativeDeploymentInput() *common.DeploymentInput {
	return &common.DeploymentInput{
		CodeInfo: common.CodeInfo{
			DriverType: common.DriverTypeNative,
			CodeID:     native.CodeEmpty,
		},
	}
}

func (client *EmptyClient) MakeTx() *core.Transaction {
	if client.codeAddr == nil {
		client.codeAddr = native.CodeEmpty
	}
	return core.NewTransaction().
		SetCodeAddr(client.codeAddr).
		SetNonce(time.Now().UnixNano()).
		SetInput([]byte(strconv.Itoa(rand.Intn(math.MaxInt)))).
		Sign(client.signer)
}

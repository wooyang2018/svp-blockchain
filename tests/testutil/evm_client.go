// Copyright (C) 2023 Chenrui
// Licensed under the GNU General Public License v3.0

package testutil

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wooyang2018/svp-blockchain/consensus"
	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/execution/evm"
	"github.com/wooyang2018/svp-blockchain/tests/cluster"
)

type EVMClient struct {
	contractPath       string
	contractCodeID     []byte
	contractUploadNode int
	signer             *core.PrivateKey
	accounts           []*core.PrivateKey
	cluster            *cluster.Cluster
	codeAddr           []byte
	nodes              []int
	invokeCount        int64
}

var _ LoadClient = (*EVMClient)(nil)

func NewEVMClient(nodes []int, account int, contractPath string) *EVMClient {
	client := &EVMClient{
		contractPath: contractPath,
		signer:       core.GenerateKey(nil),
		accounts:     make([]*core.PrivateKey, account),
		nodes:        nodes,
	}
	client.generateKeyConcurrent(client.accounts)
	return client
}

func (E *EVMClient) generateKeyConcurrent(keys []*core.PrivateKey) {
	jobs := make(chan int, 100)
	defer close(jobs)
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		go func() {
			for i := range jobs {
				keys[i] = core.GenerateKey(nil)
				wg.Done()
			}
		}()
	}
	for i := range keys {
		wg.Add(1)
		jobs <- i
	}
	wg.Wait()
}

func (E *EVMClient) SetupOnCluster(cls *cluster.Cluster) error {
	return E.setupOnCluster(cls)
}

func (E *EVMClient) SubmitTx() (int, *core.Transaction, error) {
	tx := E.makeRandomInvoke()
	nodeIdx, err := SubmitTx(E.cluster, E.nodes, tx)
	return nodeIdx, tx, err
}

func (E *EVMClient) BatchSubmitTx(num int) (int, *core.TxList, error) {
	jobCh := make(chan struct{}, num)
	defer close(jobCh)
	out := make(chan *core.Transaction, num)
	defer close(out)

	for i := 0; i < 100; i++ {
		go func(jobCh <-chan struct{}, out chan<- *core.Transaction) {
			for range jobCh {
				out <- E.makeRandomInvoke()
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
	nodeIdx, err := BatchSubmitTx(E.cluster, E.nodes, &txList)
	return nodeIdx, &txList, err
}

func (E *EVMClient) SubmitTxAndWait() (int, error) {
	return SubmitTxAndWait(E.cluster, E.makeRandomInvoke())
}

func (E *EVMClient) setupOnCluster(cls *cluster.Cluster) error {
	E.cluster = cls
	if consensus.ExecuteTxFlag {
		if err := E.deploy(); err != nil {
			return err
		}
	}
	time.Sleep(1 * time.Second)
	return nil //?
}

func (E *EVMClient) deploy() error {
	if E.contractPath != "" {
		i, codeID, err := uploadEVMChainCode(E.cluster, E.contractPath)
		if err != nil {
			return err
		}
		E.contractCodeID = codeID
		E.contractUploadNode = i
	}
	depTx := E.MakeDeploymentTx(E.signer)
	if _, err := SubmitTxAndWait(E.cluster, depTx); err != nil {
		return fmt.Errorf("cannot deploy EVM contract, %w", err)
	}
	E.codeAddr = depTx.Hash()
	return nil
}

func (E *EVMClient) MakeDeploymentTx(signer *core.PrivateKey) *core.Transaction {
	var input *common.DeploymentInput
	if E.contractCodeID != nil {
		input = E.evmDeploymentInput()
	}
	b, _ := json.Marshal(input)
	return core.NewTransaction().
		SetNonce(time.Now().UnixNano()).
		SetInput(b).
		Sign(signer)
}

func (E *EVMClient) evmDeploymentInput() *common.DeploymentInput {
	initInput := &evm.InitInput{
		Class: "Storage",
	}
	b, _ := json.Marshal(initInput)
	return &common.DeploymentInput{
		CodeInfo: common.CodeInfo{
			DriverType: common.DriverTypeEVM,
			CodeID:     E.contractCodeID,
		},
		InitInput: b,
		InstallData: []byte(fmt.Sprintf("%s/contract/%s",
			E.cluster.GetNode(E.contractUploadNode).GetEndpoint(),
			hex.EncodeToString(E.contractCodeID),
		)),
	}
}

func (E *EVMClient) makeRandomInvoke() *core.Transaction {
	tCount := int(atomic.AddInt64(&E.invokeCount, 1))
	accIdx := tCount % len(E.accounts)
	return E.MakeEVMInvokeTx(E.accounts[accIdx], 1)
}

func (E *EVMClient) MakeEVMInvokeTx(
	sender *core.PrivateKey, value uint64,
) *core.Transaction {
	input := &evm.Input{
		Method: "store",
		Params: []string{strconv.Itoa(int(value))},
		Types:  []string{"uint256"},
	}
	b, err := json.Marshal(input)
	common.Check2(err)
	return core.NewTransaction().
		SetCodeAddr(E.codeAddr).
		SetNonce(time.Now().UnixNano()).
		SetInput([]byte(b)).
		Sign(sender)
}

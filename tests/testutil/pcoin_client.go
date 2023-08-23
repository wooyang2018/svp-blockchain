// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package testutil

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wooyang2018/svp-blockchain/chaincode/pcoin"
	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution"
	"github.com/wooyang2018/svp-blockchain/tests/cluster"
)

type PCoinClient struct {
	binccPath       string
	minter          *core.PrivateKey
	accounts        []*core.PrivateKey
	dests           []*core.PrivateKey
	cluster         *cluster.Cluster
	binccCodeID     []byte
	binccUploadNode int
	codeAddr        []byte
	transferCount   int64
	nodes           []int
}

var _ LoadClient = (*PCoinClient)(nil)

// NewPCoinClient creates and setups a Load Service, submits chaincode deploy tx and waits for commission
func NewPCoinClient(nodes []int, mintCount, destCount int, binccPath string) *PCoinClient {
	client := &PCoinClient{
		binccPath: binccPath,
		minter:    core.GenerateKey(nil),
		accounts:  make([]*core.PrivateKey, mintCount),
		dests:     make([]*core.PrivateKey, destCount),
		nodes:     nodes,
	}
	client.generateKeyConcurrent(client.accounts)
	client.generateKeyConcurrent(client.dests)
	return client
}

func (client *PCoinClient) generateKeyConcurrent(keys []*core.PrivateKey) {
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

func (client *PCoinClient) SetupOnCluster(cls *cluster.Cluster) error {
	return client.setupOnCluster(cls)
}

func (client *PCoinClient) SubmitTxAndWait() (int, error) {
	return SubmitTxAndWait(client.cluster, client.makeRandomTransfer())
}

func (client *PCoinClient) SubmitTx() (int, *core.Transaction, error) {
	tx := client.makeRandomTransfer()
	nodeIdx, err := SubmitTx(client.cluster, client.nodes, tx)
	return nodeIdx, tx, err
}

func (client *PCoinClient) BatchSubmitTx(num int) (int, *core.TxList, error) {
	//使用100个协程快速生成num个交易
	jobCh := make(chan struct{}, num)
	defer close(jobCh)
	out := make(chan *core.Transaction, num)
	defer close(out)
	for i := 0; i < 100; i++ {
		go func(jobCh <-chan struct{}, out chan<- *core.Transaction) {
			for range jobCh {
				out <- client.makeRandomTransfer()
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

func (client *PCoinClient) setupOnCluster(cls *cluster.Cluster) error {
	client.cluster = cls
	if err := client.deploy(); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)
	return client.mintAccounts()
}

func (client *PCoinClient) deploy() error {
	if client.binccPath != "" {
		i, codeID, err := uploadBinChainCode(client.cluster, client.binccPath)
		if err != nil {
			return err
		}
		client.binccCodeID = codeID
		client.binccUploadNode = i
	}
	depTx := client.MakeDeploymentTx(client.minter)
	_, err := SubmitTxAndWait(client.cluster, depTx)
	if err != nil {
		return fmt.Errorf("cannot deploy pcoin %w", err)
	}
	client.codeAddr = depTx.Hash()
	return nil
}

func (client *PCoinClient) mintAccounts() error {
	errCh := make(chan error, len(client.accounts))
	for _, acc := range client.accounts {
		go func(acc *core.PublicKey) {
			errCh <- client.Mint(acc, 1000000000)
		}(acc.PublicKey())
	}
	for range client.accounts {
		err := <-errCh
		if err != nil {
			return err
		}
	}
	return nil
}

func (client *PCoinClient) Mint(dest *core.PublicKey, value int64) error {
	mintTx := client.MakeMintTx(dest, value)
	i, err := SubmitTxAndWait(client.cluster, mintTx)
	if err != nil {
		return fmt.Errorf("cannot mint pcoin %w", err)
	}
	balance, err := client.QueryBalance(client.cluster.GetNode(i), dest)
	if err != nil {
		return fmt.Errorf("cannot query pcoin balance %w", err)
	}
	if value != balance {
		return fmt.Errorf("incorrect balance %d %d", value, balance)
	}
	return nil
}

func (client *PCoinClient) makeRandomTransfer() *core.Transaction {
	tCount := int(atomic.AddInt64(&client.transferCount, 1))
	accIdx := tCount % len(client.accounts)
	destIdx := tCount % len(client.dests)
	return client.MakeTransferTx(client.accounts[accIdx],
		client.dests[destIdx].PublicKey(), 1)
}

func (client *PCoinClient) QueryBalance(node cluster.Node, dest *core.PublicKey) (int64, error) {
	result, err := QueryState(node, client.MakeBalanceQuery(dest))
	if err != nil {
		return 0, err
	}
	var balance int64
	return balance, json.Unmarshal(result, &balance)
}

func (client *PCoinClient) MakeDeploymentTx(minter *core.PrivateKey) *core.Transaction {
	input := client.nativeDeploymentInput()
	if client.binccCodeID != nil {
		input = client.binccDeploymentInput()
	}
	b, _ := json.Marshal(input)
	return core.NewTransaction().
		SetNonce(time.Now().UnixNano()).
		SetInput(b).
		Sign(minter)
}

func (client *PCoinClient) nativeDeploymentInput() *execution.DeploymentInput {
	return &execution.DeploymentInput{
		CodeInfo: execution.CodeInfo{
			DriverType: execution.DriverTypeNative,
			CodeID:     execution.NativeCodePCoin,
		},
	}
}

func (client *PCoinClient) binccDeploymentInput() *execution.DeploymentInput {
	return &execution.DeploymentInput{
		CodeInfo: execution.CodeInfo{
			DriverType: execution.DriverTypeBincc,
			CodeID:     client.binccCodeID,
		},
		InstallData: []byte(fmt.Sprintf("%s/bincc/%s",
			client.cluster.GetNode(client.binccUploadNode).GetEndpoint(),
			hex.EncodeToString(client.binccCodeID),
		)),
	}
}

func (client *PCoinClient) MakeMintTx(dest *core.PublicKey, value int64) *core.Transaction {
	input := &pcoin.Input{
		Method: "mint",
		Dest:   dest.Bytes(),
		Value:  value,
	}
	b, _ := json.Marshal(input)
	return core.NewTransaction().
		SetCodeAddr(client.codeAddr).
		SetNonce(time.Now().UnixNano()).
		SetInput(b).
		Sign(client.minter)
}

func (client *PCoinClient) MakeTransferTx(
	sender *core.PrivateKey, dest *core.PublicKey, value int64,
) *core.Transaction {
	input := &pcoin.Input{
		Method: "transfer",
		Dest:   dest.Bytes(),
		Value:  value,
	}
	b, _ := json.Marshal(input)
	return core.NewTransaction().
		SetCodeAddr(client.codeAddr).
		SetNonce(time.Now().UnixNano()).
		SetInput(b).
		Sign(sender)
}

func (client *PCoinClient) MakeBalanceQuery(dest *core.PublicKey) *execution.QueryData {
	input := &pcoin.Input{
		Method: "balance",
		Dest:   dest.Bytes(),
	}
	b, _ := json.Marshal(input)
	return &execution.QueryData{
		CodeAddr: client.codeAddr,
		Input:    b,
	}
}

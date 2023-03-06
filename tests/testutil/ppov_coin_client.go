// Copyright (C) 2021 Aung Maw
// Licensed under the GNU General Public License v3.0

package testutil

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wooyang2018/ppov-blockchain/chaincode/ppovcoin"
	"github.com/wooyang2018/ppov-blockchain/core"
	"github.com/wooyang2018/ppov-blockchain/execution"
	"github.com/wooyang2018/ppov-blockchain/tests/cluster"
)

type PPoVCoinClient struct {
	binccPath string

	minter   *core.PrivateKey
	accounts []*core.PrivateKey
	dests    []*core.PrivateKey

	cluster *cluster.Cluster

	binccCodeID     []byte
	binccUploadNode int

	codeAddr []byte

	transferCount int64
}

var _ LoadClient = (*PPoVCoinClient)(nil)

// create and setup a LoadService
// submit chaincode deploy tx and wait for commit
func NewPPoVCoinClient(mintCount, destCount int, binccPath string) *PPoVCoinClient {
	client := &PPoVCoinClient{
		binccPath: binccPath,
		minter:    core.GenerateKey(nil),
		accounts:  make([]*core.PrivateKey, mintCount),
		dests:     make([]*core.PrivateKey, destCount),
	}
	client.generateKeyConcurrent(client.accounts)
	client.generateKeyConcurrent(client.dests)
	return client
}

func (client *PPoVCoinClient) generateKeyConcurrent(keys []*core.PrivateKey) {
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

func (client *PPoVCoinClient) SetupOnCluster(cls *cluster.Cluster) error {
	return client.setupOnCluster(cls)
}

func (client *PPoVCoinClient) SubmitTxAndWait() (int, error) {
	return SubmitTxAndWait(client.cluster, client.makeRandomTransfer())
}

func (client *PPoVCoinClient) SubmitTx() (int, *core.Transaction, error) {
	tx := client.makeRandomTransfer()
	nodeIdx, err := SubmitTx(client.cluster, tx)
	return nodeIdx, tx, err
}

func (client *PPoVCoinClient) setupOnCluster(cls *cluster.Cluster) error {
	client.cluster = cls
	if err := client.deploy(); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)
	return client.mintAccounts()
}

func (client *PPoVCoinClient) deploy() error {
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
		return fmt.Errorf("cannot deploy ppovcoin %w", err)
	}
	client.codeAddr = depTx.Hash()
	return nil
}

func (client *PPoVCoinClient) mintAccounts() error {
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

func (client *PPoVCoinClient) Mint(dest *core.PublicKey, value int64) error {
	mintTx := client.MakeMintTx(dest, value)
	i, err := SubmitTxAndWait(client.cluster, mintTx)
	if err != nil {
		return fmt.Errorf("cannot mint ppovcoin %w", err)
	}
	balance, err := client.QueryBalance(client.cluster.GetNode(i), dest)
	if err != nil {
		return fmt.Errorf("cannot query ppovcoin balance %w", err)
	}
	if value != balance {
		return fmt.Errorf("incorrect balance %d %d", value, balance)
	}
	return nil
}

func (client *PPoVCoinClient) makeRandomTransfer() *core.Transaction {
	tCount := int(atomic.AddInt64(&client.transferCount, 1))
	accIdx := tCount % len(client.accounts)
	destIdx := tCount % len(client.dests)
	return client.MakeTransferTx(client.accounts[accIdx],
		client.dests[destIdx].PublicKey(), 1)
}

func (client *PPoVCoinClient) QueryBalance(node cluster.Node, dest *core.PublicKey) (int64, error) {
	result, err := QueryState(node, client.MakeBalanceQuery(dest))
	if err != nil {
		return 0, err
	}
	var balance int64
	return balance, json.Unmarshal(result, &balance)
}

func (client *PPoVCoinClient) MakeDeploymentTx(minter *core.PrivateKey) *core.Transaction {
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

func (client *PPoVCoinClient) nativeDeploymentInput() *execution.DeploymentInput {
	return &execution.DeploymentInput{
		CodeInfo: execution.CodeInfo{
			DriverType: execution.DriverTypeNative,
			CodeID:     execution.NativeCodeIDPPoVCoin,
		},
	}
}

func (client *PPoVCoinClient) binccDeploymentInput() *execution.DeploymentInput {
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

func (client *PPoVCoinClient) MakeMintTx(dest *core.PublicKey, value int64) *core.Transaction {
	input := &ppovcoin.Input{
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

func (client *PPoVCoinClient) MakeTransferTx(
	sender *core.PrivateKey, dest *core.PublicKey, value int64,
) *core.Transaction {
	input := &ppovcoin.Input{
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

func (client *PPoVCoinClient) MakeBalanceQuery(dest *core.PublicKey) *execution.QueryData {
	input := &ppovcoin.Input{
		Method: "balance",
		Dest:   dest.Bytes(),
	}
	b, _ := json.Marshal(input)
	return &execution.QueryData{
		CodeAddr: client.codeAddr,
		Input:    b,
	}
}

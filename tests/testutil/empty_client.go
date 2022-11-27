package testutil

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/wooyang2018/ppov-blockchain/core"
	"github.com/wooyang2018/ppov-blockchain/execution"
	"github.com/wooyang2018/ppov-blockchain/tests/cluster"
)

type EmptyClient struct {
	signer   *core.PrivateKey
	cluster  *cluster.Cluster
	codeAddr []byte
}

var _ LoadClient = (*EmptyClient)(nil)

func NewEmptyClient() *EmptyClient {
	return &EmptyClient{
		signer: core.GenerateKey(nil),
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
	nodeIdx, err := SubmitTx(client.cluster, tx)
	return nodeIdx, tx, err
}

func (client *EmptyClient) setupOnCluster(cls *cluster.Cluster) error {
	client.cluster = cls
	if err := client.deploy(); err != nil {
		return err
	}
	return nil
}

func (client *EmptyClient) deploy() error {
	depTx := client.MakeDeploymentTx(client.signer)
	_, err := SubmitTxAndWait(client.cluster, depTx)
	if err != nil {
		return fmt.Errorf("cannot deploy empty chaincode %w", err)
	}
	client.codeAddr = depTx.Hash()
	return nil
}

func (client *EmptyClient) MakeDeploymentTx(minter *core.PrivateKey) *core.Transaction {
	input := client.nativeDeploymentInput()
	b, _ := json.Marshal(input)
	return core.NewTransaction().
		SetNonce(time.Now().UnixNano()).
		SetInput(b).
		Sign(minter)
}

func (client *EmptyClient) nativeDeploymentInput() *execution.DeploymentInput {
	return &execution.DeploymentInput{
		CodeInfo: execution.CodeInfo{
			DriverType: execution.DriverTypeNative,
			CodeID:     execution.NativeCodeIDEmpty,
		},
	}
}

func (client *EmptyClient) MakeTx() *core.Transaction {
	return core.NewTransaction().
		SetCodeAddr(client.codeAddr).
		SetNonce(time.Now().UnixNano()).
		Sign(client.signer)
}
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/wooyang2018/svp-blockchain/chaincode/kvdb"
	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution"
	"github.com/wooyang2018/svp-blockchain/node"
)

type KVDBClient struct {
	signer   *core.PrivateKey
	codeAddr []byte
}

func NewKVDBClient(dataPath string, isDeploy bool) *KVDBClient {
	var err error
	client := &KVDBClient{}
	if !isDeploy {
		if client.codeAddr, err = os.ReadFile(path.Join(dataPath, FileCodeAddr)); err != nil {
			panic(err)
		}
	}
	if client.signer, err = node.ReadNodeKey(dataPath); err != nil {
		panic(err)
	}
	return client
}

func (client *KVDBClient) SubmitTx(tx *core.Transaction) {
	b, err := json.Marshal(tx)
	if err != nil {
		panic(err)
	}
	resp, err := http.Post(nodeUrl+"/transactions",
		"application/json", bytes.NewReader(b))
	if err != nil {
		panic(err)
	}
	msg, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("status code %d, body %s\n", resp.StatusCode, string(msg))
}

func (client *KVDBClient) QueryState(query *execution.QueryData) (ret []byte) {
	b, err := json.Marshal(query)
	if err != nil {
		panic(err)
	}
	resp, err := http.Post(nodeUrl+"/querystate",
		"application/json", bytes.NewReader(b))
	if err != nil || resp.StatusCode != 200 {
		panic("query state failed")
	}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&ret)
	return
}

func (client *KVDBClient) MakeTx(input *kvdb.Input) *core.Transaction {
	b, _ := json.Marshal(input)
	return core.NewTransaction().
		SetCodeAddr(client.codeAddr).
		SetNonce(time.Now().UnixNano()).
		SetInput(b).
		Sign(client.signer)
}

func (client *KVDBClient) MakeDeploymentTx() *core.Transaction {
	input := &execution.DeploymentInput{
		CodeInfo: execution.CodeInfo{
			DriverType: execution.DriverTypeNative,
			CodeID:     execution.NativeCodeKVDB,
		},
	}
	b, _ := json.Marshal(input)
	return core.NewTransaction().
		SetNonce(time.Now().UnixNano()).
		SetInput(b).
		Sign(client.signer)
}

func (client *KVDBClient) MakeQuery(input *kvdb.Input) *execution.QueryData {
	b, _ := json.Marshal(input)
	return &execution.QueryData{
		CodeAddr: client.codeAddr,
		Input:    b,
	}
}

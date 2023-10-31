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

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution"
	"github.com/wooyang2018/svp-blockchain/native"
	"github.com/wooyang2018/svp-blockchain/native/kvdb"
	"github.com/wooyang2018/svp-blockchain/node"
)

type Client struct {
	signer   *core.PrivateKey
	codeAddr []byte
}

func NewClient(dataPath string, isDeploy bool) *Client {
	var err error
	client := &Client{}
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

func (client *Client) SubmitTx(tx *core.Transaction) {
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

func (client *Client) QueryState(query *execution.QueryData) (ret []byte) {
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

func (client *Client) MakeTx(input *kvdb.Input) *core.Transaction {
	b, _ := json.Marshal(input)
	return core.NewTransaction().
		SetCodeAddr(client.codeAddr).
		SetNonce(time.Now().UnixNano()).
		SetInput(b).
		Sign(client.signer)
}

func (client *Client) MakeDeploymentTx() *core.Transaction {
	input := &execution.DeploymentInput{
		CodeInfo: execution.CodeInfo{
			DriverType: execution.DriverTypeNative,
			CodeID:     native.CodeKVDB,
		},
	}
	b, _ := json.Marshal(input)
	return core.NewTransaction().
		SetNonce(time.Now().UnixNano()).
		SetInput(b).
		Sign(client.signer)
}

func (client *Client) MakeQuery(input *kvdb.Input) *execution.QueryData {
	b, _ := json.Marshal(input)
	return &execution.QueryData{
		CodeAddr: client.codeAddr,
		Input:    b,
	}
}

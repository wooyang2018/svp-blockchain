// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package native

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
)

type Client struct {
	signer   *core.PrivateKey
	codeAddr []byte
}

func NewClient(isDeploy bool) *Client {
	var err error
	client := &Client{}
	if !isDeploy {
		client.codeAddr, err = os.ReadFile(path.Join(DataPath, CodeFile))
		check(err)
	}
	b, err := os.ReadFile(path.Join(DataPath, FileNodekey))
	check(err)
	client.signer, _ = core.NewPrivateKey(b)
	return client
}

func (client *Client) SubmitTx(tx *core.Transaction) {
	b, err := json.Marshal(tx)
	check(err)
	resp, err := http.Post(NodeUrl+"/transactions",
		"application/json", bytes.NewReader(b))
	check(err)
	msg, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("status code %d, %s\n", resp.StatusCode, string(msg))
}

func (client *Client) QueryState(input []byte) (ret []byte) {
	query := &common.QueryData{
		CodeAddr: client.codeAddr,
		Input:    input,
	}
	b, err := json.Marshal(query)
	check(err)
	resp, err := http.Post(NodeUrl+"/querystate", "application/json", bytes.NewReader(b))
	if err != nil || resp.StatusCode != 200 {
		log.Fatal("query state failed")
	}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&ret)
	return
}

func (client *Client) MakeTx(input []byte) *core.Transaction {
	return core.NewTransaction().
		SetCodeAddr(client.codeAddr).
		SetNonce(time.Now().UnixNano()).
		SetInput(input).
		Sign(client.signer)
}

func (client *Client) MakeDeploymentTx(codeID []byte) *core.Transaction {
	input := &common.DeploymentInput{
		CodeInfo: common.CodeInfo{
			DriverType: common.DriverTypeNative,
			CodeID:     codeID,
		},
	}
	b, _ := json.Marshal(input)
	return core.NewTransaction().
		SetNonce(time.Now().UnixNano()).
		SetInput(b).
		Sign(client.signer)
}

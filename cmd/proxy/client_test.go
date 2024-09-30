// Copyright (C) 2024 Wooyang2018
// Licensed under the GNU General Public License v3.0

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/execution/evm"
	"github.com/wooyang2018/svp-blockchain/native"
)

const ProxyUrl = "http://localhost:8080"

func checkLiveness() bool {
	resp, err := http.Get(ProxyUrl + "/setup/cluster/liveness")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func oneClickStart() error {
	params := FactoryParams{
		NodeCount:  4,
		StakeQuota: 9999,
		WindowSize: 4,
	}
	b, _ := json.Marshal(params)
	resp, err := http.Post(ProxyUrl+"/setup/oneclick", "application/json", bytes.NewReader(b))
	retErr := common.CheckResponse(resp, err)
	if retErr == nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		time.Sleep(5 * time.Second)
		return nil
	}
	return retErr
}

func getNode0Key() (*core.PrivateKey, error) {
	resp, err := http.Get(ProxyUrl + "/workdir/0/nodekey")
	retErr := common.CheckResponse(resp, err)
	if retErr != nil {
		return nil, retErr
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return core.NewPrivateKey(b)
}

func prepareClient(t *testing.T, filePath string) (*native.Client, []byte) {
	if !checkLiveness() {
		if oneClickStart() != nil || !checkLiveness() {
			t.Skip("enable this test by running chain-proxy container")
		}
	}

	asrt := assert.New(t)
	native.SetNodeUrl(ProxyUrl + "/proxy/0")
	client := &native.Client{}
	signer, err := getNode0Key()
	asrt.NoError(err)
	client.SetSigner(signer)

	codeID, err := native.UploadChainCode(common.DriverTypeEVM, filePath)
	asrt.NoError(err)
	return client, codeID
}

func TestEVMClient(t *testing.T) {
	client, codeID := prepareClient(t, "../../evm/testdata/contracts/Storage.sol")
	initInput := &evm.InitInput{Class: "Storage"}
	b, _ := json.Marshal(initInput)
	tx := client.MakeDeploymentTx(common.DriverTypeEVM, codeID, b)
	client.SubmitTxAndWait(tx)
	client.SetAddr(tx.Hash())

	input := &evm.Input{
		Method: "store",
		Params: []string{"1024"},
		Types:  []string{"uint256"},
	}
	b, _ = json.Marshal(input)
	tx = client.MakeTx(b)
	client.SubmitTxAndWait(tx)

	input = &evm.Input{Method: "retrieve"}
	b, _ = json.Marshal(input)
	ret := client.QueryState(b)
	var res []interface{}
	json.Unmarshal(ret, &res)
	assert.EqualValues(t, big.NewInt(1024), big.NewInt(int64(res[0].(float64))))
}

func TestEVMDataTypes(t *testing.T) {
	client, codeID := prepareClient(t, "../../evm/testdata/contracts/Types.sol")
	encoded := base64.StdEncoding.EncodeToString([]byte("hello world"))
	initInput := &evm.InitInput{
		Class: "DataTypes",
		Params: []string{"256", "hello world", "32", "16",
			"d04bee43b17d50bd4d9888bfece21ce808b14707",
			encoded, "true", "256,128", "64,32,16"},
		Types: []string{"uint256", "string", "uint32", "uint16",
			"address", "bytes", "bool", "uint256[2]", "uint32[]"},
	}
	b, _ := json.Marshal(initInput)
	tx := client.MakeDeploymentTx(common.DriverTypeEVM, codeID, b)
	client.SubmitTxAndWait(tx)
	client.SetAddr(tx.Hash())

	input := &evm.Input{Method: "getAllValues"}
	b, _ = json.Marshal(input)
	ret := client.QueryState(b)
	var res []interface{}
	json.Unmarshal(ret, &res)
	t.Logf("%+v\n", res)
	decoded, _ := base64.StdEncoding.DecodeString(res[5].(string))
	assert.EqualValues(t, []byte("hello world"), decoded)

	input = &evm.Input{
		Method: "updateValues",
		Params: []string{"512", "hello world", "32", "16",
			"d04bee43b17d50bd4d9888bfece21ce808b14707",
			encoded, "false", "128,128", "32,16,8"},
		Types: []string{"uint256", "string", "uint32", "uint16",
			"address", "bytes", "bool", "uint256[2]", "uint32[]"},
	}
	b, _ = json.Marshal(input)
	tx = client.MakeTx(b)
	client.SubmitTxAndWait(tx)

	input = &evm.Input{Method: "getAllValues"}
	b, _ = json.Marshal(input)
	ret = client.QueryState(b)
	json.Unmarshal(ret, &res)
	t.Logf("%+v\n", res)
}

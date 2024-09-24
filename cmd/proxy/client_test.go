package main

import (
	"bytes"
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

func TestEVMClient(t *testing.T) {
	if !checkLiveness() {
		if oneClickStart() != nil || !checkLiveness() {
			t.Skip("enable this test by running chain-proxy container")
		}
	}

	asrt := assert.New(t)
	native.SetNodeUrl(ProxyUrl + "/proxy/0")
	client := native.Client{}
	signer, err := getNode0Key()
	asrt.NoError(err)
	client.SetSigner(signer)

	filePath := "../../evm/testdata/contracts/Storage.sol"
	codeID, err := native.UploadChainCode(common.DriverTypeEVM, filePath)
	asrt.NoError(err)

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
	asrt.EqualValues(big.NewInt(1024), big.NewInt(int64(res[0].(float64))))
}

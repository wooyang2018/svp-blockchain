// Copyright (C) 2024 Wooyang2018
// Licensed under the GNU General Public License v3.0

package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/execution/evm"
	"github.com/wooyang2018/svp-blockchain/native"
)

var client *native.Client
var codeID []byte

func getLocalNode0Key(nodeID int) (*core.PrivateKey, error) {
	workDir := path.Join(WorkDir, ClusterName)
	nodeKeyPath := path.Join(workDir, strconv.Itoa(nodeID), native.FileNodekey)
	b, err := os.ReadFile(nodeKeyPath)
	if err != nil {
		return nil, err
	}
	return core.NewPrivateKey(b)
}

func newClientHandler(c *gin.Context) {
	nodeID, err := paramNodeID(c)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	native.SetNodeUrl(fmt.Sprintf("http://127.0.0.1:%d", 9090+nodeID))
	client = &native.Client{}
	if signer, err := getLocalNode0Key(nodeID); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	} else {
		client.SetSigner(signer)
		c.String(http.StatusOK, "successfully newed transaction client")
	}
}

func uploadBinCodeHandler(c *gin.Context) {
	uploadChainCode(c, common.DriverTypeBincc)
}

func uploadContractHandler(c *gin.Context) {
	uploadChainCode(c, common.DriverTypeEVM)
}

func uploadChainCode(c *gin.Context, driverType common.DriverType) {
	fh, err := c.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, "get uploaded file error:%+v", err)
		return
	}
	f, err := fh.Open()
	if err != nil {
		c.String(http.StatusBadRequest, "open multipart file error:%+v", err)
		return
	}
	defer f.Close()
	codeID, err = native.UploadChainCode2(driverType, f)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	} else {
		c.String(http.StatusOK, "successfully uploaded chaincode %s", hex.EncodeToString(codeID))
	}
}

func isContractReady(c *gin.Context, isDeploy bool) bool {
	if client == nil {
		c.String(http.StatusBadRequest, "please new a transaction client first")
		return false
	}
	if codeID == nil {
		c.String(http.StatusBadRequest, "please upload an evm contract first")
		return false
	}
	if !isDeploy && client.GetAddr() == nil {
		c.String(http.StatusBadRequest, "please deploy the uploaded contract first")
		return false
	}
	return true
}

func deployContractHandler(c *gin.Context) {
	if !isContractReady(c, true) {
		return
	}
	initInput := new(evm.InitInput)
	if err := c.ShouldBind(initInput); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	b, _ := json.Marshal(initInput)
	tx := client.MakeDeploymentTx(common.DriverTypeEVM, codeID, b)
	client.SubmitTxAndWait(tx)
	client.SetAddr(tx.Hash())
	c.JSON(http.StatusOK, gin.H{"message": "successfully deployed contract", "transaction": tx})
}

func invokeContractHandler(c *gin.Context) {
	if !isContractReady(c, false) {
		return
	}
	input := new(evm.Input)
	if err := c.ShouldBind(input); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	b, _ := json.Marshal(input)
	tx := client.MakeTx(b)
	client.SubmitTxAndWait(tx)
	c.JSON(http.StatusOK, gin.H{"message": "successfully invoked contract", "transaction": tx})
}

func queryContractHandler(c *gin.Context) {
	if !isContractReady(c, false) {
		return
	}
	input := new(evm.Input)
	if err := c.ShouldBind(input); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	b, _ := json.Marshal(input)
	ret := client.QueryState(b)
	c.JSON(http.StatusOK, gin.H{"message": "successfully queried contract", "result": string(ret)})
}

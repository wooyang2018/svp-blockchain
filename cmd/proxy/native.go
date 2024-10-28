// Copyright (C) 2024 Wooyang2018
// Licensed under the GNU General Public License v3.0

package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/native"
	"github.com/wooyang2018/svp-blockchain/native/pcoin"
	"github.com/wooyang2018/svp-blockchain/native/taddr"
	"github.com/wooyang2018/svp-blockchain/native/xcoin"
)

func getLocalCodeAddr(file string) ([]byte, error) {
	workDir := path.Join(WorkDir, ClusterName, "0")
	codeAddrPath := path.Join(workDir, native.CodePathDefault, file)
	return os.ReadFile(codeAddrPath)
}

func isNativeReady(c *gin.Context, needAddr bool) bool {
	if client == nil {
		c.String(http.StatusBadRequest, "please new a transaction client first")
		return false
	}
	if needAddr && client.GetAddr() == nil {
		c.String(http.StatusBadRequest, "please deploy a native chaincode first")
		return false
	}
	return true
}

func paramPCoinInput(c *gin.Context) (input *pcoin.Input, err error) {
	var temp struct {
		Method string `json:"method"`
		Dest   string `json:"dest"`
		Value  uint64 `json:"value"`
	}
	if err = c.ShouldBindJSON(&temp); err != nil {
		return nil, err
	}
	input = &pcoin.Input{
		Method: temp.Method,
		Value:  temp.Value,
	}
	if temp.Dest != "" {
		input.Dest, err = common.AddressToBytes(temp.Dest)
	}
	return input, err
}

func queryPCoinHandler(c *gin.Context) {
	if !isNativeReady(c, true) {
		return
	}
	input, err := paramPCoinInput(c)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	b, _ := json.Marshal(input)
	state := client.QueryState(b)
	result := common.AddressToString(state)
	if result == "" {
		temp := common.DecodeBalance(state)
		result = strconv.FormatUint(temp, 10)
	}
	c.JSON(http.StatusOK, gin.H{"message": "successfully queried native pcoin", "result": result})
}

func invokePCoinHandler(c *gin.Context) {
	if !isNativeReady(c, true) {
		return
	}
	input, err := paramPCoinInput(c)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	b, _ := json.Marshal(input)
	tx := client.MakeTx(b)
	client.SubmitTxAndWait(tx)
	c.JSON(http.StatusOK, gin.H{"message": "successfully invoked native pcoin", "transaction": tx})
}

func deployPCoinHandler(c *gin.Context) {
	if !isNativeReady(c, false) {
		return
	}
	tx := client.MakeDeploymentTx(common.DriverTypeNative, native.CodePCoin, nil)
	client.SubmitTxAndWait(tx)
	client.SetAddr(tx.Hash())
	c.JSON(http.StatusOK, gin.H{"message": "successfully deployed native pcoin", "transaction": tx})
}

func paramTAdrrInput(c *gin.Context) (*taddr.Input, error) {
	var temp struct {
		Method string `json:"method"`
		Addr   string `json:"addr"`
	}
	if err := c.ShouldBindJSON(&temp); err != nil {
		return nil, err
	}
	destBytes, err := common.AddressToBytes(temp.Addr)
	if err != nil {
		return nil, err
	}
	return &taddr.Input{
		Method: temp.Method,
		Addr:   destBytes,
	}, nil
}

func queryTAddrHandler(c *gin.Context) {
	if !isNativeReady(c, false) {
		return
	}

	input, err := paramTAdrrInput(c)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	addr, err := getLocalCodeAddr(native.FileCodeTAddr)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	client.SetAddr(addr)

	rawBytes, _ := json.Marshal(input)
	result := common.AddressToString(client.QueryState(rawBytes))
	c.JSON(http.StatusOK, gin.H{"message": "successfully queried native taddr", "result": result})
}

func paramXCoinInput(c *gin.Context) (*xcoin.Input, error) {
	var temp struct {
		Method string `json:"method"`
		Dest   string `json:"dest"`
		Value  uint64 `json:"value"`
	}
	if err := c.ShouldBindJSON(&temp); err != nil {
		return nil, err
	}
	destBytes, err := common.AddressToBytes(temp.Dest)
	if err != nil {
		return nil, err
	}
	return &xcoin.Input{
		Method: temp.Method,
		Dest:   destBytes,
		Value:  temp.Value,
	}, nil
}

func queryXCoinHandler(c *gin.Context) {
	if !isNativeReady(c, false) {
		return
	}

	input, err := paramXCoinInput(c)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	addr, err := getLocalCodeAddr(native.FileCodeXCoin)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	client.SetAddr(addr)

	rawBytes, _ := json.Marshal(input)
	result := common.DecodeBalance(client.QueryState(rawBytes))
	c.JSON(http.StatusOK, gin.H{"message": "successfully queried native xcoin", "result": result})
}

func invokeXCoinHandler(c *gin.Context) {
	if !isNativeReady(c, false) {
		return
	}

	input, err := paramXCoinInput(c)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	addr, err := getLocalCodeAddr(native.FileCodeXCoin)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	client.SetAddr(addr)

	rawBytes, _ := json.Marshal(input)
	tx := client.MakeTx(rawBytes)
	client.SubmitTxAndWait(tx)
	c.JSON(http.StatusOK, gin.H{"message": "successfully invoked native xcoin", "transaction": tx})
}

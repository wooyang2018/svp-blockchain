// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package node

import (
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/logger"
)

type nodeAPI struct {
	node *Node
}

func serveNodeAPI(node *Node) {
	api := &nodeAPI{node}

	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/consensus", api.getConsensusStatus)
	r.GET("/txpool", api.getTxPoolStatus)
	r.POST("/transactions", api.submitTx)
	r.POST("/transactions/batch", api.batchSubmitTxs)
	r.GET("/transactions/:hash/status", api.getTxStatus)
	r.GET("/transactions/:hash/commit", api.getTxCommit)
	r.GET("/blocks/:hash", api.getBlock)
	r.GET("/blocks/height/:height", api.getBlockByHeight)
	r.POST("/querystate", api.queryState)
	r.POST("/bincc", api.uploadBinChainCode)
	r.POST("/contract", api.uploadContractCode)
	r.Static("/bincc", node.config.ExecutionConfig.BinccDir)
	r.Static("/contract", node.config.ExecutionConfig.ContractDir)
	go func() {
		if err := r.Run(fmt.Sprintf(":%d", node.config.APIPort)); err != nil {
			logger.I().Fatalf("failed to start api %+v", err)
		}
	}()
}

func (api *nodeAPI) getConsensusStatus(c *gin.Context) {
	c.JSON(http.StatusOK, api.node.consensus.GetStatus())
}

func (api *nodeAPI) getTxPoolStatus(c *gin.Context) {
	c.JSON(http.StatusOK, api.node.txpool.GetStatus())
}

func (api *nodeAPI) submitTx(c *gin.Context) {
	tx := core.NewTransaction()
	if err := c.ShouldBind(tx); err != nil {
		c.String(http.StatusBadRequest, "cannot parse tx")
		return
	}
	if err := api.node.txpool.SubmitTx(tx); err != nil {
		logger.I().Warnf("submit tx failed, %+v", err)
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.String(http.StatusOK, "transaction accepted")
}

func (api *nodeAPI) batchSubmitTxs(c *gin.Context) {
	txs := core.NewTxList()
	if err := c.ShouldBind(txs); err != nil {
		c.String(http.StatusBadRequest, "cannot parse txs")
		return
	}
	if err := api.node.txpool.StoreTxs(txs); err != nil {
		logger.I().Warnf("store txs failed %+v", err)
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.String(http.StatusOK, "transactions accepted")
}

func (api *nodeAPI) queryState(c *gin.Context) {
	query := new(common.QueryData)
	if err := c.ShouldBind(query); err != nil {
		c.String(http.StatusBadRequest, "cannot parse request")
		return
	}
	result, err := api.node.execution.Query(query)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, result)
}

func (api *nodeAPI) getTxStatus(c *gin.Context) {
	hash, err := api.getHash(c)
	if err != nil {
		c.String(http.StatusBadRequest, "cannot parse request")
		return
	}
	status := api.node.txpool.GetTxStatus(hash)
	c.JSON(http.StatusOK, status)
}

func (api *nodeAPI) getTxCommit(c *gin.Context) {
	hash, err := api.getHash(c)
	if err != nil {
		c.String(http.StatusBadRequest, "cannot parse hash")
		return
	}
	txc, err := api.node.storage.GetTxCommit(hash)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, txc)
}

func (api *nodeAPI) getBlock(c *gin.Context) {
	hash, err := api.getHash(c)
	if err != nil {
		c.String(http.StatusBadRequest, "cannot parse hash")
		return
	}
	blk, err := api.node.GetBlock(hash)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, blk)
}

func (api *nodeAPI) getHash(c *gin.Context) ([]byte, error) {
	hash := c.Param("hash")
	return hex.DecodeString(hash)
}

func (api *nodeAPI) getBlockByHeight(c *gin.Context) {
	hstr := c.Param("height")
	height, err := strconv.ParseUint(hstr, 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "cannot parse height")
		return
	}
	blk, err := api.node.storage.GetBlockByHeight(height)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, blk)
}

func (api *nodeAPI) uploadBinChainCode(c *gin.Context) {
	uploadCode(c, api.node.config.ExecutionConfig.BinccDir)
}

func (api *nodeAPI) uploadContractCode(c *gin.Context) {
	uploadCode(c, api.node.config.ExecutionConfig.ContractDir)
}

func uploadCode(c *gin.Context, path string) {
	fh, err := c.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, "cannot get uploaded file")
		return
	}
	f, err := fh.Open()
	if err != nil {
		c.String(http.StatusBadRequest, "cannot open multipart file")
		return
	}
	defer f.Close()
	codeID, err := common.StoreCode(path, f)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, codeID)
}

// Copyright (C) 2024 Wooyang2018
// Licensed under the GNU General Public License v3.0

package main

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/multiformats/go-multiaddr"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/node"
	"github.com/wooyang2018/svp-blockchain/tests/cluster"
)

const (
	GinAddr     = ":8080"
	WorkDir     = "./workdir"
	ClusterName = "cluster_template"
)

var (
	params  *FactoryParams
	factory *cluster.LocalFactory
	config  ConfigFiles
	cls     *cluster.Cluster
)

type FactoryParams struct {
	NodeCount  int `json:"nodeCount"`
	StakeQuota int `json:"stakeQuota"`
	WindowSize int `json:"windowSize"`
}

type ConfigFiles struct {
	keys       []*core.PrivateKey
	quotas     []uint64
	pointAddrs []multiaddr.Multiaddr
	topicAddrs []multiaddr.Multiaddr
}

func proxyHandler(c *gin.Context) {
	nodeID, err := strconv.Atoi(c.Param("node"))
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	if nodeID == -1 {
		nodeID = rand.Intn(params.NodeCount)
	}
	if nodeID < 0 || nodeID >= params.NodeCount {
		c.String(http.StatusBadRequest, fmt.Sprintf("only support nodes from 0 to %d to proxy", params.NodeCount-1))
	}

	path := c.Param("path")
	if path == "/" {
		c.String(http.StatusBadRequest, "path can not be empty")
		return
	}

	targetURL := fmt.Sprintf("http://localhost:%d%s", 9090+nodeID, path)
	fmt.Println("proxy http request:", targetURL)
	req, err := http.NewRequest(c.Request.Method, targetURL, c.Request.Body)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	req.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.String(http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	} else {
		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
	}
}

func commandHandler(c *gin.Context) {
	cmd := c.PostForm("cmd")
	if cmd == "" {
		c.String(http.StatusBadRequest, "command can not be empty")
		return
	}
	fmt.Println("execute command:", cmd)
	command := exec.Command("sh", "-c", cmd)
	output, err := command.CombinedOutput()
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("execute command error:%+v", err))
	} else {
		c.String(http.StatusOK, string(output))
	}
}

func clusterFactoryHandler(c *gin.Context) {
	params = new(FactoryParams)
	if err := c.ShouldBind(params); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	factory = cluster.NewLocalFactory(cluster.LocalFactoryParams{
		BinPath:     "./chain",
		WorkDir:     WorkDir,
		NodeCount:   params.NodeCount,
		StakeQuota:  params.StakeQuota,
		WindowSize:  params.WindowSize,
		SetupDocker: false,
		NodeConfig:  node.DefaultConfig,
	})
	err := os.RemoveAll(WorkDir)
	err = os.MkdirAll(factory.TemplateDir(), 0755)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	} else {
		c.String(http.StatusOK, "successfully created cluster factory.")
	}
}

func localAddrsHandler(c *gin.Context) {
	pointAddrs, topicAddrs, err := factory.MakeLocalAddrs()
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	config.pointAddrs = pointAddrs
	config.topicAddrs = topicAddrs
	c.JSON(http.StatusOK, gin.H{"point addrs": pointAddrs, "topic addrs": topicAddrs})
}

func randomKeysHandler(c *gin.Context) {
	config.keys = cluster.MakeRandomKeys(params.NodeCount)
	config.quotas = cluster.MakeRandomQuotas(params.NodeCount, params.StakeQuota)
	keyStrs := make([]string, params.NodeCount)
	for i, key := range config.keys {
		keyStrs[i] = key.PublicKey().String()
	}
	c.JSON(http.StatusOK, gin.H{"validator keys": keyStrs, "stake quotas": config.quotas})
}

func templateDirHandler(c *gin.Context) {
	genesis := &node.Genesis{
		Validators:  make([]string, params.NodeCount),
		StakeQuotas: make([]uint64, params.NodeCount),
		WindowSize:  params.WindowSize,
	}
	for i, v := range config.keys {
		genesis.Validators[i] = v.PublicKey().String()
		genesis.StakeQuotas[i] = config.quotas[i]
	}
	peers := cluster.MakePeers(config.keys, config.pointAddrs, config.topicAddrs)
	err := cluster.SetupTemplateDir(factory.TemplateDir(), config.keys, genesis, peers)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	} else {
		c.String(http.StatusOK, "successfully setup template dir")
	}
}

func buildChainHandler(c *gin.Context) {
	cmd := exec.Command("go", "build", "./cmd/chain")
	fmt.Printf(" $ %s\n", strings.Join(cmd.Args, " "))
	if err := cmd.Run(); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	} else {
		c.String(http.StatusOK, "successfully build chain")
	}
}

func createClusterHandler(c *gin.Context) {
	var err error
	if cls, err = factory.SetupCluster(ClusterName); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	}
	cls.Stop() // to make sure no existing process keeps running
	if err = cls.Start(); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	} else {
		time.Sleep(10 * time.Second)
		c.String(http.StatusOK, "started cluster")
	}
}

func startClusterHandler(c *gin.Context) {
	cls.Stop() // to make sure no existing process keeps running
	if err := cls.Start(); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	} else {
		time.Sleep(10 * time.Second)
		c.String(http.StatusOK, "started cluster")
	}
}

func stopClusterHandler(c *gin.Context) {
	cls.Stop()
	c.String(http.StatusOK, "stopped cluster")
}

func main() {
	gin.SetMode(gin.DebugMode)
	r := gin.Default()

	r.Any("/proxy/:node/*path", proxyHandler)
	r.POST("/execute", commandHandler)

	r.POST("/setup/factory", clusterFactoryHandler)
	r.POST("/setup/addrs", localAddrsHandler)
	r.POST("/setup/random", randomKeysHandler)
	r.POST("/setup/template", templateDirHandler)
	r.POST("/setup/build/chain", buildChainHandler)
	r.POST("/setup/cluster/create", createClusterHandler)
	r.POST("/setup/cluster/start", startClusterHandler)
	r.POST("/setup/cluster/stop", stopClusterHandler)

	r.Static("/workdir", path.Join(WorkDir, ClusterName))
	common.Check2(r.Run(GinAddr))
}

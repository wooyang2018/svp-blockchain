// Copyright (C) 2024 Wooyang2018
// Licensed under the GNU General Public License v3.0

package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wooyang2018/svp-blockchain/node"
	"github.com/wooyang2018/svp-blockchain/tests/cluster"
	"github.com/wooyang2018/svp-blockchain/tests/testutil"
)

func oneClickHandler(c *gin.Context) {
	handlers := []gin.HandlerFunc{
		clusterFactoryHandler, resetWorkDirHandler,
		localAddrsHandler, randomKeysHandler,
		templateDirHandler, buildChainHandler,
		newClusterHandler, startClusterHandler,
	}
	for _, handler := range handlers {
		handler(c)
		if c.Writer.Status() == http.StatusOK {
			c.String(http.StatusOK, "\n")
		} else {
			break
		}
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
	c.String(http.StatusOK, "successfully newed cluster factory")
}

func resetWorkDirHandler(c *gin.Context) {
	err := os.RemoveAll(WorkDir)
	err = os.MkdirAll(factory.TemplateDir(), 0755)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	} else {
		c.String(http.StatusOK, "successfully reset working directory")
	}
}

func resetStatusHandler(c *gin.Context) {
	cmd := exec.Command("pkill", "-9 -f", "^./chain")
	fmt.Printf(" $ %s\n", strings.Join(cmd.Args, " "))
	cmd.Run()
	c.String(http.StatusOK, "successfully reset global variables and blockchain processes")

	params = nil
	factory = nil
	config = ConfigFiles{}
	cls = nil
	client = nil
	codeID = nil
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
		c.String(http.StatusOK, "successfully setup template directory")
	}
}

func buildChainHandler(c *gin.Context) {
	cmd := exec.Command("go", "build", "./cmd/chain")
	fmt.Printf(" $ %s\n", strings.Join(cmd.Args, " "))
	if err := cmd.Run(); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	} else {
		c.String(http.StatusOK, "successfully built chain binary file")
	}
}

func newClusterHandler(c *gin.Context) {
	var err error
	if cls, err = factory.SetupCluster(ClusterName); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	} else {
		c.String(http.StatusOK, "successfully newed cluster")
	}
}

func startClusterHandler(c *gin.Context) {
	cls.Stop() // to make sure no existing process keeps running
	if err := cls.Start(); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	time.Sleep(5 * time.Second)
	ret := testutil.GetStatusAll(cls)
	if len(ret) < params.NodeCount {
		c.String(http.StatusInternalServerError,
			"failed to get status from %d node", params.NodeCount-len(ret))
	} else {
		c.String(http.StatusOK, "successfully started cluster")
	}
}

func stopClusterHandler(c *gin.Context) {
	cls.Stop()
	c.String(http.StatusOK, "successfully stopped cluster")
}

func checkLivenessHandler(c *gin.Context) {
	ret := testutil.GetStatusAll(cls)
	if len(ret) < params.NodeCount {
		failed := make([]int, 0)
		for i := 0; i < params.NodeCount; i++ {
			if _, ok := ret[i]; !ok {
				failed = append(failed, i)
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf(
			"failed to get status from node %+v", failed), "result": ret})
	} else {
		c.JSON(http.StatusOK, ret)
	}
}

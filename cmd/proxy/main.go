// Copyright (C) 2024 Wooyang2018
// Licensed under the GNU General Public License v3.0

package main

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/multiformats/go-multiaddr"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/node"
	"github.com/wooyang2018/svp-blockchain/tests/cluster"
	"github.com/wooyang2018/svp-blockchain/tests/testutil"
)

const (
	GinAddr     = ":8080"
	WorkDir     = "./workdir"
	ClusterName = "cluster_template"
	LastNLines  = 200
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
	nodeID, err := paramNodeID(c)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	path := c.Param("path")
	if path == "/" {
		c.String(http.StatusBadRequest, "path cannot be empty")
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
		c.String(http.StatusBadRequest, "command cannot be empty")
		return
	}
	fmt.Println("execute command:", cmd)
	command := exec.Command("sh", "-c", cmd)
	output, err := command.CombinedOutput()
	if err != nil {
		c.String(http.StatusInternalServerError, "execute command error:%+v", err)
	} else {
		c.String(http.StatusOK, string(output))
	}
}

func streamLogHandler(c *gin.Context) {
	nodeID, err := paramNodeID(c)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	workDir := path.Join(WorkDir, ClusterName)
	filePath := path.Join(workDir, strconv.Itoa(nodeID)+"/log.txt")
	file, err := os.Open(filePath)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer file.Close()

	startPosition, err := getStartPosition(file, LastNLines)
	if err != nil {
		c.String(http.StatusInternalServerError, "get start position error:%+v", err)
		return
	}
	if _, err := file.Seek(startPosition, 0); err != nil {
		c.String(http.StatusInternalServerError, "seek position error:%+v", err)
		return
	}

	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Content-Type")
	c.Header("Content-Type", "text/plain")
	c.Header("Transfer-Encoding", "chunked")
	c.Stream(func(w io.Writer) bool {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if _, err := w.Write([]byte(line + "\n")); err != nil {
				return false // client closes the connection
			}
			c.Writer.Flush()
		}
		if err := scanner.Err(); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return false
		}
		time.Sleep(1 * time.Second) // check for file updates every once in a while.
		return true
	})
}

func paramNodeID(c *gin.Context) (nodeID int, err error) {
	nodeID, err = strconv.Atoi(c.Param("node"))
	if err != nil {
		return params.NodeCount, err
	}
	if nodeID == -1 {
		nodeID = rand.Intn(params.NodeCount)
	}
	if nodeID < 0 || nodeID >= params.NodeCount {
		return params.NodeCount, fmt.Errorf("only support nodes from 0 to %d to proxy", params.NodeCount-1)
	}
	return nodeID, nil
}

// getStartPosition gets the position of the Nth line from the bottom.
func getStartPosition(file *os.File, n int) (int64, error) {
	var lines []int64
	fileInfo, err := file.Stat()
	if err != nil {
		return 0, err
	}
	fileSize := fileInfo.Size()

	// start reading from the end of the file.
	for position := fileSize - 1; position >= 0; position-- {
		if _, err := file.Seek(position, 0); err != nil {
			return 0, err
		}
		b := make([]byte, 1)
		if _, err := file.Read(b); err != nil {
			return 0, err
		}
		if b[0] == '\n' {
			lines = append(lines, position)
			if len(lines) == n {
				break
			}
		}
	}

	if len(lines) < n {
		return 0, fmt.Errorf("file has less than %d lines", n)
	}
	return lines[len(lines)-1], nil
}

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
		c.String(http.StatusInternalServerError, "failed to get status from %d node", params.NodeCount-len(ret))
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

func main() {
	gin.SetMode(gin.DebugMode)
	r := gin.Default()
	r.Use(CustomRecovery())

	r.Any("/proxy/:node/*path", proxyHandler)
	r.GET("/stream/:node", streamLogHandler)
	r.POST("/execute", commandHandler)
	r.POST("/setup/oneclick", oneClickHandler)

	r.POST("/setup/new/factory", clusterFactoryHandler)
	r.POST("/setup/reset/workdir", resetWorkDirHandler)
	r.POST("/setup/genesis/addrs", localAddrsHandler)
	r.POST("/setup/genesis/random", randomKeysHandler)
	r.POST("/setup/genesis/template", templateDirHandler)
	r.POST("/setup/build/chain", buildChainHandler)
	r.POST("/setup/new/cluster", newClusterHandler)
	r.POST("/setup/cluster/start", startClusterHandler)
	r.POST("/setup/cluster/stop", stopClusterHandler)
	r.GET("/setup/cluster/liveness", checkLivenessHandler)

	r.Static("/workdir", path.Join(WorkDir, ClusterName))
	common.Check2(r.Run(GinAddr))
}

func CustomRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintf("%+v", r))
				fmt.Printf("%s\n", debug.Stack())
				c.Abort()
			}
		}()
		c.Next()
	}
}

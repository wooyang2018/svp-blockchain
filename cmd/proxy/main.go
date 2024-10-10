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
	"github.com/wooyang2018/svp-blockchain/tests/cluster"
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

type TaskStatus int8

const (
	Pending TaskStatus = iota // task is pending
	Failed                    // task has failed
	Success                   // task was successful
)

var scores = map[string]map[string]TaskStatus{
	"setup": {
		"/new/factory":      Pending,
		"/reset/workdir":    Pending,
		"/genesis/addrs":    Pending,
		"/genesis/random":   Pending,
		"/genesis/template": Pending,
		"/build/chain":      Pending,
		"/new/cluster":      Pending,
		"/cluster/start":    Pending,
	},
	"transaction": {
		"/new/client/:node": Pending,
		"/upload/contract":  Pending,
		"/deploy/contract":  Pending,
		"/invoke/contract":  Pending,
		"/query/contract":   Pending,
	},
	"native": {
		"/deploy/pcoin": Pending,
		"/invoke/pcoin": Pending,
		"/query/pcoin":  Pending,
		"/invoke/xcoin": Pending,
		"/query/xcoin":  Pending,
		"/query/taddr":  Pending,
	},
}

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
	filePath := path.Join(workDir, strconv.Itoa(nodeID), "log.txt")
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

func scoresHandler(c *gin.Context) {
	c.JSON(http.StatusOK, scores)
}

func fileListHandler(c *gin.Context) {
	nodeID, err := paramNodeID(c)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	workDir := path.Join(WorkDir, ClusterName)
	nodePath := path.Join(workDir, strconv.Itoa(nodeID))
	fileList, err := getFileListRecur(nodePath)
	if err != nil {
		c.String(http.StatusInternalServerError, "read working directory error:%+v", err)
	} else {
		c.JSON(http.StatusOK, fileList)
	}
}

func getFileListRecur(dirPath string) ([]gin.H, error) {
	dir, err := os.Open(dirPath)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	files, err := dir.Readdir(-1)
	if err != nil {
		return nil, err
	}

	var fileList []gin.H
	for _, file := range files {
		if file.Name() == "db" {
			continue
		}
		// if it s a directory read recursively
		if file.IsDir() {
			filePath := path.Join(dirPath, file.Name())
			subFiles, err := getFileListRecur(filePath)
			if err != nil {
				return nil, err
			}
			fileList = append(fileList, gin.H{
				"name":  file.Name(),
				"size":  file.Size(),
				"time":  file.ModTime(),
				"files": subFiles,
			})
		} else { // if it s a file add it to the list
			fileList = append(fileList, gin.H{
				"name": file.Name(),
				"size": file.Size(),
				"time": file.ModTime(),
			})
		}
	}
	return fileList, nil
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

func main() {
	gin.SetMode(gin.DebugMode)
	r := gin.Default()
	r.Use(CustomRecovery())

	r.Any("/proxy/:node/*path", proxyHandler)
	r.GET("/stream/:node", streamLogHandler)
	r.POST("/execute", commandHandler)
	r.GET("/scores", scoresHandler)
	r.GET("/workdir/list/:node", fileListHandler)
	r.Static("/workdir/file", path.Join(WorkDir, ClusterName))

	r.POST("/setup/oneclick", oneClickHandler)
	r.POST("/setup/new/factory", clusterFactoryHandler)
	r.POST("/setup/reset/workdir", resetWorkDirHandler)
	r.POST("/setup/reset/status", resetStatusHandler)
	r.POST("/setup/genesis/addrs", localAddrsHandler)
	r.POST("/setup/genesis/random", randomKeysHandler)
	r.POST("/setup/genesis/template", templateDirHandler)
	r.POST("/setup/build/chain", buildChainHandler)
	r.POST("/setup/new/cluster", newClusterHandler)
	r.POST("/setup/cluster/start", startClusterHandler)
	r.POST("/setup/cluster/stop", stopClusterHandler)
	r.GET("/setup/cluster/liveness", checkLivenessHandler)

	r.POST("/transaction/new/client/:node", newClientHandler)
	r.POST("/transaction/upload/contract", uploadContractHandler)
	r.POST("/transaction/upload/bincc", uploadBinCodeHandler)
	r.POST("/transaction/deploy/contract", deployContractHandler)
	r.POST("/transaction/invoke/contract", invokeContractHandler)
	r.POST("/transaction/query/contract", queryContractHandler)

	r.POST("/native/deploy/pcoin", deployPCoinHandler)
	r.POST("/native/invoke/pcoin", invokePCoinHandler)
	r.POST("/native/query/pcoin", queryPCoinHandler)
	r.POST("/native/invoke/xcoin", invokeXCoinHandler)
	r.POST("/native/query/xcoin", queryXCoinHandler)
	r.POST("/native/query/taddr", queryTAddrHandler)

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

		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")

		route := c.FullPath()
		for _, prefix := range []string{"/setup", "/transaction", "/native"} {
			if strings.HasPrefix(route, prefix) {
				route = strings.TrimPrefix(route, prefix)
				addScore(prefix[1:], route, c.Writer.Status() == http.StatusOK)
				break
			}
		}
	}
}

func addScore(prefix string, route string, pass bool) {
	if _, ok := scores[prefix]; !ok {
		return
	}
	if _, ok := scores[prefix][route]; !ok {
		return
	}
	if pass {
		scores[prefix][route] = Success
	} else {
		scores[prefix][route] = Failed
	}
}

// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package testutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/wooyang2018/svp-blockchain/consensus"
	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/tests/cluster"
	"github.com/wooyang2018/svp-blockchain/txpool"
)

func GetStatus(node cluster.Node) (*consensus.Status, error) {
	if !node.IsRunning() {
		return nil, errors.New("node is not running")
	}
	resp, err := common.GetRequestWithRetry(node.GetEndpoint() + "/consensus")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	ret := new(consensus.Status)
	if err = json.NewDecoder(resp.Body).Decode(ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func GetTxPoolStatus(node cluster.Node) (*txpool.Status, error) {
	if !node.IsRunning() {
		return nil, errors.New("node is not running")
	}
	resp, err := common.GetRequestWithRetry(node.GetEndpoint() + "/txpool")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	ret := new(txpool.Status)
	if err = json.NewDecoder(resp.Body).Decode(ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func GetStatusAll(cls *cluster.Cluster) map[int]*consensus.Status {
	resps := make(map[int]*consensus.Status)
	var mtx sync.Mutex
	var wg sync.WaitGroup
	wg.Add(cls.NodeCount())
	for i := 0; i < cls.NodeCount(); i++ {
		go func(i int) {
			defer wg.Done()
			resp, err := GetStatus(cls.GetNode(i))
			if err == nil {
				mtx.Lock()
				defer mtx.Unlock()
				resps[i] = resp
			}
		}(i)
	}
	wg.Wait()
	return resps
}

func GetTxPoolStatusAll(cls *cluster.Cluster) map[int]*txpool.Status {
	resps := make(map[int]*txpool.Status)
	var mtx sync.Mutex
	var wg sync.WaitGroup
	wg.Add(cls.NodeCount())
	for i := 0; i < cls.NodeCount(); i++ {
		go func(i int) {
			defer wg.Done()
			resp, err := GetTxPoolStatus(cls.GetNode(i))
			if err == nil {
				mtx.Lock()
				defer mtx.Unlock()
				resps[i] = resp
			}
		}(i)
	}
	wg.Wait()
	return resps
}

func GetBlockByHeight(node cluster.Node, height uint64) (*core.Block, error) {
	if !node.IsRunning() {
		return nil, errors.New("node is not running")
	}
	resp, err := common.GetRequestWithRetry(fmt.Sprintf("%s/blocks/height/%d", node.GetEndpoint(), height))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	ret := core.NewBlock()
	if err = json.NewDecoder(resp.Body).Decode(ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func GetBlockByHeightAll(cls *cluster.Cluster, height uint64) map[int]*core.Block {
	resps := make(map[int]*core.Block)
	var mtx sync.Mutex
	var wg sync.WaitGroup
	wg.Add(cls.NodeCount())
	for i := 0; i < cls.NodeCount(); i++ {
		go func(i int) {
			defer wg.Done()
			resp, err := GetBlockByHeight(cls.GetNode(i), height)
			if err == nil {
				mtx.Lock()
				defer mtx.Unlock()
				resps[i] = resp
			}
		}(i)
	}
	wg.Wait()
	return resps
}

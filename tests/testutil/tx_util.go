// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package testutil

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/tests/cluster"
	"github.com/wooyang2018/svp-blockchain/txpool"
)

func SubmitTxAndWait(cls *cluster.Cluster, tx *core.Transaction) (int, error) {
	idx, err := SubmitTx(cls, nil, tx)
	if err != nil {
		return 0, err
	}
	for {
		if err = WaitTxCommitted(cls.GetNode(idx), tx); err != nil {
			// maybe current leader doesn't receive tx. resubmit tx again
			time.Sleep(1 * time.Second)
			return SubmitTxAndWait(cls, tx)
		} else {
			return idx, nil
		}
	}
}

func WaitTxCommitted(node cluster.Node, tx *core.Transaction) error {
	start := time.Now()
	for {
		status, err := GetTxStatus(node, tx.Hash())
		if err != nil {
			return fmt.Errorf("get tx status error, %w", err)
		} else {
			if status == txpool.TxStatusNotFound {
				return errors.New("submitted tx status not found")
			}
			if status == txpool.TxStatusCommitted {
				return nil
			}
		}
		if time.Since(start) > 1*time.Second {
			return errors.New("tx wait timeout")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func SubmitTx(cls *cluster.Cluster, retryOrder []int, tx *core.Transaction) (int, error) {
	b, err := json.Marshal(tx)
	if err != nil {
		return 0, err
	}
	if len(retryOrder) == 0 {
		retryOrder = PickUniqueRandoms(cls.NodeCount(), cls.NodeCount())
	}
	var retErr error
	for _, i := range retryOrder {
		if !cls.GetNode(i).IsRunning() {
			continue
		}
		resp, err := http.Post(cls.GetNode(i).GetEndpoint()+"/transactions",
			"application/json", bytes.NewReader(b))
		retErr = common.CheckResponse(resp, err)
		if retErr == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			return i, nil
		}
	}
	return 0, retErr
}

func BatchSubmitTx(cls *cluster.Cluster, retryOrder []int, txs *core.TxList) (int, error) {
	b, err := json.Marshal(txs)
	if err != nil {
		return 0, err
	}
	if len(retryOrder) == 0 {
		retryOrder = PickUniqueRandoms(cls.NodeCount(), cls.NodeCount())
	}
	var retErr error
	for _, i := range retryOrder {
		if !cls.GetNode(i).IsRunning() {
			continue
		}
		resp, err := http.Post(cls.GetNode(i).GetEndpoint()+"/transactions/batch",
			"application/json", bytes.NewReader(b))
		retErr = common.CheckResponse(resp, err)
		if retErr == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			return i, nil
		}
	}
	return 0, retErr
}

func GetTxStatus(node cluster.Node, hash []byte) (txpool.TxStatus, error) {
	hashStr := hex.EncodeToString(hash)
	resp, err := common.GetRequestWithRetry(node.GetEndpoint() +
		fmt.Sprintf("/transactions/%s/status", hashStr))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var status txpool.TxStatus
	return status, json.NewDecoder(resp.Body).Decode(&status)
}

func QueryState(node cluster.Node, query *common.QueryData) ([]byte, error) {
	b, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}
	resp, err := http.Post(node.GetEndpoint()+"/querystate",
		"application/json", bytes.NewReader(b))
	if err = common.CheckResponse(resp, err); err != nil {
		return nil, fmt.Errorf("cannot query state, %w", err)
	}
	defer resp.Body.Close()
	var ret []byte
	return ret, json.NewDecoder(resp.Body).Decode(&ret)
}

func uploadBinChainCode(cls *cluster.Cluster, binccPath string) (int, []byte, error) {
	return uploadChainCode(cls, binccPath, "bincc")
}

func uploadEVMChainCode(cls *cluster.Cluster, contractPath string) (int, []byte, error) {
	return uploadChainCode(cls, contractPath, "contract")
}

func uploadChainCode(cls *cluster.Cluster, path string, flag string) (int, []byte, error) {
	buf, contentType, err := common.CreateRequestBody(path)
	if err != nil {
		return 0, nil, err
	}
	var retErr error
	retryOrder := PickUniqueRandoms(cls.NodeCount(), cls.NodeCount())
	for _, i := range retryOrder {
		if !cls.GetNode(i).IsRunning() {
			continue
		}
		resp, err := http.Post(cls.GetNode(i).GetEndpoint()+"/"+flag, contentType, buf)
		retErr = common.CheckResponse(resp, err)
		if retErr == nil {
			defer resp.Body.Close()
			var codeID []byte
			return i, codeID, json.NewDecoder(resp.Body).Decode(&codeID)
		}
	}
	return 0, nil, fmt.Errorf("cannot upload %s chaincode, %w", flag, retErr)
}

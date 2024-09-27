// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package native

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/txpool"
)

type Client struct {
	signer   *core.PrivateKey
	codeAddr []byte
}

func NewClient(isDeploy bool) *Client {
	var err error
	client := &Client{}
	if !isDeploy {
		client.codeAddr, err = os.ReadFile(path.Join(dataPath, codeFile))
		common.Check2(err)
	}
	b, err := os.ReadFile(path.Join(dataPath, FileNodekey))
	common.Check2(err)
	client.signer, _ = core.NewPrivateKey(b)
	return client
}

func (client *Client) SetSigner(signer *core.PrivateKey) {
	client.signer = signer
}

func (client *Client) SetAddr(addr []byte) {
	client.codeAddr = addr
}

func (client *Client) GetAddr() []byte {
	return client.codeAddr
}

func (client *Client) SubmitTxAndWait(tx *core.Transaction) {
	client.SubmitTx(tx)
	if err := WaitTxCommitted(tx); err != nil {
		time.Sleep(1 * time.Second)
		client.SubmitTxAndWait(tx)
	}
}

func (client *Client) SubmitTx(tx *core.Transaction) {
	b, err := json.Marshal(tx)
	common.Check2(err)
	resp, err := http.Post(nodeUrl+"/transactions",
		"application/json", bytes.NewReader(b))
	common.Check2(err)
	msg, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("status code %d, %s\n", resp.StatusCode, string(msg))
}

func (client *Client) QueryState(input []byte) (ret []byte) {
	query := &common.QueryData{
		CodeAddr: client.codeAddr,
		Input:    input,
		Sender:   client.signer.PublicKey().Bytes(),
	}
	b, err := json.Marshal(query)
	common.Check2(err)
	resp, err := http.Post(nodeUrl+"/querystate", "application/json", bytes.NewReader(b))
	err = common.CheckResponse(resp, err)
	common.Check2(err)
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&ret)
	return
}

func (client *Client) MakeTx(input []byte) *core.Transaction {
	return core.NewTransaction().
		SetCodeAddr(client.codeAddr).
		SetNonce(time.Now().UnixNano()).
		SetInput(input).
		Sign(client.signer)
}

func (client *Client) makeTxWithInput(input *common.DeploymentInput) *core.Transaction {
	b, err := json.Marshal(input)
	common.Check2(err)
	return core.NewTransaction().
		SetNonce(time.Now().UnixNano()).
		SetInput(b).
		Sign(client.signer)
}

func (client *Client) MakeDeploymentTx(driverType common.DriverType,
	codeID []byte, initInput []byte) *core.Transaction {
	input := &common.DeploymentInput{
		CodeInfo: common.CodeInfo{
			DriverType: driverType,
			CodeID:     codeID,
		},
		InitInput: initInput,
	}
	switch driverType {
	case common.DriverTypeNative:
	case common.DriverTypeBincc:
		input.InstallData = []byte(nodeUrl + "/bincc/" + hex.EncodeToString(codeID))
	case common.DriverTypeEVM:
		input.InstallData = []byte(nodeUrl + "/contract/" + hex.EncodeToString(codeID))
	}
	return client.makeTxWithInput(input)
}

func UploadChainCode(driverType common.DriverType, filePath string) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return UploadChainCode2(driverType, f)
}

func UploadChainCode2(driverType common.DriverType, f io.Reader) ([]byte, error) {
	buf, contentType, err := common.CreateRequestBody(f)
	if err != nil {
		return nil, err
	}

	var urlPath string
	switch driverType {
	case common.DriverTypeBincc:
		urlPath = nodeUrl + "/bincc"
	case common.DriverTypeEVM:
		urlPath = nodeUrl + "/contract"
	case common.DriverTypeNative:
		return nil, errors.New("native chaincode no need to upload")
	}

	resp, err := http.Post(urlPath, contentType, buf)
	retErr := common.CheckResponse(resp, err)
	if retErr != nil {
		return nil, retErr
	}
	defer resp.Body.Close()
	var codeID []byte
	return codeID, json.NewDecoder(resp.Body).Decode(&codeID)
}

func GetTxStatus(hash []byte) (txpool.TxStatus, error) {
	hashStr := hex.EncodeToString(hash)
	resp, err := common.GetRequestWithRetry(nodeUrl +
		fmt.Sprintf("/transactions/%s/status", hashStr))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var status txpool.TxStatus
	return status, json.NewDecoder(resp.Body).Decode(&status)
}

func WaitTxCommitted(tx *core.Transaction) error {
	start := time.Now()
	for {
		status, err := GetTxStatus(tx.Hash())
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

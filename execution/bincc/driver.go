// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package bincc

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os"
	"path"
	"sync"
	"time"

	"github.com/wooyang2018/svp-blockchain/execution/common"
)

type CodeDriver struct {
	codeDir     string
	execTimeout time.Duration
	mtxInstall  sync.Mutex
}

var _ common.CodeDriver = (*CodeDriver)(nil)

func NewCodeDriver(codeDir string, timeout time.Duration) *CodeDriver {
	return &CodeDriver{
		codeDir:     codeDir,
		execTimeout: timeout,
	}
}

func (drv *CodeDriver) Install(codeID, data []byte) error {
	drv.mtxInstall.Lock()
	defer drv.mtxInstall.Unlock()
	return drv.downloadCodeIfRequired(codeID, data)
}

func (drv *CodeDriver) GetInstance(codeID []byte) (common.Chaincode, error) {
	return &Runner{
		codePath: path.Join(drv.codeDir, hex.EncodeToString(codeID)),
		timeout:  drv.execTimeout,
	}, nil
}

func (drv *CodeDriver) downloadCodeIfRequired(codeID, data []byte) error {
	filepath := path.Join(drv.codeDir, hex.EncodeToString(codeID))
	if _, err := os.Stat(filepath); err == nil {
		return nil // code file already exist
	}
	resp, err := common.DownloadCode(string(data), 5)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	sum, buf, err := common.CopyAndSumCode(resp.Body)
	if err != nil {
		return err
	}
	if !bytes.Equal(codeID, sum) {
		return errors.New("invalid code hash")
	}
	return common.WriteCodeFile(drv.codeDir, codeID, buf)
}

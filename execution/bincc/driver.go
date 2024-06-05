// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package bincc

import (
	"encoding/hex"
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
	return common.DownloadCodeIfRequired(drv.codeDir, codeID, data)
}

func (drv *CodeDriver) GetInstance(codeID []byte) (common.Chaincode, error) {
	return &Runner{
		codePath: path.Join(drv.codeDir, hex.EncodeToString(codeID)),
		timeout:  drv.execTimeout,
	}, nil
}

// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package evm

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os"
	"path"
	"sync"

	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/storage"
)

type CodeDriver struct {
	codeDir    string
	mtxInstall sync.Mutex

	driver  common.CodeDriver
	storage storage.PersistStore
	state   common.StateStore
	rootTrk *common.StateTracker
}

var _ common.CodeDriver = (*CodeDriver)(nil)

func NewCodeDriver(codeDir string, nativeDriver common.CodeDriver, storage storage.PersistStore,
	state common.StateStore) *CodeDriver {
	driver := &CodeDriver{
		codeDir: codeDir,
		driver:  nativeDriver,
		storage: storage,
		state:   state,
	}
	driver.rootTrk = common.NewStateTracker(state, nil)
	return driver
}

func (drv *CodeDriver) Install(codeID, data []byte) error {
	drv.mtxInstall.Lock()
	defer drv.mtxInstall.Unlock()
	return drv.downloadCodeIfRequired(codeID, data)
}

func (drv *CodeDriver) GetInstance(codeID []byte) (common.Chaincode, error) {
	return NewRunner(drv.driver, drv.storage, drv.rootTrk.Spawn(nil)), nil
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

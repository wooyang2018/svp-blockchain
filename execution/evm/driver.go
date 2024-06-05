// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package evm

import (
	"encoding/hex"
	"path"
	"sync"

	ethcomm "github.com/ethereum/go-ethereum/common"

	"github.com/wooyang2018/svp-blockchain/evm/statedb"
	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/storage"
)

type CodeDriver struct {
	codeDir    string
	driver     common.CodeDriver
	storage    storage.PersistStore
	stateDB    *statedb.StateDB
	mtxInstall sync.Mutex
}

var _ common.CodeDriver = (*CodeDriver)(nil)

func NewCodeDriver(codeDir string, nativeDriver common.CodeDriver, storage storage.PersistStore) *CodeDriver {
	cache := statedb.NewCacheDB(storage) // TODO implement OngBalanceHandle
	return &CodeDriver{
		codeDir: codeDir,
		driver:  nativeDriver,
		storage: storage,
		stateDB: statedb.NewStateDB(cache, ethcomm.Hash{}, ethcomm.Hash{}, statedb.NewDummy()),
	}
}

func (drv *CodeDriver) Install(codeID, data []byte) error {
	drv.mtxInstall.Lock()
	defer drv.mtxInstall.Unlock()
	return common.DownloadCodeIfRequired(drv.codeDir, codeID, data)
}

func (drv *CodeDriver) GetInstance(codeID []byte) (common.Chaincode, error) {
	return &Runner{
		CodePath: path.Join(drv.codeDir, hex.EncodeToString(codeID)),
		Driver:   drv.driver,
		Storage:  drv.storage,
		StateDB:  drv.stateDB,
	}, nil
}

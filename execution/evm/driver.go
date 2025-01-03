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
	proxy      *NativeProxy
	storage    storage.PersistStore
	stateDB    *statedb.StateDB
	mtxInstall sync.Mutex
}

var _ common.CodeDriver = (*CodeDriver)(nil)

func NewCodeDriver(codeDir string, nativeDriver common.CodeDriver, storage storage.PersistStore) *CodeDriver {
	cache := statedb.NewCacheDB(storage)
	proxy := NewNativeProxy(nativeDriver)
	stateDB := statedb.NewStateDB(cache, ethcomm.Hash{}, ethcomm.Hash{}, proxy)
	return &CodeDriver{
		codeDir: codeDir,
		proxy:   proxy,
		storage: storage,
		stateDB: stateDB,
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
		Proxy:    drv.proxy,
		Storage:  drv.storage,
		StateDB:  drv.stateDB,
	}, nil
}

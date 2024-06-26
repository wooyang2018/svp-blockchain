// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package statedb

import (
	"testing"

	ethcomm "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/wooyang2018/svp-blockchain/evm/common"
	"github.com/wooyang2018/svp-blockchain/storage"
)

func TestEtherAccount(t *testing.T) {
	a := require.New(t)

	memback := storage.NewMemLevelDBStore()
	cache := NewCacheDB(memback)
	// don't consider ong yet
	sd := NewStateDB(cache, ethcomm.Hash{}, ethcomm.Hash{}, NewDummy())
	a.NotNil(sd, "fail")

	h := crypto.Keccak256Hash([]byte("hello"))

	ea := &EthAccount{
		Nonce:    1023,
		CodeHash: h,
	}
	a.False(ea.IsEmpty(), "expect not empty")

	sink := common.NewZeroCopySink(nil)
	ea.Serialization(sink)

	clone := &EthAccount{}
	source := common.NewZeroCopySource(sink.Bytes())
	err := clone.Deserialization(source)
	a.Nil(err, "fail")
	a.Equal(clone, ea, "fail")

	pri, err := crypto.GenerateKey()
	a.Nil(err, "fail")
	ethAddr := crypto.PubkeyToAddress(pri.PublicKey)

	sd.cacheDB.PutEthAccount(ethAddr, *ea)

	getea := sd.getEthAccount(ethAddr)
	a.Equal(getea, *clone, "fail")

	a.Equal(sd.GetNonce(ethAddr), ea.Nonce, "fail")
	sd.SetNonce(ethAddr, 1024)
	a.Equal(sd.GetNonce(ethAddr), ea.Nonce+1, "fail")
	// don't effect code hash
	a.Equal(sd.getEthAccount(ethAddr).CodeHash, ea.CodeHash, "fail")

	sd.SetCode(ethAddr, []byte("hello again"))
	a.Equal(sd.GetCodeHash(ethAddr), crypto.Keccak256Hash([]byte("hello again")), "fail")
	a.Equal(sd.GetCode(ethAddr), []byte("hello again"), "fail")

	a.False(sd.HasSuicided(ethAddr), "fail")
	ret := sd.Suicide(ethAddr)
	a.True(ret, "fail")
	a.True(sd.HasSuicided(ethAddr), "fail")

	// nonexist account get ==> default value
	pri2, _ := crypto.GenerateKey()
	anotherAddr := crypto.PubkeyToAddress(pri2.PublicKey)

	nonce := sd.GetNonce(anotherAddr)
	a.Equal(nonce, uint64(0), "fail")
	hash := sd.GetCodeHash(anotherAddr)
	a.Equal(hash, ethcomm.Hash{}, "fail")

	sd.SetNonce(anotherAddr, 1)
	a.Equal(sd.GetNonce(anotherAddr), uint64(1), "fail")
}

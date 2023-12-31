// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package runtime

import (
	"math/big"
	"runtime"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/wooyang2018/svp-blockchain/evm/statedb"
	"github.com/wooyang2018/svp-blockchain/storage"
)

func makeConfig() *Config {
	cfg := new(Config)
	SetDefaults(cfg)

	memback := storage.NewMemLevelDBStore()
	cache := statedb.NewCacheDB(memback)
	cfg.State = statedb.NewStateDB(cache, common.Hash{}, common.Hash{}, statedb.NewDummy())

	cfg.GasLimit = 10000000
	cfg.Origin = common.HexToAddress("0xffffff")

	return cfg
}

func TestCreate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("compiling contracts on windows is not supported")
	}
	create(t, false)
	create(t, true)
}

func create(t *testing.T, is2 bool) {
	a := require.New(t)
	cfg := makeConfig()
	compiled := CompileCode("../testdata/contracts/Storage.sol")

	var contract *Contract
	// create with value
	cfg.State.AddBalance(cfg.Origin, big.NewInt(1e18))
	cfg.Value = big.NewInt(1e1)
	if is2 {
		contract = Create2Contract(cfg, compiled["Storage"][1], compiled["Storage"][0], 0xffff)
	} else {
		contract = CreateContract(cfg, compiled["Storage"][1], compiled["Storage"][0])
	}

	contract.AutoCommit = true
	cfg.Value = big.NewInt(0)
	_, _, err := contract.Call("store", big.NewInt(1024))
	a.Nil(err, "fail")

	ret, _, err := contract.Call("retrieve")
	a.Nil(err, "fail to retrive")
	a.Equal(big.NewInt(1024), big.NewInt(0).SetBytes(ret), "fail")
	// before self destruction
	a.Equal(contract.Balance(), big.NewInt(10), "fail")
	a.Equal(contract.BalanceOf(cfg.Origin), big.NewInt(1e18-1e1), "fail")

	// self destruction
	contract.AutoCommit = false
	contract.Call("close")
	a.Equal(contract.Balance(), big.NewInt(0), "fail")
	a.Equal(contract.BalanceOf(cfg.Origin), big.NewInt(1e18), "fail")
	a.True(cfg.State.Suicided[contract.Address])

	// get again
	contract.AutoCommit = true
	ret, _, err = contract.Call("retrieve")
	a.Nil(err, "fail")
	a.Equal(big.NewInt(1024), big.NewInt(0).SetBytes(ret), "fail")
	a.False(cfg.State.Suicided[contract.Address])

	// storage should be cleaned after commit
	ret, _, err = contract.Call("retrieve")
	a.Nil(err, "calling non exist contract will be taken as normal transfer")
	a.Nil(ret, "fail")
}

func TestContractChainDelete(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("compiling contracts on windows is not supported")
	}
	contractChaindelete(t, false)
	contractChaindelete(t, true)
}

func contractChaindelete(t *testing.T, is2 bool) {
	a := require.New(t)
	cfg := makeConfig()
	callee := CompileCode("../testdata/contracts/Callee.sol")
	caller := CompileCode("../testdata/contracts/Caller.sol")

	var lee *Contract
	if is2 {
		lee = Create2Contract(cfg, callee["Callee"][1], callee["Callee"][0], 0xffff)
	} else {
		lee = CreateContract(cfg, callee["Callee"][1], callee["Callee"][0])
	}
	lee.AutoCommit = true
	cfg.Value = big.NewInt(0)

	_, _, err := lee.Call("store", big.NewInt(1024))
	a.Nil(err, "fail to call store")

	ret, _, err := lee.Call("retrieve")
	a.Nil(err, "fail to call retrieve")
	a.Equal(big.NewInt(1024), big.NewInt(0).SetBytes(ret), "fail")

	var ler *Contract
	if is2 {
		ler = Create2Contract(cfg, caller["Caller"][1], caller["Caller"][0], 0xffff, lee.Address)
	} else {
		ler = CreateContract(cfg, caller["Caller"][1], caller["Caller"][0], lee.Address)
	}
	ler.AutoCommit = true

	ret, _, err = ler.Call("getNum")
	a.Nil(err, "fail to get caller getNum")
	a.Equal(big.NewInt(0).SetBytes(ret), big.NewInt(1024), "fail to get callee store")

	// caller set num and callee self destruct
	ler.AutoCommit = true
	cfg.Value = big.NewInt(0)
	ret, _, err = ler.Call("setA", big.NewInt(1023))
	a.Nil(err, "fail to call setA")
	a.Equal(big.NewInt(0).SetBytes(ret), big.NewInt(1023), "fail")
	a.False(cfg.State.Suicided[ler.Address], "fail")
}

func TestCreateOnDeletedAddress(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("compiling contracts on windows is not supported")
	}
	a := require.New(t)
	cfg := makeConfig()
	compiled := CompileCode("../testdata/contracts/Storage.sol")

	c := Create2Contract(cfg, compiled["Storage"][1], compiled["Storage"][0], 0xffff)
	a.NotNil(c, "fail")

	_, _, err := c.Call("store", big.NewInt(0x1234))
	a.Nil(err, "fail")

	c.AutoCommit = true
	_, _, err = c.Call("close")
	a.Nil(err, "fail")

	ret, _, err := c.Call("retrieve")
	a.Nil(err, "fail")
	a.Nil(ret, "fail")

	c2 := Create2Contract(cfg, compiled["Storage"][1], compiled["Storage"][0], 0xffff)
	a.NotNil(c2, "fail")
	a.Equal(c.Address, c2.Address, "create2 should get the same contract address with same salt")

	ret, _, err = c2.Call("retrieve")
	a.Nil(err, "fail")
	a.True((big.NewInt(0).SetBytes(ret).Cmp(big.NewInt(0)) == 0), "should not get previous value 0x1234")
}

func byteArrayToUint64(byteArray []byte) uint64 {
	var result uint64
	for _, b := range byteArray {
		result = (result << 8) | uint64(b)
	}
	return result
}

func TestENSRegistry(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("compiling contracts on windows is not supported")
	}
	a := require.New(t)
	cfg := makeConfig()
	compiled := CompileCode("../testdata/contracts/ens/ENSRegistry.sol")

	c := CreateContract(cfg, compiled["ENSRegistry"][1], compiled["ENSRegistry"][0])
	a.NotNil(c, "fail")

	_, _, err := c.Call("setRecord", [32]byte{}, common.HexToAddress("0xffffff"), common.HexToAddress("0x2"), uint64(0x10))
	a.NoError(err, "fail")

	ret, _, err := c.Call("owner", [32]byte{}) // make sure owner=msg.sender
	a.Equal(common.HexToAddress("0xffffff"), common.BytesToAddress(ret), "the owner should be same as input")
	a.NoError(err, "fail")

	ret, _, err = c.Call("resolver", [32]byte{})
	a.Equal(common.HexToAddress("0x2"), common.BytesToAddress(ret), "the resolver should be same as input")
	a.NoError(err, "fail")

	ret, _, err = c.Call("ttl", [32]byte{})
	a.Equal(uint64(0x10), byteArrayToUint64(ret), "the ttl should be same as input")
	a.NoError(err, "fail")

	ret, _, err = c.Call("recordExists", [32]byte{})
	a.True(new(big.Int).SetBytes(ret).Cmp(big.NewInt(1)) == 0, "record existence should be the same")
	a.NoError(err, "fail")

	ret, _, err = c.Call("setApprovalForAll", common.HexToAddress("0x3"), true)
	a.NoError(err, "fail")

	ret, _, err = c.Call("isApprovedForAll", common.HexToAddress("0xffffff"), common.HexToAddress("0x3"))
	a.True(new(big.Int).SetBytes(ret).Cmp(big.NewInt(1)) == 0, "approval or not should be the same")
	a.NoError(err, "fail")

	ret, _, err = c.Call("setSubnodeRecord", [32]byte{}, [32]byte{0x4}, common.HexToAddress("0xffffff"), common.HexToAddress("0x4"), uint64(0x10))
	a.NoError(err, "fail")
}

func TestENSRegistryWithFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("compiling contracts on windows is not supported")
	}
	a := require.New(t)
	cfg := makeConfig()
	compiled := CompileCode("../testdata/contracts/ens/ENSRegistryWithFallback.sol")

	ens := CreateContract(cfg, compiled["ENSRegistry"][1], compiled["ENSRegistry"][0])
	c := CreateContract(cfg, compiled["ENSRegistryWithFallback"][1], compiled["ENSRegistryWithFallback"][0], ens.Address)
	a.NotNil(c, "fail")

	_, _, err := ens.Call("setRecord", [32]byte{}, common.HexToAddress("0xffffff"), common.HexToAddress("0x2"), uint64(0x10))
	a.NoError(err, "fail")

	ret, _, err := c.Call("owner", [32]byte{})
	a.Equal(common.HexToAddress("0xffffff"), common.BytesToAddress(ret), "the owner should be same as input")
	a.NoError(err, "fail")

	ret, _, err = c.Call("resolver", [32]byte{})
	a.Equal(common.HexToAddress("0x2"), common.BytesToAddress(ret), "the resolver should be same as input")
	a.NoError(err, "fail")
}

func TestFIFSRegistrar(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("compiling contracts on windows is not supported")
	}
	a := require.New(t)
	cfg := makeConfig()
	compiled := CompileCode("../testdata/contracts/ens/FIFSRegistrar.sol")
	compiled2 := CompileCode("../testdata/contracts/ens/ENSRegistry.sol")

	ens := CreateContract(cfg, compiled2["ENSRegistry"][1], compiled2["ENSRegistry"][0])
	c := CreateContract(cfg, compiled["FIFSRegistrar"][1], compiled["FIFSRegistrar"][0], ens.Address, [32]byte{})
	a.NotNil(c, "fail")

	_, _, err := ens.Call("setApprovalForAll", c.Address, true)
	a.NoError(err, "fail")
	_, _, err = c.Call("register", [32]byte{0x4}, common.HexToAddress("0xffffff")) // label owner
	a.NoError(err, "fail")
}

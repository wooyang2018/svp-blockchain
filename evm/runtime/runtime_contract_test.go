package runtime

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"github.com/wooyang2018/svp-blockchain/evm/storage"
	"github.com/wooyang2018/svp-blockchain/evm/storage/overlaydb"
	"github.com/wooyang2018/svp-blockchain/storage/leveldbstore"
)

const (
	// // SPDX-License-Identifier: GPL-3.0
	// pragma solidity >0.6.0;
	// contract Storage {
	//  address payable private owner;
	//  uint256 number;
	//  constructor() payable {
	//   owner = payable(msg.sender);
	//  }
	//  function store(uint256 num) public {
	//   number = num;
	//  }
	//  function retrieve() public view returns (uint256){
	//   return number;
	//  }
	//  function close() public {
	//   selfdestruct(owner);
	//  }
	// }

	StorageBin = "0x6080604052336000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550610113806100536000396000f3fe6080604052348015600f57600080fd5b5060043610603c5760003560e01c80632e64cec114604157806343d726d614605d5780636057361d146065575b600080fd5b60476090565b6040518082815260200191505060405180910390f35b6063609a565b005b608e60048036036020811015607957600080fd5b810190808035906020019092919050505060d3565b005b6000600154905090565b60008054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16ff5b806001819055505056fea2646970667358221220d0c72c744e1607d4f5949be99171995482c4e8eda2cd79d0179b06b0795d6e1564736f6c63430007060033"
	StorageABI = "[{\"inputs\":[],\"stateMutability\":\"payable\",\"type\":\"constructor\"},{\"inputs\":[],\"name\":\"close\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"retrieve\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"num\",\"type\":\"uint256\"}],\"name\":\"store\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]"

	// this is a real contract bin compiled with solc
	// callee
	// // SPDX-License-Identifier: GPL-3.0
	// pragma solidity >0.6.0;
	// contract Callee {
	//     address payable private owner;
	//     uint256 number;
	//     constructor() payable {
	//         owner = payable(msg.sender);
	//     }
	//     function store(uint256 num) public {
	//         number = num;
	//     }
	//     function storedtor (uint256 num) public {
	//         number = num;
	//         selfdestruct(owner);
	//     }
	//     function retrieve() public view returns (uint256){
	//         return number;
	//     }
	//     function close() public {
	//         selfdestruct(owner);
	//     }
	// }

	// CalleeABI is the input ABI used to generate the binding from.
	CalleeABI = "[{\"inputs\":[],\"stateMutability\":\"payable\",\"type\":\"constructor\"},{\"inputs\":[],\"name\":\"close\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"retrieve\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"num\",\"type\":\"uint256\"}],\"name\":\"store\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"num\",\"type\":\"uint256\"}],\"name\":\"storedtor\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]"

	// CalleeBin is the compiled bytecode used for deploying new contracts.
	CalleeBin = "0x6080604052336000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550610198806100536000396000f3fe608060405234801561001057600080fd5b506004361061004c5760003560e01c80632e64cec11461005157806343d726d61461006f5780636057361d146100795780638ef9db3d146100a7575b600080fd5b6100596100d5565b6040518082815260200191505060405180910390f35b6100776100df565b005b6100a56004803603602081101561008f57600080fd5b8101908080359060200190929190505050610118565b005b6100d3600480360360208110156100bd57600080fd5b8101908080359060200190929190505050610122565b005b6000600154905090565b60008054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16ff5b8060018190555050565b8060018190555060008054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16fffea2646970667358221220ef7c81ae86fe3b0f53373d74ddee4e9e4333fb42e2566074b7a4357bac25cd8d64736f6c63430007060033"

	// caller
	// SPDX-License-Identifier: GPL-3.0
	// pragma solidity >0.6.0;
	// contract Callee {
	//     function store(uint256 num) public{}
	//     function retrieve() public view returns (uint256){}
	//     function storedtor (uint256 num) public  {}
	// }
	// contract Caller  {
	//     Callee st;
	//     constructor(address _t) payable {
	//         st = Callee(_t);
	//     }
	//     function getNum() public view returns (uint256 result) {
	//         return st.retrieve();
	//     }
	//     function setA(uint256 _val) public payable returns (uint256 result) {
	//         st.storedtor(_val);
	//         return _val;
	//     }
	// }

	// CallerABI is the input ABI used to generate the binding from.
	CallerABI = "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_t\",\"type\":\"address\"}],\"stateMutability\":\"payable\",\"type\":\"constructor\"},{\"inputs\":[],\"name\":\"getNum\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"result\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_val\",\"type\":\"uint256\"}],\"name\":\"setA\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"result\",\"type\":\"uint256\"}],\"stateMutability\":\"payable\",\"type\":\"function\"}]"

	// CallerBin is the compiled bytecode used for deploying new contracts.
	CallerBin = "0x60806040526040516102973803806102978339818101604052602081101561002657600080fd5b8101908080519060200190929190505050806000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050610210806100876000396000f3fe6080604052600436106100295760003560e01c806367e0badb1461002e578063ee919d5014610059575b600080fd5b34801561003a57600080fd5b5061004361009b565b6040518082815260200191505060405180910390f35b6100856004803603602081101561006f57600080fd5b8101908080359060200190929190505050610144565b6040518082815260200191505060405180910390f35b60008060009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16632e64cec16040518163ffffffff1660e01b815260040160206040518083038186803b15801561010457600080fd5b505afa158015610118573d6000803e3d6000fd5b505050506040513d602081101561012e57600080fd5b8101908080519060200190929190505050905090565b60008060009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16638ef9db3d836040518263ffffffff1660e01b815260040180828152602001915050600060405180830381600087803b1580156101ba57600080fd5b505af11580156101ce573d6000803e3d6000fd5b5050505081905091905056fea26469706673582212204539e1a8dfe53fc5ae35507ba94bcfaa5c20605fe237c6af0dd1137b1ecbbea564736f6c63430007060033"
)

func mkcfg() *Config {
	cfg := new(Config)
	setDefaults(cfg)

	memback := leveldbstore.NewMemLevelDBStore()
	overlay := overlaydb.NewOverlayDB(memback)

	cache := storage.NewCacheDB(overlay)
	cfg.State = storage.NewStateDB(cache, common.Hash{}, common.Hash{}, storage.NewDummy())

	cfg.GasLimit = 10000000
	cfg.Origin = common.HexToAddress("0x123456")

	return cfg
}

func TestCreate(t *testing.T) {
	create(t, false)
}

func TestCreate2(t *testing.T) {
	create(t, true)
}

func create(t *testing.T, is2 bool) {
	a := require.New(t)

	cfg := mkcfg()

	// 1. create
	// 2. set
	// 3. get
	// 4. selfdestruction
	// 5. get again
	var contract *Contract
	// create with value
	cfg.State.AddBalance(cfg.Origin, big.NewInt(1e18))
	cfg.Value = big.NewInt(1e1)
	if is2 {
		contract = Create2Contract(cfg, StorageABI, StorageBin, 0xffff)
	} else {
		contract = CreateContract(cfg, StorageABI, StorageBin)
	}

	contract.AutoCommit = true
	cfg.Value = big.NewInt(0)
	_, _, err := contract.Call("store", big.NewInt(1024))
	a.Nil(err, "fail")

	ret, _, err := contract.Call("retrieve")
	a.Nil(err, "fail to retrive")
	a.Equal(big.NewInt(1024), (&big.Int{}).SetBytes(ret), "fail")

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
	a.Equal(big.NewInt(1024), (&big.Int{}).SetBytes(ret), "fail")

	a.False(cfg.State.Suicided[contract.Address])
	// after commit, storage should be cleaned
	ret, _, err = contract.Call("retrieve")
	a.Nil(err, "calling non exist contract will be taken as normal transfer")
	a.Nil(ret, "fail")
}

func TestContractChainDelete(t *testing.T) {
	contractChaindelete(t, false)
}

func TestContractChainDelete2(t *testing.T) {
	contractChaindelete(t, true)
}

func contractChaindelete(t *testing.T, is2 bool) {
	a := require.New(t)

	cfg := mkcfg()

	var lee *Contract
	if is2 {
		lee = Create2Contract(cfg, CalleeABI, CalleeBin, 0xffff)
	} else {
		lee = CreateContract(cfg, CalleeABI, CalleeBin)
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
		ler = Create2Contract(cfg, CallerABI, CallerBin, 0xffff, lee.Address)
	} else {
		ler = CreateContract(cfg, CallerABI, CallerBin, lee.Address)
	}
	ler.AutoCommit = true

	ret, _, err = ler.Call("getNum")
	a.Nil(err, "fail to get caller getNum")
	a.Equal(big.NewInt(0).SetBytes(ret), big.NewInt(1024), "fail to get callee store")

	// caller set num and callee selfdestruct
	ler.AutoCommit = true
	cfg.Value = big.NewInt(0)
	ret, _, err = ler.Call("setA", big.NewInt(1023))
	a.Nil(err, "fail to call setA")
	a.Equal(big.NewInt(0).SetBytes(ret), big.NewInt(1023), "fail")
	a.False(cfg.State.Suicided[ler.Address], "fail")
}

func TestCreateOnDeletedAddress(t *testing.T) {
	a := require.New(t)
	cfg := mkcfg()
	c := Create2Contract(cfg, StorageABI, StorageBin, 0xffff)
	a.NotNil(c, "fail")

	_, _, err := c.Call("store", big.NewInt(0x1234))
	a.Nil(err, "fail")

	c.AutoCommit = true
	_, _, err = c.Call("close")
	a.Nil(err, "fail")

	ret, _, err := c.Call("retrieve")
	a.Nil(err, "fail")
	a.Nil(ret, "fail")

	c2 := Create2Contract(cfg, StorageABI, StorageBin, 0xffff)
	a.NotNil(c2, "fail")
	a.Equal(c.Address, c2.Address, "create2 should get the same contract address with same salt")
	ret, _, err = c2.Call("retrieve")
	a.Nil(err, "fail")
	a.True((big.NewInt(0).SetBytes(ret).Cmp(big.NewInt(0)) == 0), "should not get previous value 0x1234")
}

package evm

import (
	"encoding/json"
	"errors"
	"math/big"

	ethcomm "github.com/ethereum/go-ethereum/common"

	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/native"
	"github.com/wooyang2018/svp-blockchain/native/taddr"
	"github.com/wooyang2018/svp-blockchain/native/xcoin"
)

type NativeProxy struct {
	addr20Cache map[[20]byte][32]byte
	addr32Cache map[[32]byte][20]byte
	txTrk       *common.StateTracker
	tmpTrk      map[string]*common.StateTracker
	driver      common.CodeDriver
}

func NewNativeProxy(driver common.CodeDriver) *NativeProxy {
	return &NativeProxy{
		addr20Cache: make(map[[20]byte][32]byte),
		addr32Cache: make(map[[32]byte][20]byte),
		tmpTrk:      make(map[string]*common.StateTracker),
		driver:      driver,
	}
}

func (p *NativeProxy) SubBalance(addr ethcomm.Address, val *big.Int) error {
	addr32, err := p.queryAddr(addr.Bytes())
	if err != nil {
		return err
	}
	balance, err := p.QueryBalanceByAddr32(addr32)
	if err != nil {
		return err
	}
	oldVal := common.DecodeBalance(balance)
	if oldVal < val.Uint64() {
		return errors.New("not enough balance")
	}
	return p.SetBalanceByAddr32(addr32, oldVal-val.Uint64())
}

func (p *NativeProxy) AddBalance(addr ethcomm.Address, val *big.Int) error {
	addr32, err := p.queryAddr(addr.Bytes())
	if err != nil {
		return err
	}
	balance, err := p.QueryBalanceByAddr32(addr32)
	if err != nil {
		return err
	}
	oldVal := common.DecodeBalance(balance)
	return p.SetBalanceByAddr32(addr32, oldVal+val.Uint64())
}

func (p *NativeProxy) SetBalance(addr ethcomm.Address, val *big.Int) error {
	return p.SetBalanceByAddr20(addr.Bytes(), val.Uint64())
}

func (p *NativeProxy) GetBalance(addr ethcomm.Address) (*big.Int, error) {
	balance, err := p.QueryBalanceByAddr20(addr.Bytes())
	if err != nil {
		return nil, err
	}
	oldVal := common.DecodeBalance(balance)
	return big.NewInt(0).SetUint64(oldVal), nil
}

func (p *NativeProxy) QueryContractAddr20(hash []byte) ([]byte, error) {
	invokeTrk := p.txTrk.Spawn(hash)
	addr20 := ethcomm.BytesToAddress(invokeTrk.GetState(keyAddr))
	return addr20.Bytes(), nil
}

func (p *NativeProxy) QueryContractAddr32(hash []byte) ([]byte, error) {
	addr20, _ := p.QueryContractAddr20(hash)
	addr32, err := p.queryAddr(addr20)
	if err != nil {
		return nil, err
	}
	return addr32, nil
}

func (p *NativeProxy) QueryBalanceByAddr20(addr20 []byte) ([]byte, error) {
	addr32, err := p.queryAddr(addr20)
	if err != nil {
		return nil, err
	}
	return p.QueryBalanceByAddr32(addr32)
}

func (p *NativeProxy) QueryBalanceByAddr32(addr32 []byte) ([]byte, error) {
	cc, err := p.driver.GetInstance(native.CodeXCoin)
	if err != nil {
		return nil, err
	}
	queryTrk := p.getTrk(native.FileCodeXCoin)
	input := &xcoin.Input{
		Method: "balance",
		Dest:   addr32,
	}
	rawInput, _ := json.Marshal(input)
	return cc.Query(&common.CallContextQuery{
		StateGetter: queryTrk,
		RawInput:    rawInput,
	})
}

func (p *NativeProxy) SetBalanceByAddr20(addr20 []byte, value uint64) error {
	addr32, err := p.queryAddr(addr20)
	if err != nil {
		return err
	}
	return p.SetBalanceByAddr32(addr32, value)
}

func (p *NativeProxy) SetBalanceByAddr32(addr32 []byte, value uint64) error {
	cc, err := p.driver.GetInstance(native.CodeXCoin)
	if err != nil {
		return err
	}
	invokeTrk := p.getTrk(native.FileCodeXCoin)
	input := &xcoin.Input{
		Method: "set",
		Dest:   addr32,
		Value:  value,
	}
	rawInput, _ := json.Marshal(input)
	return cc.Invoke(&common.CallContextTx{
		StateTracker: invokeTrk,
		RawInput:     rawInput,
	})
}

func (p *NativeProxy) queryAddr(addr []byte) ([]byte, error) {
	if len(addr) == 20 {
		if v, ok := p.addr20Cache[[20]byte(addr)]; ok {
			return v[:], nil
		}
	} else if len(addr) == 32 {
		if v, ok := p.addr32Cache[[32]byte(addr)]; ok {
			return v[:], nil
		}
	}
	cc, err := p.driver.GetInstance(native.CodeTAddr)
	if err != nil {
		return nil, err
	}
	queryTrk := p.getTrk(native.FileCodeTAddr)
	input := &taddr.Input{
		Method: "query",
		Addr:   addr,
	}
	rawInput, _ := json.Marshal(input)
	resAddr, err := cc.Query(&common.CallContextQuery{
		StateGetter: queryTrk,
		RawInput:    rawInput,
	})
	if err != nil {
		return nil, err
	}
	if len(addr) == 20 {
		p.addr20Cache[[20]byte(addr)] = [32]byte(resAddr)
	} else if len(addr) == 32 {
		p.addr32Cache[[32]byte(addr)] = [20]byte(resAddr)
	}
	return resAddr, nil
}

func (p *NativeProxy) storeAddr(addr []byte) error {
	cc, err := p.driver.GetInstance(native.CodeTAddr)
	if err != nil {
		return err
	}
	invokeTrk := p.getTrk(native.FileCodeTAddr)
	input := &taddr.Input{
		Method: "store",
		Addr:   addr,
	}
	rawInput, _ := json.Marshal(input)
	return cc.Invoke(&common.CallContextTx{
		StateTracker: invokeTrk,
		RawInput:     rawInput,
	})
}

func (p *NativeProxy) setTxTrk(txTrk *common.StateTracker) {
	p.txTrk = txTrk
}

func (p *NativeProxy) mergeTrks() {
	for _, trk := range p.tmpTrk {
		p.txTrk.Merge(trk)
	}
}

func (p *NativeProxy) getTrk(key string) *common.StateTracker {
	if res, ok := p.tmpTrk[key]; ok {
		return res
	}
	p.tmpTrk[key] = p.txTrk.Spawn(common.GetCodeAddr(key))
	return p.tmpTrk[key]
}

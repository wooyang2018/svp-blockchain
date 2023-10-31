// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package execution

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/native"
	"github.com/wooyang2018/svp-blockchain/native/pcoin"
)

func TestExecution(t *testing.T) {
	asrt := assert.New(t)

	state := newMapStateStore()
	reg := newCodeRegistry()
	reg.registerDriver(common.DriverTypeNative, native.NewCodeDriver())

	execution := &Execution{
		stateStore:   state,
		codeRegistry: reg,
		config:       DefaultConfig,
	}
	execution.config.TxExecTimeout = 1 * time.Second

	priv := core.GenerateKey(nil)
	blk := core.NewBlock().SetHeight(10).Sign(priv)

	cinfo := common.CodeInfo{
		DriverType: common.DriverTypeNative,
		CodeID:     native.CodePCoin,
	}
	cinfo2 := common.CodeInfo{
		DriverType: common.DriverTypeNative,
		CodeID:     []byte{2, 2, 2}, // invalid code id
	}

	depInput := &common.DeploymentInput{CodeInfo: cinfo}
	b, _ := json.Marshal(depInput)

	depInput.CodeInfo = cinfo2
	b2, _ := json.Marshal(depInput)

	tx1 := core.NewTransaction().SetNonce(time.Now().Unix()).SetInput(b).Sign(priv)
	tx2 := core.NewTransaction().SetNonce(time.Now().Unix()).SetInput(b2).Sign(priv)
	tx3 := core.NewTransaction().SetNonce(time.Now().Unix()).SetInput(b).Sign(priv)

	bcm, txcs := execution.Execute(blk, []*core.Transaction{tx1, tx2, tx3})

	asrt.Equal(blk.Hash(), bcm.Hash())
	asrt.EqualValues(3, len(txcs))
	asrt.NotEmpty(bcm.StateChanges())

	asrt.Equal(tx1.Hash(), txcs[0].Hash())
	asrt.Equal(tx2.Hash(), txcs[1].Hash())
	asrt.Equal(tx3.Hash(), txcs[2].Hash())

	for _, sc := range bcm.StateChanges() {
		state.SetState(sc.Key(), sc.Value())
	}

	regTrk := newStateTracker(state, codeRegistryAddr)
	resci, err := reg.getCodeInfo(tx1.Hash(), regTrk)
	asrt.NoError(err)
	asrt.Equal(&cinfo, resci)

	resci, err = reg.getCodeInfo(tx2.Hash(), regTrk)
	asrt.Error(err)
	asrt.Nil(resci)

	resci, err = reg.getCodeInfo(tx3.Hash(), regTrk)
	asrt.NoError(err)
	asrt.Equal(&cinfo, resci)

	ccInput, _ := json.Marshal(pcoin.Input{Method: "minter"})
	minter, err := execution.Query(&common.QueryData{tx1.Hash(), ccInput})
	asrt.NoError(err)
	asrt.Equal(priv.PublicKey().Bytes(), minter)

	minter, err = execution.Query(&common.QueryData{tx2.Hash(), ccInput})
	asrt.Error(err)
	asrt.Nil(minter)

	minter, err = execution.Query(&common.QueryData{tx3.Hash(), ccInput})
	asrt.NoError(err)
	asrt.Equal(priv.PublicKey().Bytes(), minter)
}

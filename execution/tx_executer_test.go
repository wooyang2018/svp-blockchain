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

func TestTxExecutor(t *testing.T) {
	asrt := assert.New(t)

	priv := core.GenerateKey(nil)
	depInput := &common.DeploymentInput{
		CodeInfo: common.CodeInfo{
			DriverType: common.DriverTypeNative,
			CodeID:     native.CodePCoin,
		},
	}
	b, _ := json.Marshal(depInput)
	txDep := core.NewTransaction().SetInput(b).Sign(priv)

	blk := core.NewBlock().SetHeight(10).Sign(priv)
	trk := common.NewStateTracker(common.NewMapStateStore(), nil)
	reg := newCodeRegistry()
	texe := txExecutor{
		codeRegistry: reg,
		timeout:      1 * time.Second,
		txTrk:        trk,
		blk:          blk,
		tx:           txDep,
	}
	txc := texe.execute()

	asrt.NotEqual("", txc.Error(), "code driver not registered")

	reg.registerDriver(common.DriverTypeNative, native.NewCodeDriver())
	txc = texe.execute()

	asrt.Equal("", txc.Error())
	asrt.Equal(blk.Hash(), txc.BlockHash())
	asrt.Equal(blk.Height(), txc.BlockHeight())

	// codeinfo must be saved by key (transaction hash)
	cinfo, err := reg.getCodeInfo(txDep.Hash(), trk.Spawn(codeRegistryAddr))

	asrt.NoError(err)
	asrt.Equal(*cinfo, depInput.CodeInfo)

	cc, err := reg.getInstance(txDep.Hash(), trk.Spawn(codeRegistryAddr))

	asrt.NoError(err)
	asrt.NotNil(cc)

	ccInput := &pcoin.Input{
		Method: "minter",
	}
	b, _ = json.Marshal(ccInput)
	minter, err := cc.Query(&common.CallContextTx{
		RawInput:     b,
		StateTracker: trk.Spawn(txDep.Hash()),
	})

	asrt.NoError(err)
	asrt.Equal(priv.PublicKey().Bytes(), minter, "deployer must be set as minter")

	ccInput.Method = "mint"
	ccInput.Dest = priv.PublicKey().Bytes()
	ccInput.Value = 100
	b, _ = json.Marshal(ccInput)
	txInvoke := core.NewTransaction().SetCodeAddr(txDep.Hash()).SetInput(b).Sign(priv)
	texe.tx = txInvoke
	txc = texe.execute()

	asrt.Equal("", txc.Error())

	ccInput.Method = "balance"
	ccInput.Value = 0
	b, _ = json.Marshal(ccInput)
	b, err = cc.Query(&common.CallContextTx{
		RawInput:     b,
		StateTracker: trk.Spawn(txDep.Hash()),
	})

	asrt.NoError(err)
	asrt.EqualValues(100, common.DecodeBalance(b))
}

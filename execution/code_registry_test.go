// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package execution

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/native"
)

func TestCodeRegistry(t *testing.T) {
	asrt := assert.New(t)

	trk := common.NewStateTracker(common.NewMapStateStore(), codeRegistryAddr)
	reg := newCodeRegistry()

	codeAddr := bytes.Repeat([]byte{1}, 32)
	dep := &common.DeploymentInput{
		CodeInfo: common.CodeInfo{
			DriverType: common.DriverTypeNative,
			CodeID:     native.CodePCoin,
		},
	}

	cc, err := reg.getInstance(codeAddr, trk)

	asrt.Error(err, "code not deployed yet")
	asrt.Nil(cc)

	cc, err = reg.deploy(codeAddr, dep, trk)

	asrt.Error(err, "native driver not registered yet")
	asrt.Nil(cc)

	reg.registerDriver(common.DriverTypeNative, native.NewCodeDriver())
	cc, err = reg.deploy(codeAddr, dep, trk)

	asrt.NoError(err)
	asrt.NotNil(cc)

	err = reg.registerDriver(common.DriverTypeNative, native.NewCodeDriver())

	asrt.Error(err, "registered driver twice")

	cc, err = reg.getInstance(codeAddr, trk)

	asrt.NoError(err)
	asrt.NotNil(cc)

	cc, err = reg.getInstance(bytes.Repeat([]byte{2}, 32), trk)

	asrt.Error(err, "wrong code address")
	asrt.Nil(cc)
}

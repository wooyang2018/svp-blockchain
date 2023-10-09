// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wooyang2018/svp-blockchain/evm/common"
)

func TestDeployCode_Serialize(t *testing.T) {
	ty, _ := VmTypeFromByte(1)
	deploy, err := NewDeployCode([]byte{1, 2, 3}, ty, "", "", "", "", "")
	assert.Nil(t, err)
	sink := common.NewZeroCopySink(nil)
	deploy.Serialization(sink)
	bs := sink.Bytes()
	var deploy2 DeployCode

	source := common.NewZeroCopySource(bs)
	deploy2.Deserialization(source)
	assert.Equal(t, &deploy2, deploy)

	source = common.NewZeroCopySource(bs[:len(bs)-1])
	err = deploy2.Deserialization(source)
	assert.NotNil(t, err)
}

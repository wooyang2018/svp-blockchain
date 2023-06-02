// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package execution

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNativeCodeDriver(t *testing.T) {
	assert := assert.New(t)

	drv := newNativeCodeDriver()
	cc, err := drv.GetInstance([]byte(NativeCodeIDPCoin))

	assert.NoError(err)
	assert.NotNil(cc)

	err = drv.Install([]byte(NativeCodeIDPCoin), nil)

	assert.NoError(err)
}

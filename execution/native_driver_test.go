// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package execution

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNativeCodeDriver(t *testing.T) {
	asrt := assert.New(t)

	drv := newNativeCodeDriver()
	cc, err := drv.GetInstance(NativeCodePCoin)

	asrt.NoError(err)
	asrt.NotNil(cc)

	err = drv.Install(NativeCodePCoin, nil)

	asrt.NoError(err)
}

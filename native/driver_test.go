// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package native

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCodeDriver(t *testing.T) {
	asrt := assert.New(t)
	drv := NewCodeDriver()

	cc, err := drv.GetInstance(CodePCoin)
	asrt.NoError(err)
	asrt.NotNil(cc)

	err = drv.Install(CodePCoin, nil)
	asrt.NoError(err)
}

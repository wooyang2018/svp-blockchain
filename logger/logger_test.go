// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	asrt := assert.New(t)
	asrt.NotPanics(func() {
		I().Info("hello", "key", "value", "key1", 1)
	})
}

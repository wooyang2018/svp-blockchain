// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStateChange(t *testing.T) {
	asrt := assert.New(t)
	sc := NewStateChange().
		SetKey([]byte("key")).
		SetValue([]byte("value")).
		SetPrevValue([]byte("prevValue")).
		SetTreeIndex([]byte{1}).
		SetPrevTreeIndex(nil)

	b, err := sc.Marshal()
	asrt.NoError(err)
	sc = NewStateChange()
	err = sc.Unmarshal(b)
	asrt.NoError(err)

	asrt.Equal([]byte("key"), sc.Key())
	asrt.Equal([]byte("value"), sc.Value())
	asrt.Equal([]byte("prevValue"), sc.PrevValue())
	asrt.Equal([]byte{1}, sc.TreeIndex())
	asrt.Nil(sc.PrevTreeIndex())
}

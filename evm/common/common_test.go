// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package common

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHexAndBytesTransfer(t *testing.T) {
	testBytes := []byte("10, 11, 12, 13, 14, 15, 16, 17, 18, 19")
	stringAfterTrans := ToHexString(testBytes)
	bytesAfterTrans, err := HexToBytes(stringAfterTrans)
	assert.Nil(t, err)
	assert.Equal(t, testBytes, bytesAfterTrans)
}

func TestBase58(t *testing.T) {
	addr := ADDRESS_EMPTY
	fmt.Println("emtpy addr:", addr.ToBase58())
}

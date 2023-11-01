// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package common

import (
	"encoding/hex"
	"math"
)

type Serializable interface {
	Serialization(sink *ZeroCopySink)
}

func SerializeToBytes(values ...Serializable) []byte {
	sink := NewZeroCopySink(nil)
	for _, val := range values {
		val.Serialization(sink)
	}
	return sink.Bytes()
}

// ToHexString convert []byte to hex string
func ToHexString(data []byte) string {
	return hex.EncodeToString(data)
}

// HexToBytes convert hex string to []byte
func HexToBytes(value string) ([]byte, error) {
	return hex.DecodeString(value)
}

func ReverseArray(arr []byte) []byte {
	l := len(arr)
	x := make([]byte, 0)
	for i := l - 1; i >= 0; i-- {
		x = append(x, arr[i])
	}
	return x
}

func SafeSub(x, y uint64) (uint64, bool) {
	return x - y, x < y
}

func SafeAdd(x, y uint64) (uint64, bool) {
	return x + y, y > math.MaxUint64-x
}

func SafeMul(x, y uint64) (uint64, bool) {
	if x == 0 || y == 0 {
		return 0, false
	}
	return x * y, y > math.MaxUint64/x
}

// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package common

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func BenchmarkZeroCopySource(b *testing.B) {
	const N = 12000
	buf := make([]byte, N)
	rand.Read(buf)

	for i := 0; i < b.N; i++ {
		source := NewZeroCopySource(buf)
		for j := 0; j < N/100; j++ {
			source.NextUint16()
			source.NextByte()
			source.NextUint64()
			source.NextVarUint()
			source.NextBytes(20)
		}
	}
}

func TestReadFromNil(t *testing.T) {
	s := NewZeroCopySource(nil)
	_, _, _, eof := s.NextString()
	assert.True(t, eof)
}

func TestReadVarInt(t *testing.T) {
	s := NewZeroCopySource([]byte{0xfd})
	_, _, _, eof := s.NextString()
	assert.True(t, eof)
}

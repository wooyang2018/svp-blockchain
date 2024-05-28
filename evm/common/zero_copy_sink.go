// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package common

import (
	"bytes"
	"encoding/binary"
	"math"

	ethcomm "github.com/ethereum/go-ethereum/common"
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

type ZeroCopySink struct {
	buf []byte
}

// NewZeroCopySink returns a new ZeroCopySink reading from b.
func NewZeroCopySink(b []byte) *ZeroCopySink {
	if b == nil {
		b = make([]byte, 0, 512)
	}
	return &ZeroCopySink{b}
}

// tryGrowByReslice is a inlineable version of grow for the fast-case where the
// internal buffer only needs to be resliced.
// It returns the index where bytes should be written and whether it succeeded.
func (self *ZeroCopySink) tryGrowByReslice(n int) (int, bool) {
	if l := len(self.buf); n <= cap(self.buf)-l {
		self.buf = self.buf[:l+n]
		return l, true
	}
	return 0, false
}

// grow grows the buffer to guarantee space for n more bytes.
// It returns the index where bytes should be written.
// If the buffer can't grow it will panic with ErrTooLarge.
func (self *ZeroCopySink) grow(n int) int {
	// Try to grow by means of a reslice.
	if i, ok := self.tryGrowByReslice(n); ok {
		return i
	}
	l := len(self.buf)
	c := cap(self.buf)
	if c > math.MaxInt-c-n {
		panic(bytes.ErrTooLarge)
	}
	// Not enough space anywhere, we need to allocate.
	buf := makeSlice(2*c + n)
	copy(buf, self.buf)
	self.buf = buf[:l+n]
	return l
}

func (self *ZeroCopySink) WriteBytes(p []byte) {
	data := self.NextBytes(uint64(len(p)))
	copy(data, p)
}

func (self *ZeroCopySink) NextBytes(n uint64) (data []byte) {
	m, ok := self.tryGrowByReslice(int(n))
	if !ok {
		m = self.grow(int(n))
	}
	data = self.buf[m:]
	return
}

func (self *ZeroCopySink) BackUp(n uint64) {
	l := len(self.buf) - int(n)
	self.buf = self.buf[:l]
}

func (self *ZeroCopySink) WriteUint8(data uint8) {
	buf := self.NextBytes(1)
	buf[0] = data
}

func (self *ZeroCopySink) WriteByte(c byte) {
	self.WriteUint8(c)
}

func (self *ZeroCopySink) WriteBool(data bool) {
	if data {
		self.WriteByte(1)
	} else {
		self.WriteByte(0)
	}
}

func (self *ZeroCopySink) WriteUint16(data uint16) {
	buf := self.NextBytes(2)
	binary.LittleEndian.PutUint16(buf, data)
}

func (self *ZeroCopySink) WriteUint32(data uint32) {
	buf := self.NextBytes(4)
	binary.LittleEndian.PutUint32(buf, data)
}

func (self *ZeroCopySink) WriteUint64(data uint64) *ZeroCopySink {
	buf := self.NextBytes(8)
	binary.LittleEndian.PutUint64(buf, data)
	return self
}

func (self *ZeroCopySink) WriteInt64(data int64) {
	self.WriteUint64(uint64(data))
}

func (self *ZeroCopySink) WriteInt32(data int32) {
	self.WriteUint32(uint32(data))
}

func (self *ZeroCopySink) WriteInt16(data int16) {
	self.WriteUint16(uint16(data))
}

func (self *ZeroCopySink) WriteVarBytes(data []byte) (size uint64) {
	l := uint64(len(data))
	size = self.WriteVarUint(l) + l
	self.WriteBytes(data)
	return
}

func (self *ZeroCopySink) WriteString(data string) (size uint64) {
	return self.WriteVarBytes([]byte(data))
}

func (self *ZeroCopySink) WriteAddress(addr ethcomm.Address) {
	self.WriteBytes(addr.Bytes())
}

func (self *ZeroCopySink) WriteHash(hash ethcomm.Hash) {
	self.WriteBytes(hash.Bytes())
}

func (self *ZeroCopySink) WriteVarUint(data uint64) (size uint64) {
	buf := self.NextBytes(9)
	if data < 0xFD {
		buf[0] = uint8(data)
		size = 1
	} else if data <= 0xFFFF {
		buf[0] = 0xFD
		binary.LittleEndian.PutUint16(buf[1:], uint16(data))
		size = 3
	} else if data <= 0xFFFFFFFF {
		buf[0] = 0xFE
		binary.LittleEndian.PutUint32(buf[1:], uint32(data))
		size = 5
	} else {
		buf[0] = 0xFF
		binary.LittleEndian.PutUint64(buf[1:], uint64(data))
		size = 9
	}
	self.BackUp(9 - size)
	return
}

func (self *ZeroCopySink) Size() uint64 { return uint64(len(self.buf)) }

func (self *ZeroCopySink) Bytes() []byte { return self.buf }

func (self *ZeroCopySink) Reset() { self.buf = self.buf[:0] }

// makeSlice allocates a slice of size n. If the allocation fails, it panics with ErrTooLarge.
func makeSlice(n int) []byte {
	// If the make fails, give a known error.
	defer func() {
		if recover() != nil {
			panic(bytes.ErrTooLarge)
		}
	}()
	return make([]byte, n)
}

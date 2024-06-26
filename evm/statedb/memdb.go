// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

// this implementation is based on goleveldb "https://github.com/syndtr/goleveldb"
// the semantics of Delete(key) operation is changed to Put(key, nil) to support overlaydb

package statedb

import (
	"math/rand"

	"github.com/syndtr/goleveldb/leveldb/comparer"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/util"
)

var (
	ErrNotFound     = errors.ErrNotFound
	ErrIterReleased = errors.New("statedb/memdb: iterator released")
)

const tMaxHeight = 12

const (
	nKV = iota
	nKey
	nVal
	nHeight
	nNext
)

// MemDB is an in-memdb key/value database.
type MemDB struct {
	cmp comparer.BasicComparer
	rnd *rand.Rand

	kvData []byte
	// Node data:
	// [0]         : KV offset
	// [1]         : Key length
	// [2]         : Value length
	// [3]         : Height
	// [3..height] : Next nodes
	nodeData  []int
	prevNode  [tMaxHeight]int
	maxHeight int
	n         int
	kvSize    int
}

// NewMemDB creates a new initialized in-memdb key/value MemDB. The capacity
// is the initial key/value buffer capacity. The capacity is advisory,
// not enforced.
//
// This MemDB is append-only, deleting an entry would remove entry node but not
// reclaim KV buffer.
//
// The returned MemDB instance is safe for concurrent use.
func NewMemDB(capacity int, kvNum int) *MemDB {
	p := &MemDB{
		cmp:       comparer.DefaultComparer,
		rnd:       rand.New(rand.NewSource(0xdeadbeef)),
		maxHeight: 1,
		kvData:    make([]byte, 0, capacity),
		nodeData:  make([]int, 4+tMaxHeight, (4+tMaxHeight)*(1+kvNum)),
	}
	p.nodeData[nHeight] = tMaxHeight
	return p
}

func (self *MemDB) DeepClone() *MemDB {
	cloned := &MemDB{
		cmp:       self.cmp,
		rnd:       self.rnd,
		kvData:    append([]byte{}, self.kvData...),
		nodeData:  append([]int{}, self.nodeData...),
		prevNode:  self.prevNode,
		maxHeight: self.maxHeight,
		n:         self.n,
		kvSize:    self.kvSize,
	}

	return cloned
}

func (p *MemDB) randHeight() (h int) {
	const branching = 4
	h = 1
	for h < tMaxHeight && p.rnd.Int()%branching == 0 {
		h++
	}
	return
}

// Must hold RW-lock if prev == true, as it use shared prevNode slice.
func (p *MemDB) findGE(key []byte, prev bool) (int, bool) {
	node := 0
	h := p.maxHeight - 1
	for {
		next := p.nodeData[node+nNext+h]
		cmp := 1
		if next != 0 {
			o := p.nodeData[next]
			cmp = p.cmp.Compare(p.kvData[o:o+p.nodeData[next+nKey]], key)
		}
		if cmp < 0 {
			// Keep searching in this list
			node = next
		} else {
			if prev {
				p.prevNode[h] = node
			} else if cmp == 0 {
				return next, true
			}
			if h == 0 {
				return next, cmp == 0
			}
			h--
		}
	}
}

func (p *MemDB) findLT(key []byte) int {
	node := 0
	h := p.maxHeight - 1
	for {
		next := p.nodeData[node+nNext+h]
		o := p.nodeData[next]
		if next == 0 || p.cmp.Compare(p.kvData[o:o+p.nodeData[next+nKey]], key) >= 0 {
			if h == 0 {
				break
			}
			h--
		} else {
			node = next
		}
	}
	return node
}

func (p *MemDB) findLast() int {
	node := 0
	h := p.maxHeight - 1
	for {
		next := p.nodeData[node+nNext+h]
		if next == 0 {
			if h == 0 {
				break
			}
			h--
		} else {
			node = next
		}
	}
	return node
}

// Put sets the value for the given key. It overwrites any previous value
// for that key; a MemDB is not a multi-map.
//
// It is safe to modify the contents of the arguments after Put returns.
func (p *MemDB) Put(key []byte, value []byte) {
	if node, exact := p.findGE(key, true); exact {
		if len(value) != 0 {
			kvOffset := len(p.kvData)
			p.kvData = append(p.kvData, key...)
			p.kvData = append(p.kvData, value...)
			p.nodeData[node] = kvOffset
		}
		m := p.nodeData[node+nVal]
		p.nodeData[node+nVal] = len(value)
		p.kvSize += len(value) - m
		return
	}

	h := p.randHeight()
	if h > p.maxHeight {
		for i := p.maxHeight; i < h; i++ {
			p.prevNode[i] = 0
		}
		p.maxHeight = h
	}

	kvOffset := len(p.kvData)
	p.kvData = append(p.kvData, key...)
	p.kvData = append(p.kvData, value...)
	// Node
	node := len(p.nodeData)
	p.nodeData = append(p.nodeData, kvOffset, len(key), len(value), h)
	for i, n := range p.prevNode[:h] {
		m := n + nNext + i
		p.nodeData = append(p.nodeData, p.nodeData[m])
		p.nodeData[m] = node
	}

	p.kvSize += len(key) + len(value)
	p.n++
}

// Delete deletes the value for the given key.
//
// It is safe to modify the contents of the arguments after Delete returns.
func (p *MemDB) Delete(key []byte) {
	p.Put(key, nil)
}

// Get gets the value for the given key. It returns unkown == true if the
// MemDB does not contain the key. It returns nil, false if MemDB has deleted the key
//
// The caller should not modify the contents of the returned slice, but
// it is safe to modify the contents of the argument after Get returns.
func (p *MemDB) Get(key []byte) (value []byte, unkown bool) {
	if node, exact := p.findGE(key, false); exact {
		valen := p.nodeData[node+nVal]
		if valen != 0 {
			o := p.nodeData[node] + p.nodeData[node+nKey]
			value = p.kvData[o : o+valen]
		}
	} else {
		unkown = true
	}
	return
}

// Find finds key/value pair whose key is greater than or equal to the
// given key. It returns ErrNotFound if the table doesn't contain
// such pair.
//
// The caller should not modify the contents of the returned slice, but
// it is safe to modify the contents of the argument after Find returns.
func (p *MemDB) Find(key []byte) (rkey, value []byte, err error) {
	if node, _ := p.findGE(key, false); node != 0 {
		n := p.nodeData[node]
		m := n + p.nodeData[node+nKey]
		rkey = p.kvData[n:m]
		valen := p.nodeData[node+nVal]
		if valen != 0 {
			value = p.kvData[m : m+valen]
		}
	} else {
		err = ErrNotFound
	}
	return
}

func (p *MemDB) ForEach(f func(key, val []byte)) {
	for node := p.nodeData[nNext]; node != 0; node = p.nodeData[node+nNext] {
		n := p.nodeData[node]
		m := n + p.nodeData[node+nKey]
		key := p.kvData[n:m]
		val := p.kvData[m : m+p.nodeData[node+nVal]]
		f(key, val)
	}

}

// NewIterator returns an iterator of the MemDB.
// The returned iterator is not safe for concurrent use, but it is safe to use
// multiple iterators concurrently, with each in a dedicated goroutine.
// It is also safe to use an iterator concurrently with modifying its
// underlying MemDB. However, the resultant key/value pairs are not guaranteed
// to be a consistent snapshot of the MemDB at a particular point in time.
//
// Slice allows slicing the iterator to only contains keys in the given
// range. A nil Range.Start is treated as a key before all keys in the
// MemDB. And a nil Range.Limit is treated as a key after all keys in
// the MemDB.
//
// The iterator must be released after use, by calling Release method.
//
// Also read Iterator documentation of the leveldb/iterator package.
func (p *MemDB) NewIterator(slice *util.Range) iterator.Iterator {
	return &dbIter{p: p, slice: slice}
}

// Capacity returns keys/values buffer capacity.
func (p *MemDB) Capacity() int {
	return cap(p.kvData)
}

// Size returns sum of keys and values length. Note that deleted
// key/value will not be accounted for, but it will still consume
// the buffer, since the buffer is append only.
func (p *MemDB) Size() int {
	return p.kvSize
}

// Free returns keys/values free buffer before need to grow.
func (p *MemDB) Free() int {
	return cap(p.kvData) - len(p.kvData)
}

// Len returns the number of entries in the MemDB.
func (p *MemDB) Len() int {
	return p.n
}

// Reset resets the MemDB to initial empty state. Allows reuse the buffer.
func (p *MemDB) Reset() {
	p.rnd = rand.New(rand.NewSource(0xdeadbeef))
	p.maxHeight = 1
	p.n = 0
	p.kvSize = 0
	p.kvData = p.kvData[:0]
	p.nodeData = p.nodeData[:nNext+tMaxHeight]
	p.nodeData[nKV] = 0
	p.nodeData[nKey] = 0
	p.nodeData[nVal] = 0
	p.nodeData[nHeight] = tMaxHeight
	for n := 0; n < tMaxHeight; n++ {
		p.nodeData[nNext+n] = 0
		p.prevNode[n] = 0
	}
}

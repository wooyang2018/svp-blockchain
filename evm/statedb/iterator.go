// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package statedb

import (
	"github.com/syndtr/goleveldb/leveldb/comparer"
	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/wooyang2018/svp-blockchain/storage"
)

type KeyOrigin byte

const (
	FromMem  KeyOrigin = iota
	FromBack           = iota
	FromBoth           = iota
)

type JoinIter struct {
	backend     storage.StoreIterator
	memdb       storage.StoreIterator
	key, value  []byte
	keyOrigin   KeyOrigin
	nextMemEnd  bool
	nextBackEnd bool
	cmp         comparer.BasicComparer
}

func NewJoinIter(memIter, backendIter storage.StoreIterator) *JoinIter {
	return &JoinIter{
		backend: backendIter,
		memdb:   memIter,
		cmp:     comparer.DefaultComparer,
	}
}

func (iter *JoinIter) First() bool {
	f := iter.first()
	if !f {
		return false
	}
	for len(iter.value) == 0 {
		if !iter.next() {
			return false
		}
	}

	return true
}

func (iter *JoinIter) first() bool {
	var bkey, bval, mkey, mval []byte
	back := iter.backend.First()
	mem := iter.memdb.First()
	// check error
	if iter.Error() != nil {
		return false
	}
	if back {
		bkey = iter.backend.Key()
		bval = iter.backend.Value()
		if !mem {
			iter.key = bkey
			iter.value = bval
			iter.keyOrigin = FromBack
			return true
		}
		mkey = iter.memdb.Key()
		mval = iter.memdb.Value()
		cmp := iter.cmp.Compare(mkey, bkey)
		if cmp < 1 {
			iter.key = mkey
			iter.value = mval
			if cmp == 0 {
				iter.keyOrigin = FromBoth
			} else {
				iter.keyOrigin = FromMem
			}
		} else {
			iter.key = bkey
			iter.value = bval
			iter.keyOrigin = FromBack
		}
		return true
	} else {
		if mem {
			iter.key = iter.memdb.Key()
			iter.value = iter.memdb.Value()
			iter.keyOrigin = FromMem
			return true
		}
		return false
	}
}

func (iter *JoinIter) Key() []byte {
	return iter.key
}

func (iter *JoinIter) Value() []byte {
	return iter.value
}

func (iter *JoinIter) Next() bool {
	f := iter.next()
	if !f {
		return false
	}

	for len(iter.value) == 0 {
		if !iter.next() {
			return false
		}
	}

	return true
}

func (iter *JoinIter) next() bool {
	if (iter.keyOrigin == FromMem || iter.keyOrigin == FromBoth) && !iter.nextMemEnd {
		iter.nextMemEnd = !iter.memdb.Next()
	}
	if (iter.keyOrigin == FromBack || iter.keyOrigin == FromBoth) && !iter.nextBackEnd {
		iter.nextBackEnd = !iter.backend.Next()
	}

	// check error
	if iter.Error() != nil {
		return false
	}

	if iter.nextBackEnd {
		if iter.nextMemEnd {
			iter.key = nil
			iter.value = nil
			return false
		} else {
			iter.key = iter.memdb.Key()
			iter.value = iter.memdb.Value()
			iter.keyOrigin = FromMem
		}
	} else {
		if iter.nextMemEnd {
			iter.key = iter.backend.Key()
			iter.value = iter.backend.Value()
			iter.keyOrigin = FromBack
		} else {
			bkey := iter.backend.Key()
			mkey := iter.memdb.Key()
			cmp := iter.cmp.Compare(mkey, bkey)
			switch cmp {
			case -1:
				iter.key = mkey
				iter.value = iter.memdb.Value()
				iter.keyOrigin = FromMem
			case 0:
				iter.key = mkey
				iter.value = iter.memdb.Value()
				iter.keyOrigin = FromBoth
			case 1:
				iter.key = bkey
				iter.value = iter.backend.Value()
				iter.keyOrigin = FromBack
			default:
				panic("unreachable")
			}
		}
	}

	return true
}

func (iter *JoinIter) Release() {
	iter.memdb.Release()
	iter.backend.Release()
}

func (iter *JoinIter) Error() error {
	if iter.backend.Error() != nil {
		return iter.backend.Error()
	} else if iter.memdb.Error() != nil {
		return iter.memdb.Error()
	}
	return nil
}

type dbIter struct {
	util.BasicReleaser
	p          *MemDB
	slice      *util.Range
	node       int
	forward    bool
	key, value []byte
	err        error
}

func (i *dbIter) fill(checkStart, checkLimit bool) bool {
	if i.node != 0 {
		n := i.p.nodeData[i.node]
		m := n + i.p.nodeData[i.node+nKey]
		i.key = i.p.kvData[n:m]
		if i.slice != nil {
			switch {
			case checkLimit && i.slice.Limit != nil && i.p.cmp.Compare(i.key, i.slice.Limit) >= 0:
				fallthrough
			case checkStart && i.slice.Start != nil && i.p.cmp.Compare(i.key, i.slice.Start) < 0:
				i.node = 0
				goto bail
			}
		}
		i.value = i.p.kvData[m : m+i.p.nodeData[i.node+nVal]]
		return true
	}
bail:
	i.key = nil
	i.value = nil
	return false
}

func (i *dbIter) Valid() bool {
	return i.node != 0
}

func (i *dbIter) First() bool {
	if i.Released() {
		i.err = ErrIterReleased
		return false
	}

	i.forward = true
	if i.slice != nil && i.slice.Start != nil {
		i.node, _ = i.p.findGE(i.slice.Start, false)
	} else {
		i.node = i.p.nodeData[nNext]
	}
	return i.fill(false, true)
}

func (i *dbIter) Last() bool {
	if i.Released() {
		i.err = ErrIterReleased
		return false
	}

	i.forward = false
	if i.slice != nil && i.slice.Limit != nil {
		i.node = i.p.findLT(i.slice.Limit)
	} else {
		i.node = i.p.findLast()
	}
	return i.fill(true, false)
}

func (i *dbIter) Seek(key []byte) bool {
	if i.Released() {
		i.err = ErrIterReleased
		return false
	}

	i.forward = true
	if i.slice != nil && i.slice.Start != nil && i.p.cmp.Compare(key, i.slice.Start) < 0 {
		key = i.slice.Start
	}
	i.node, _ = i.p.findGE(key, false)
	return i.fill(false, true)
}

func (i *dbIter) Next() bool {
	if i.Released() {
		i.err = ErrIterReleased
		return false
	}

	if i.node == 0 {
		if !i.forward {
			return i.First()
		}
		return false
	}
	i.forward = true
	i.node = i.p.nodeData[i.node+nNext]
	return i.fill(false, true)
}

func (i *dbIter) Prev() bool {
	if i.Released() {
		i.err = ErrIterReleased
		return false
	}

	if i.node == 0 {
		if i.forward {
			return i.Last()
		}
		return false
	}
	i.forward = false
	i.node = i.p.findLT(i.key)
	return i.fill(true, false)
}

func (i *dbIter) Key() []byte {
	return i.key
}

func (i *dbIter) Value() []byte {
	return i.value
}

func (i *dbIter) Error() error { return i.err }

func (i *dbIter) Release() {
	if !i.Released() {
		i.p = nil
		i.node = 0
		i.key = nil
		i.value = nil
		i.BasicReleaser.Release()
	}
}

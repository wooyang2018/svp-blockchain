// Copyright (C) 2021 Aung Maw
// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package core

import (
	"github.com/wooyang2018/ppov-blockchain/pb"
	"google.golang.org/protobuf/proto"
)

type StateChange struct {
	data *pb.StateChange
}

func NewStateChange() *StateChange {
	return &StateChange{
		data: new(pb.StateChange),
	}
}

func (sc *StateChange) Key() []byte           { return sc.data.Key }
func (sc *StateChange) Value() []byte         { return sc.data.Value }
func (sc *StateChange) PrevValue() []byte     { return sc.data.PrevValue }
func (sc *StateChange) TreeIndex() []byte     { return sc.data.TreeIndex }
func (sc *StateChange) PrevTreeIndex() []byte { return sc.data.PrevTreeIndex }

func (sc *StateChange) setData(val *pb.StateChange) error {
	sc.data = val
	return nil
}

func (sc *StateChange) SetKey(val []byte) *StateChange {
	sc.data.Key = val
	return sc
}

func (sc *StateChange) SetValue(val []byte) *StateChange {
	sc.data.Value = val
	return sc
}

func (sc *StateChange) SetPrevValue(val []byte) *StateChange {
	sc.data.PrevValue = val
	return sc
}

func (sc *StateChange) SetTreeIndex(val []byte) *StateChange {
	sc.data.TreeIndex = val
	return sc
}

func (sc *StateChange) SetPrevTreeIndex(val []byte) *StateChange {
	sc.data.PrevTreeIndex = val
	return sc
}

func (sc *StateChange) Marshal() ([]byte, error) {
	return proto.Marshal(sc.data)
}

func (sc *StateChange) Unmarshal(b []byte) error {
	data := new(pb.StateChange)
	if err := proto.Unmarshal(b, data); err != nil {
		return err
	}
	return sc.setData(data)
}

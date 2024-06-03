// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package common

import (
	"sync"

	"github.com/wooyang2018/svp-blockchain/core"
)

// StateTracker tracks state changes in key order
// get latest changed state for each key
// get state from base state getter if no changes occured for a key
type StateTracker struct {
	keyPrefix []byte
	baseState StateGetter

	trackDep     bool
	dependencies map[string]struct{} // getState calls
	changes      map[string][]byte   // setState calls

	mtxChg sync.RWMutex
	mtxDep sync.RWMutex
}

func NewStateTracker(state StateGetter, keyPrefix []byte) *StateTracker {
	return &StateTracker{
		keyPrefix: keyPrefix,
		baseState: state,

		dependencies: make(map[string]struct{}),
		changes:      make(map[string][]byte),
	}
}

func (trk *StateTracker) GetState(key []byte) []byte {
	trk.mtxChg.RLock()
	defer trk.mtxChg.RUnlock()
	return trk.getState(key)
}

func (trk *StateTracker) SetState(key, value []byte) {
	trk.mtxChg.Lock()
	defer trk.mtxChg.Unlock()
	trk.setState(key, value)
}

// Spawn creates a new tracker with current tracker as base StateGetter
func (trk *StateTracker) Spawn(keyPrefix []byte) *StateTracker {
	child := NewStateTracker(trk, keyPrefix)
	child.trackDep = true
	return child
}

func (trk *StateTracker) HasDependencyChanges(child *StateTracker) bool {
	trk.mtxChg.RLock()
	defer trk.mtxChg.RUnlock()

	child.mtxDep.RLock()
	defer child.mtxDep.RUnlock()

	prefixStr := string(trk.keyPrefix)

	for key := range child.dependencies {
		key = prefixStr + key
		if _, changed := trk.changes[key]; changed {
			return true
		}
	}
	return false
}

func (trk *StateTracker) Merge(child *StateTracker) {
	trk.mtxChg.Lock()
	defer trk.mtxChg.Unlock()

	child.mtxChg.RLock()
	defer child.mtxChg.RUnlock()

	for key, value := range child.changes {
		trk.setState([]byte(key), value)
	}
}

func (trk *StateTracker) GetStateChanges() []*core.StateChange {
	trk.mtxChg.RLock()
	defer trk.mtxChg.RUnlock()

	scList := make([]*core.StateChange, 0, len(trk.changes))
	for key, value := range trk.changes {
		scList = append(scList, core.NewStateChange().SetKey([]byte(key)).SetValue(value))
	}
	return scList
}

func (trk *StateTracker) getState(key []byte) []byte {
	key = ConcatBytes(trk.keyPrefix, key)
	trk.setDependency(key)
	if value, ok := trk.changes[string(key)]; ok {
		return value
	}
	return trk.baseState.GetState(key)
}

func (trk *StateTracker) setDependency(key []byte) {
	if !trk.trackDep {
		return
	}
	trk.mtxDep.Lock()
	defer trk.mtxDep.Unlock()
	trk.dependencies[string(key)] = struct{}{}
}

func (trk *StateTracker) setState(key, value []byte) {
	key = ConcatBytes(trk.keyPrefix, key)
	keyStr := string(key)
	trk.changes[keyStr] = value
}

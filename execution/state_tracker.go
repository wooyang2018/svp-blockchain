// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package execution

import (
	"sync"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
)

// stateTracker tracks state changes in key order
// get latest changed state for each key
// get state from base state getter if no changes occured for a key
type stateTracker struct {
	keyPrefix []byte
	baseState common.StateGetter

	trackDep     bool
	dependencies map[string]struct{} // getState calls
	changes      map[string][]byte   // setState calls

	mtxChg sync.RWMutex
	mtxDep sync.RWMutex
}

func newStateTracker(state common.StateGetter, keyPrefix []byte) *stateTracker {
	return &stateTracker{
		keyPrefix: keyPrefix,
		baseState: state,

		dependencies: make(map[string]struct{}),
		changes:      make(map[string][]byte),
	}
}

func (trk *stateTracker) GetState(key []byte) []byte {
	trk.mtxChg.RLock()
	defer trk.mtxChg.RUnlock()
	return trk.getState(key)
}

func (trk *stateTracker) SetState(key, value []byte) {
	trk.mtxChg.Lock()
	defer trk.mtxChg.Unlock()
	trk.setState(key, value)
}

// spawn creates a new tracker with current tracker as base StateGetter
func (trk *stateTracker) spawn(keyPrefix []byte) *stateTracker {
	child := newStateTracker(trk, keyPrefix)
	child.trackDep = true
	return child
}

func (trk *stateTracker) hasDependencyChanges(child *stateTracker) bool {
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

func (trk *stateTracker) merge(child *stateTracker) {
	trk.mtxChg.Lock()
	defer trk.mtxChg.Unlock()

	child.mtxChg.RLock()
	defer child.mtxChg.RUnlock()

	for key, value := range child.changes {
		trk.setState([]byte(key), value)
	}
}

func (trk *stateTracker) getStateChanges() []*core.StateChange {
	trk.mtxChg.RLock()
	defer trk.mtxChg.RUnlock()

	scList := make([]*core.StateChange, 0, len(trk.changes))
	for key, value := range trk.changes {
		scList = append(scList, core.NewStateChange().SetKey([]byte(key)).SetValue(value))
	}
	return scList
}

func (trk *stateTracker) getState(key []byte) []byte {
	key = common.ConcatBytes(trk.keyPrefix, key)
	trk.setDependency(key)
	if value, ok := trk.changes[string(key)]; ok {
		return value
	}
	return trk.baseState.GetState(key)
}

func (trk *stateTracker) setDependency(key []byte) {
	if !trk.trackDep {
		return
	}
	trk.mtxDep.Lock()
	defer trk.mtxDep.Unlock()
	trk.dependencies[string(key)] = struct{}{}
}

func (trk *stateTracker) setState(key, value []byte) {
	key = common.ConcatBytes(trk.keyPrefix, key)
	keyStr := string(key)
	trk.changes[keyStr] = value
}

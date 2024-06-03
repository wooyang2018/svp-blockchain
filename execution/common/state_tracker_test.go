// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStateTrackerGetState(t *testing.T) {
	asrt := assert.New(t)

	ms := NewMapStateStore()
	trk := NewStateTracker(ms, nil)
	ms.SetState([]byte{1}, []byte{200})

	asrt.Equal([]byte{200}, trk.GetState([]byte{1}))
	asrt.Nil(trk.GetState([]byte{2}))

	trkChild := trk.Spawn(nil)
	asrt.Equal([]byte{200}, trkChild.GetState([]byte{1}), "child get state from root store")
	asrt.Nil(trkChild.GetState([]byte{2}))

	trk.SetState([]byte{1}, []byte{100})
	asrt.Equal([]byte{100}, trk.GetState([]byte{1}), "get latest state")
	asrt.Equal([]byte{100}, trkChild.GetState([]byte{1}), "child get latest state from parent")
}

func TestStateTrackerSetState(t *testing.T) {
	asrt := assert.New(t)

	ms := NewMapStateStore()
	trk := NewStateTracker(ms, nil)
	ms.SetState([]byte{1}, []byte{200})

	trk.SetState([]byte{1}, []byte{100})
	trk.SetState([]byte{1}, []byte{50})

	asrt.Equal([]byte{50}, trk.GetState([]byte{1}))

	scList := trk.GetStateChanges()
	asrt.Equal(1, len(scList))

	trk.SetState([]byte{3}, []byte{30})
	trk.SetState([]byte{2}, []byte{60})
	trk.setState([]byte{2}, []byte{20})
	trk.SetState([]byte{1}, []byte{10})

	asrt.Equal([]byte{10}, trk.GetState([]byte{1}))
	asrt.Equal([]byte{30}, trk.GetState([]byte{3}))
	asrt.Equal([]byte{20}, trk.GetState([]byte{2}))

	scList = trk.GetStateChanges()
	asrt.Equal(3, len(scList))
}

func TestStateTrackerMerge(t *testing.T) {
	asrt := assert.New(t)

	ms := NewMapStateStore()
	trk := NewStateTracker(ms, nil)

	trk.SetState([]byte{1}, []byte{200})
	trkChild := trk.Spawn(nil)
	trkChild.SetState([]byte{2}, []byte{20})
	trkChild.SetState([]byte{1}, []byte{10})

	asrt.Equal([]byte{20}, trkChild.GetState([]byte{2}))
	asrt.Equal([]byte{10}, trkChild.GetState([]byte{1}))

	asrt.Equal([]byte{200}, trk.GetState([]byte{1}), "child does not set parent state")

	trk.Merge(trkChild)

	asrt.Equal([]byte{10}, trk.GetState([]byte{1}))
	asrt.Equal([]byte{20}, trk.GetState([]byte{2}))

	scList := trk.GetStateChanges()
	asrt.Equal(2, len(scList))
}

func TestStateTrackerWithPrefix(t *testing.T) {
	asrt := assert.New(t)

	ms := NewMapStateStore()
	trk := NewStateTracker(ms, nil)
	trk.SetState([]byte{1, 1}, []byte{50})

	trkChild := trk.Spawn([]byte{1})
	asrt.Equal([]byte{50}, trkChild.GetState([]byte{1}))

	trkChild.SetState([]byte{1}, []byte{10})
	trkChild.SetState([]byte{2}, []byte{20})
	asrt.Equal([]byte{10}, trkChild.GetState([]byte{1}))
	asrt.Equal([]byte{20}, trkChild.GetState([]byte{2}))

	scList := trkChild.GetStateChanges()
	asrt.Equal(2, len(scList))

	trk.Merge(trkChild)
	asrt.Equal([]byte{10}, trk.GetState([]byte{1, 1}))
	asrt.Equal([]byte{20}, trk.GetState([]byte{1, 2}))
}

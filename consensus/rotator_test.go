// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package consensus

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/wooyang2018/posv-blockchain/core"
)

func setupTestRotator() (*rotator, *core.Proposal) {
	key0, _, resources := setupTestResources()
	b0 := core.NewBlock().Sign(key0)
	pro := core.NewProposal().SetBlock(b0).SetView(0).Sign(key0)
	q0 := core.NewQuorumCert().Build(key0, []*core.Vote{pro.Vote(key0)})
	pro.SetQuorumCert(q0)

	driver := &driver{
		resources:  resources,
		state:      newState(),
		proposalCh: make(chan *core.Proposal),
	}
	driver.setupInnerState(b0, q0)
	driver.state.setBlock(b0)
	driver.state.setQC(q0)

	rot := &rotator{
		resources: resources,
		config:    DefaultConfig,
		driver:    driver,
	}
	rot.config.DeltaTime = 500 * time.Millisecond
	return rot, pro
}

func TestRotator_changeView(t *testing.T) {
	asrt := assert.New(t)

	rot, _ := setupTestRotator()
	rot.start()
	defer rot.stop()

	msgSvc := new(MockMsgService)
	msgSvc.On("BroadcastProposal", mock.Anything).Return(nil)
	msgSvc.On("BroadcastQC", mock.Anything).Return(nil)
	rot.resources.MsgSvc = msgSvc

	rot.changeView()

	msgSvc.AssertExpectations(t)
	asrt.EqualValues(rot.driver.getViewChange(), 0)
	asrt.EqualValues(rot.driver.getLeaderIndex(), 1)
}

func Test_rotator_isNewViewApproval(t *testing.T) {
	asrt := assert.New(t)

	rot0, _ := setupTestRotator()
	rot1, _ := setupTestRotator()

	rot0.driver.setViewChange(1)
	rot1.driver.setViewChange(0)

	tests := []struct {
		name        string
		rot         *rotator
		proposerIdx uint32
		want        bool
	}{
		{"pending and same leader", rot0, 0, true},
		{"not pending and different leader", rot1, 1, true},
		{"pending and different leader", rot0, 1, false},
		{"not pending and same leader", rot1, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asrt.EqualValues(tt.want, tt.rot.isNewViewApproval(0, tt.proposerIdx))
		})
	}
}

func TestRotator_resetViewTimer(t *testing.T) {
	asrt := assert.New(t)

	rot, _ := setupTestRotator()
	rot.driver.setViewChange(1)
	rot.approveViewLeader(1, 1)

	asrt.EqualValues(rot.driver.getViewChange(), 0)
	asrt.EqualValues(rot.driver.getView(), 1)
	asrt.EqualValues(rot.driver.getLeaderIndex(), 1)
}

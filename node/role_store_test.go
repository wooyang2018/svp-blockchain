package node

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/execution/common"
	"github.com/wooyang2018/svp-blockchain/native/srole"
)

func TestMajorityCount(t *testing.T) {
	type args struct {
		validatorCount int
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{"single node", args{1}, 1},
		{"exact factor", args{4}, 3},  // n = 3f+1, f=1
		{"exact factor", args{10}, 7}, // f=3, m=10-3
		{"middle", args{12}, 9},       // f=3, m=12-3
		{"middle", args{14}, 10},      // f=4, m=14-4
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MajorityCount(tt.args.validatorCount)
			assert.Equal(t, tt.want, got)
		})
	}
}

func makeInitCtx() *common.MockCallContext {
	nodeCount := 4
	vlds := make([]*core.PublicKey, nodeCount)
	peers := make([]*Peer, nodeCount)
	genesis := &Genesis{
		Validators:  make([]string, nodeCount),
		StakeQuotas: make([]uint64, nodeCount),
		WindowSize:  4,
	}
	input := srole.InitInput{
		Size:  genesis.WindowSize,
		Peers: make([]*srole.Peer, nodeCount),
	}

	for i := 0; i < nodeCount; i++ {
		vlds[i] = core.GenerateKey(nil).PublicKey()
		genesis.Validators[i] = vlds[i].String()
		genesis.StakeQuotas[i] = 100
		pointAddr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", DefaultConfig.PointPort+i)
		topicAddr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", DefaultConfig.TopicPort+i)
		peers[i] = &Peer{vlds[i].Bytes(), pointAddr, topicAddr}
		input.Peers[i] = &srole.Peer{
			Addr:  vlds[i].Bytes(),
			Quota: genesis.StakeQuotas[i],
			Point: pointAddr,
			Topic: topicAddr,
		}
	}
	vs := NewRoleStore(genesis, peers)
	vs.SetHost(nil)
	core.SetSRole(vs)

	ctx := new(common.MockCallContext)
	ctx.MemStateStore = common.NewMemStateStore()
	ctx.MockSender = vlds[0].Bytes()
	ctx.MockInput, _ = json.Marshal(input)
	return ctx
}

func TestNativeCode(t *testing.T) {
	asrt := assert.New(t)
	jctx := new(srole.SRole)
	ctx := makeInitCtx()
	err := jctx.Init(ctx)
	asrt.NoError(err)

	input := &srole.Input{
		Method: "genesis",
	}
	ctx.MockInput, _ = json.Marshal(input)
	ret0, err := jctx.Query(ctx)
	asrt.NoError(err)

	vld := core.GenerateKey(nil).PublicKey()
	input = &srole.Input{
		Method: "add",
		Addr:   vld.Bytes(),
		Quota:  100,
		Point:  fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", DefaultConfig.PointPort+4),
		Topic:  fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", DefaultConfig.TopicPort+4),
	}
	ctx.MockInput, _ = json.Marshal(input)
	err = jctx.Invoke(ctx)
	asrt.NoError(err)

	input = &srole.Input{
		Method: "delete",
		Addr:   vld.Bytes(),
	}
	ctx.MockInput, _ = json.Marshal(input)
	err = jctx.Invoke(ctx)
	asrt.NoError(err)

	input = &srole.Input{
		Method: "genesis",
	}
	ctx.MockInput, _ = json.Marshal(input)
	ret1, err := jctx.Query(ctx)
	asrt.NoError(err)
	asrt.Equal(ret0, ret1)
}

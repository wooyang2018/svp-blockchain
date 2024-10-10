package node

import (
	"encoding/json"
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
	vlds := make([]*core.PublicKey, 4)
	genesis := &Genesis{
		Validators:  make([]string, 4),
		StakeQuotas: make([]uint64, 4),
		WindowSize:  4,
	}
	for i := range vlds {
		vlds[i] = core.GenerateKey(nil).PublicKey()
		genesis.Validators[i] = vlds[i].String()
		genesis.StakeQuotas[i] = 100
	}
	vs := NewRoleStore(genesis)
	core.SetSRole(vs)

	ctx := new(common.MockCallContext)
	ctx.MemStateStore = common.NewMemStateStore()
	ctx.MockSender = vlds[0].Bytes()
	input := srole.InitInput{}
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
		Method: "print",
	}
	ctx.MockInput, _ = json.Marshal(input)
	ret0, err := jctx.Query(ctx)
	asrt.NoError(err)

	vld := core.GenerateKey(nil).PublicKey()
	input = &srole.Input{
		Method: "add",
		Addr:   vld.Bytes(),
		Quota:  100,
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
		Method: "print",
	}
	ctx.MockInput, _ = json.Marshal(input)
	ret1, err := jctx.Query(ctx)
	asrt.NoError(err)
	asrt.Equal(ret0, ret1)
}

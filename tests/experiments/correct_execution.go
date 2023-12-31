// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package experiments

import (
	"fmt"

	"github.com/wooyang2018/svp-blockchain/core"
	"github.com/wooyang2018/svp-blockchain/tests/cluster"
	"github.com/wooyang2018/svp-blockchain/tests/testutil"
)

type CorrectExecution struct{}

func (expm *CorrectExecution) Name() string {
	return "correct_execution"
}

func (expm *CorrectExecution) Run(cls *cluster.Cluster) error {
	jc := testutil.NewPCoinClient(nil, 0, 0, "")

	if err := jc.SetupOnCluster(cls); err != nil {
		return fmt.Errorf("setup pcoin failed, %w", err)
	}
	acc1 := core.GenerateKey(nil)
	acc2 := core.GenerateKey(nil)

	i, err := testutil.SubmitTxAndWait(cls, jc.MakeMintTx(acc1.PublicKey(), 100))
	if err != nil {
		return fmt.Errorf("submit mint tx failed, %w", err)
	}
	b1, err := jc.QueryBalance(cls.GetNode(i), acc1.PublicKey())
	if err != nil {
		return fmt.Errorf("query balance failed, %w", err)
	}
	if b1 != 100 {
		return fmt.Errorf("wrong mint balance, expected 100, got %d", b1)
	}

	// transfer 40. acc1 -> acc2
	i, err = testutil.SubmitTxAndWait(cls, jc.MakeTransferTx(acc1, acc2.PublicKey(), 40))
	if err != nil {
		return fmt.Errorf("submit transfer tx failed, %w", err)
	}
	b1, err = jc.QueryBalance(cls.GetNode(i), acc1.PublicKey())
	if err != nil {
		return fmt.Errorf("query balance failed, %w", err)
	}
	b2, err := jc.QueryBalance(cls.GetNode(i), acc2.PublicKey())
	if err != nil {
		return fmt.Errorf("query balance failed, %w", err)
	}
	if b1 != 60 {
		return fmt.Errorf("wrong transfer balance b1, expected 60, got %d", b1)
	}
	if b2 != 40 {
		return fmt.Errorf("wrong transfer balance b2, expected 40, got %d", b2)
	}

	// transfer 100. acc1 -> acc2 (invalid)
	i, err = testutil.SubmitTxAndWait(cls, jc.MakeTransferTx(acc1, acc2.PublicKey(), 100))
	if err != nil {
		return fmt.Errorf("submit transfer tx failed, %w", err)
	}
	b1, err = jc.QueryBalance(cls.GetNode(i), acc1.PublicKey())
	if err != nil {
		return fmt.Errorf("query balance failed, %w", err)
	}
	b2, err = jc.QueryBalance(cls.GetNode(i), acc2.PublicKey())
	if err != nil {
		return fmt.Errorf("query balance failed, %w", err)
	}
	if b1 != 60 {
		return fmt.Errorf("b1 balance should not change, expected 60, got %d", b1)
	}
	if b2 != 40 {
		return fmt.Errorf("b2 balance should not change, expected 40, got %d", b2)
	}

	fmt.Println("All execution correct.")
	return nil
}

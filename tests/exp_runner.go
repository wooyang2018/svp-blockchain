// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/wooyang2018/svp-blockchain/tests/cluster"
	"github.com/wooyang2018/svp-blockchain/tests/health"
	"github.com/wooyang2018/svp-blockchain/tests/testutil"
)

type Experiment interface {
	Name() string
	Run(c *cluster.Cluster) error
}

type ExperimentRunner struct {
	experiments []Experiment
	cfactory    cluster.ClusterFactory
}

func (r *ExperimentRunner) Run() (pass, fail int) {
	bold := color.New(color.Bold)
	boldGrean := color.New(color.Bold, color.FgGreen)
	boldRed := color.New(color.Bold, color.FgRed)

	fmt.Println("\nRunning Experiments")
	for i, expm := range r.experiments {
		bold.Printf("%3d. %s\n", i, expm.Name())
	}

	killed := make(chan os.Signal, 1)
	signal.Notify(killed, os.Interrupt)

	for i, expm := range r.experiments {
		bold.Printf("\nExperiment %d. %s\n", i, expm.Name())
		if err := r.runSingleExperiment(expm); err != nil {
			fail++
			fmt.Printf("%s %s\n", boldRed.Sprint("FAIL"), bold.Sprint(expm.Name()))
			fmt.Printf("error: %+v\n", err)
		} else {
			pass++
			fmt.Printf("%s %s\n", boldGrean.Sprint("PASS"), bold.Sprint(expm.Name()))
		}
		select {
		case <-killed:
			return pass, fail
		default:
		}
	}
	return pass, fail
}

func (r *ExperimentRunner) runSingleExperiment(expm Experiment) error {
	var cls *cluster.Cluster
	var err error
	done := make(chan struct{})
	loadCtx, stopLoad := context.WithCancel(context.Background())
	go func() {
		defer close(done)
		fmt.Println("Setting up a new cluster")
		if cls, err = r.cfactory.SetupCluster(expm.Name()); err != nil {
			return
		}
		cls.EmptyChainCode = EmptyChainCode
		cls.CheckRotation = CheckRotation
		fmt.Println("Starting cluster")
		cls.Stop() // to make sure no existing process keeps running
		if err = cls.Start(); err != nil {
			return
		}
		fmt.Println("Started cluster")
		testutil.Sleep(20 * time.Second)

		fmt.Println("Setting up load generator")
		if err = testutil.LoadGen.SetupOnCluster(cls); err != nil {
			return
		}
		go testutil.LoadGen.Run(loadCtx)
		fmt.Println("Load generator running")
		testutil.Sleep(20 * time.Second)

		if err = health.CheckAllNodes(cls); err != nil {
			fmt.Printf("health check failed before experiment, %+v\n", err)
			cls.Stop()
			os.Exit(1)
			return
		}

		fmt.Println("==> Running experiment")
		if err = expm.Run(cls); err != nil {
			fmt.Println("==> Experiment failed")
			return
		}
		fmt.Println("==> Finished experiment")
		err = health.CheckAllNodes(cls)
	}()

	killed := make(chan os.Signal, 1)
	signal.Notify(killed, os.Interrupt, syscall.SIGTERM)

	select {
	case s := <-killed:
		fmt.Println("\nGot signal:", s)
		err = errors.New("interrupted")
		if cls != nil {
			fmt.Println("Removing effects")
			cls.RemoveEffects()
		}
	case <-done:
	}
	stopLoad()
	if cls != nil {
		fmt.Println("Stopping cluster")
		cls.Stop()
		fmt.Println("Stopped cluster")
	}
	return err
}

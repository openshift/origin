package main

import (
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"k8s.io/apiserver/pkg/util/logs"

	"github.com/openshift/origin/pkg/cmd/server/start"
	"github.com/openshift/origin/pkg/cmd/util/serviceability"
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	logs.InitLogs()
	defer logs.FlushLogs()
	defer serviceability.BehaviorOnPanic(os.Getenv("OPENSHIFT_ON_PANIC"))()
	defer serviceability.Profile(os.Getenv("OPENSHIFT_PROFILE")).Stop()

	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	command := NewHyperShiftCommand()
	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func NewHyperShiftCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hypershift",
		Short: "Combined server command for OpenShift",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
			os.Exit(1)
		},
	}

	startEtcd, _ := start.NewCommandStartEtcdServer(start.RecommendedStartEtcdServerName, "hypershift", os.Stdout, os.Stderr)
	startEtcd.Deprecated = "will be removed in 3.10"
	startEtcd.Hidden = true
	cmd.AddCommand(startEtcd)

	return cmd
}

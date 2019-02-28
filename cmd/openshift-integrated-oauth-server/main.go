package main

import (
	goflag "flag"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	genericapiserver "k8s.io/apiserver/pkg/server"
	utilflag "k8s.io/apiserver/pkg/util/flag"
	"k8s.io/apiserver/pkg/util/logs"

	"github.com/openshift/library-go/pkg/serviceability"
	"github.com/openshift/origin/pkg/cmd/openshift-integrated-oauth-server"
	"github.com/openshift/origin/pkg/version"
)

func main() {
	stopCh := genericapiserver.SetupSignalHandler()

	rand.Seed(time.Now().UTC().UnixNano())

	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	logs.InitLogs()
	defer logs.FlushLogs()
	defer serviceability.BehaviorOnPanic(os.Getenv("OPENSHIFT_ON_PANIC"), version.Get())()
	defer serviceability.Profile(os.Getenv("OPENSHIFT_PROFILE")).Stop()

	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	command := NewOpenshiftIntegratedOAuthServerCommand(stopCh)
	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func NewOpenshiftIntegratedOAuthServerCommand(stopCh <-chan struct{}) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "openshift-integrated-oauth-server",
		Short: "Command for the OpenShift integrated OAuth server",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
			os.Exit(1)
		},
	}

	startOsin := openshift_integrated_oauth_server.NewOsinServer(os.Stdout, os.Stderr, stopCh)
	cmd.AddCommand(startOsin)

	return cmd
}

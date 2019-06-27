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
	utilflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"

	"github.com/openshift/library-go/pkg/serviceability"
	openshift_integrated_oauth_server "github.com/openshift/oauth-server/pkg/cmd/oauth-server"
	"github.com/openshift/openshift-apiserver/pkg/cmd/openshift-apiserver"
	"github.com/openshift/openshift-controller-manager/pkg/cmd/openshift-controller-manager"
	"github.com/openshift/sdn/pkg/openshift-network-controller"

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

	command := NewHyperShiftCommand(stopCh)
	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func NewHyperShiftCommand(stopCh <-chan struct{}) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hypershift",
		Short: "Combined server command for OpenShift",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
			os.Exit(1)
		},
	}

	startOpenShiftAPIServer := openshift_apiserver.NewOpenShiftAPIServerCommand(openshift_apiserver.RecommendedStartAPIServerName, os.Stdout, os.Stderr, stopCh)
	startOpenShiftAPIServer.Deprecated = "will be removed in 4.2"
	startOpenShiftAPIServer.Hidden = true
	cmd.AddCommand(startOpenShiftAPIServer)

	startOpenShiftControllerManager := openshift_controller_manager.NewOpenShiftControllerManagerCommand(openshift_controller_manager.RecommendedStartControllerManagerName, os.Stdout, os.Stderr)
	startOpenShiftControllerManager.Deprecated = "will be removed in 4.2"
	startOpenShiftControllerManager.Hidden = true
	cmd.AddCommand(startOpenShiftControllerManager)

	startOpenShiftNetworkController := openshift_network_controller.NewOpenShiftNetworkControllerCommand(openshift_network_controller.RecommendedStartNetworkControllerName, "hypershift", os.Stdout, os.Stderr)
	cmd.AddCommand(startOpenShiftNetworkController)

	startOsin := openshift_integrated_oauth_server.NewOsinServer(os.Stdout, os.Stderr, stopCh)
	startOsin.Use = "openshift-osinserver"
	startOsin.Deprecated = "will be removed in 4.0"
	startOsin.Hidden = true
	cmd.AddCommand(startOsin)

	return cmd
}

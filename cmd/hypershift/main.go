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
	"github.com/openshift/origin/pkg/cmd/openshift-apiserver"
	"github.com/openshift/origin/pkg/cmd/openshift-controller-manager"
	"github.com/openshift/origin/pkg/cmd/openshift-etcd"
	"github.com/openshift/origin/pkg/cmd/openshift-integrated-oauth-server"
	"github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver"
	"github.com/openshift/origin/pkg/cmd/openshift-network-controller"
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

	startEtcd, _ := openshift_etcd.NewCommandStartEtcdServer(openshift_etcd.RecommendedStartEtcdServerName, "hypershift", os.Stdout, os.Stderr)
	startEtcd.Deprecated = "will be removed in 3.10"
	startEtcd.Hidden = true
	cmd.AddCommand(startEtcd)

	startOpenShiftAPIServer := openshift_apiserver.NewOpenShiftAPIServerCommand(openshift_apiserver.RecommendedStartAPIServerName, "hypershift", os.Stdout, os.Stderr, stopCh)
	cmd.AddCommand(startOpenShiftAPIServer)

	startOpenShiftKubeAPIServer := openshift_kube_apiserver.NewOpenShiftKubeAPIServerServerCommand(openshift_kube_apiserver.RecommendedStartAPIServerName, "hypershift", os.Stdout, os.Stderr, stopCh)
	cmd.AddCommand(startOpenShiftKubeAPIServer)

	startOpenShiftControllerManager := openshift_controller_manager.NewOpenShiftControllerManagerCommand(openshift_controller_manager.RecommendedStartControllerManagerName, "hypershift", os.Stdout, os.Stderr)
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

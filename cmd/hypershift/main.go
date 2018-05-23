package main

import (
	goflag "flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/openshift/origin/pkg/cmd/openshift-experimental"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	utilflag "k8s.io/apiserver/pkg/util/flag"
	"k8s.io/apiserver/pkg/util/logs"

	"github.com/openshift/origin/pkg/cmd/openshift-apiserver"
	"github.com/openshift/origin/pkg/cmd/openshift-controller-manager"
	"github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver"
	"github.com/openshift/origin/pkg/cmd/server/start"
	"github.com/openshift/origin/pkg/cmd/util/serviceability"
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

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

// addPidFileHandler handles writing PID files for hypershift commands
// TODO: This should be not needed after 3.10 -> 3.11 transition to operators. In 3.10 this is needed to handle systemd shims.
func addPidFileHandler(cmd *cobra.Command) {
	var pidFile string
	flags := cmd.Flags()
	flags.StringVar(&pidFile, "pid-file", "", "A file name where the command should write its PID.")
	flags.MarkHidden("pid-file")

	cmd.PreRun = func(c *cobra.Command, args []string) {
		if len(pidFile) == 0 {
			return
		}
		if err := ioutil.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0666); err != nil {
			fmt.Fprintf(os.Stderr, fmt.Sprintf("error writing PID to file %q: %v", pidFile, err))
			os.Exit(1)
		}
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

	startOpenShiftAPIServer := openshift_apiserver.NewOpenShiftAPIServerCommand(openshift_apiserver.RecommendedStartAPIServerName, "hypershift", os.Stdout, os.Stderr)
	addPidFileHandler(startOpenShiftAPIServer)
	cmd.AddCommand(startOpenShiftAPIServer)

	startOpenShiftKubeAPIServer := openshift_kube_apiserver.NewOpenShiftKubeAPIServerServerCommand(openshift_kube_apiserver.RecommendedStartAPIServerName, "hypershift", os.Stdout, os.Stderr)
	addPidFileHandler(startOpenShiftKubeAPIServer)
	cmd.AddCommand(startOpenShiftKubeAPIServer)

	startOpenShiftControllerManager := openshift_controller_manager.NewOpenShiftControllerManagerCommand(openshift_controller_manager.RecommendedStartControllerManagerName, "hypershift", os.Stdout, os.Stderr)
	addPidFileHandler(startOpenShiftControllerManager)
	cmd.AddCommand(startOpenShiftControllerManager)

	experimental := openshift_experimental.NewExperimentalCommand(os.Stdout, os.Stderr)
	cmd.AddCommand(experimental)

	return cmd
}

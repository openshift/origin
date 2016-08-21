package kubernetes

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/cli/cmd"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

const kubernetesLong = `
Start Kubernetes server components

The primary Kubernetes server components can be started individually using their direct
arguments. No configuration settings will be used when launching these components.
`

func NewCommand(name, fullName string, out io.Writer) *cobra.Command {
	cmds := &cobra.Command{
		Use:   name,
		Short: "Kubernetes server components",
		Long:  fmt.Sprintf(kubernetesLong),
		Run: func(c *cobra.Command, args []string) {
			c.SetOutput(os.Stdout)
			c.Help()
		},
	}

	cmds.AddCommand(NewAPIServerCommand("apiserver", fullName+" apiserver", out))
	cmds.AddCommand(NewControllersCommand("controller-manager", fullName+" controller-manager", out))
	cmds.AddCommand(NewKubeletCommand("kubelet", fullName+" kubelet", out))
	cmds.AddCommand(NewProxyCommand("proxy", fullName+" proxy", out))
	cmds.AddCommand(NewSchedulerCommand("scheduler", fullName+" scheduler", out))
	if "hyperkube" == fullName {
		cmds.AddCommand(cmd.NewCmdVersion(fullName, nil, out, cmd.VersionOptions{}))
	}

	return cmds
}

func startProfiler() {
	if cmdutil.Env("OPENSHIFT_PROFILE", "") == "web" {
		go func() {
			runtime.SetBlockProfileRate(1)
			profile_port := cmdutil.Env("OPENSHIFT_PROFILE_PORT", "6060")
			glog.Infof(fmt.Sprintf("Starting profiling endpoint at http://127.0.0.1:%s/debug/pprof/", profile_port))
			glog.Fatal(http.ListenAndServe(fmt.Sprintf("127.0.0.1:%s", profile_port), nil))
		}()
	}
}

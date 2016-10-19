package kubernetes

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/cli/cmd/version"
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
		cmds.AddCommand(version.NewCmdVersion(fullName, nil, out, version.VersionOptions{}))
	}

	return cmds
}

func startProfiler() {
	if cmdutil.Env("OPENSHIFT_PROFILE", "") == "web" {
		go func() {
			runtime.SetBlockProfileRate(1)
			profilePort := cmdutil.Env("OPENSHIFT_PROFILE_PORT", "6060")
			profileHost := cmdutil.Env("OPENSHIFT_PROFILE_HOST", "127.0.0.1")
			glog.Infof(fmt.Sprintf("Starting profiling endpoint at http://%s:%s/debug/pprof/", profileHost, profilePort))
			glog.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%s", profileHost, profilePort), nil))
		}()
	}
}

package kubernetes

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/version"
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
		cmds.AddCommand(version.NewVersionCommand(fullName))
	}

	return cmds
}

func startProfiler() {
	if cmdutil.Env("OPENSHIFT_PROFILE", "") == "web" {
		go func() {
			glog.Infof("Starting profiling endpoint at http://127.0.0.1:6060/debug/pprof/")
			glog.Fatal(http.ListenAndServe("127.0.0.1:6060", nil))
		}()
	}
}

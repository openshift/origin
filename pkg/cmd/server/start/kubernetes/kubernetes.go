package kubernetes

import (
	"fmt"
	"io"
	"net/http"
	"runtime"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	proxyapp "k8s.io/kubernetes/cmd/kube-proxy/app"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/cli/cmd"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

const kubernetesLong = `
Start Kubernetes server components

The primary Kubernetes server components can be started individually using their direct
arguments. No configuration settings will be used when launching these components.
`

func NewCommand(name, fullName string, out, errOut io.Writer) *cobra.Command {
	cmds := &cobra.Command{
		Use:   name,
		Short: "Kubernetes server components",
		Long:  fmt.Sprintf(kubernetesLong),
		Run:   kcmdutil.DefaultSubCommandRun(errOut),
	}

	cmds.AddCommand(NewAPIServerCommand("apiserver", fullName+" apiserver", out))
	cmds.AddCommand(NewControllersCommand("controller-manager", fullName+" controller-manager", out))
	cmds.AddCommand(NewKubeletCommand("kubelet", fullName+" kubelet", out))
	cmds.AddCommand(proxyapp.NewProxyCommand())
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
			profilePort := cmdutil.Env("OPENSHIFT_PROFILE_PORT", "6060")
			profileHost := cmdutil.Env("OPENSHIFT_PROFILE_HOST", "127.0.0.1")
			glog.Infof(fmt.Sprintf("Starting profiling endpoint at http://%s:%s/debug/pprof/", profileHost, profilePort))
			glog.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%s", profileHost, profilePort), nil))
		}()
	}
}

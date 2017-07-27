package kubefed

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"k8s.io/apiserver/pkg/util/flag"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/federation/pkg/kubefed"
	kubefedinit "k8s.io/kubernetes/federation/pkg/kubefed/init"
	"k8s.io/kubernetes/federation/pkg/kubefed/util"
	kubectl "k8s.io/kubernetes/pkg/kubectl/cmd"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"

	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/oc/cli/cmd"
	"github.com/openshift/origin/pkg/version"
)

// This file was copied from vendor/k8s.io/kubernetes/federation/pkg/kubefed and
// modified to support the openshift version command as per the inline comments.

var (
	// serverImageName is the name of the default image (without version)
	// used for the federation services (api and controller manager).  It
	// should be set during build via -ldflags.
	serverImageName string

	// defaultEtcImage is the default image (including version) used to run
	// etcd for the federation apiserver.  It should be set during build via
	// -ldflags.
	defaultEtcdImage string
)

// NewKubeFedCommand creates the `kubefed` command and its nested children.
func NewKubeFedCommand(in io.Reader, out, err io.Writer) *cobra.Command {
	defaultServerImage := fmt.Sprintf("%s:%s", serverImageName, version.Get())

	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   "kubefed",
		Short: "kubefed controls an OpenShift Cluster Federation",
		Long: templates.LongDesc(`
      kubefed controls an OpenShift Cluster Federation.

      Find more information at https://github.com/openshift/origin.`),
		Run: runHelp,
	}

	// Use an openshift command factory to ensure CmdNewVersion will work.
	// It is interface compatible with the kube equivalent, so any calls to
	// kube code will continue to work.
	f := osclientcmd.New(cmds.PersistentFlags())

	// From this point and forward we get warnings on flags that contain "_" separators
	cmds.SetGlobalNormalizationFunc(flag.WarnWordSepNormalizeFunc)

	groups := templates.CommandGroups{
		{
			Message: "Basic Commands:",
			Commands: []*cobra.Command{
				kubefedinit.NewCmdInit(out, util.NewAdminConfig(clientcmd.NewDefaultPathOptions()), defaultServerImage, defaultEtcdImage),
				kubefed.NewCmdJoin(f, out, util.NewAdminConfig(clientcmd.NewDefaultPathOptions())),
				kubefed.NewCmdUnjoin(f, out, err, util.NewAdminConfig(clientcmd.NewDefaultPathOptions())),
			},
		},
	}
	groups.Add(cmds)

	filters := []string{
		"options",
	}
	templates.ActsAsRootCommand(cmds, filters, groups...)

	// Use the openshift-specific version command
	cmds.AddCommand(cmd.NewCmdVersion("kubefed", f, out, cmd.VersionOptions{PrintClientFeatures: true}))

	cmds.AddCommand(kubectl.NewCmdOptions(out))

	return cmds
}

func runHelp(cmd *cobra.Command, args []string) {
	cmd.Help()
}

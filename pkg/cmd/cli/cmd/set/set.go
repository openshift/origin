package set

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/kubectl/cmd/set"

	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	setLong = `
Configure application resources

These commands help you make changes to existing application resources.`
)

// NewCmdSet exposes commands for modifying objects.
func NewCmdSet(fullName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	set := &cobra.Command{
		Use:   "set COMMAND",
		Short: "Commands that help set specific features on objects",
		Long:  setLong,
		Run:   cmdutil.DefaultSubCommandRun(out),
	}

	name := fmt.Sprintf("%s set", fullName)

	groups := templates.CommandGroups{
		{
			Message: "Replication controllers, deployments, and daemon sets:",
			Commands: []*cobra.Command{
				NewCmdEnv(name, f, in, out),
				NewCmdVolume(name, f, out, errout),
				NewCmdProbe(name, f, out, errout),
				NewCmdDeploymentHook(name, f, out, errout),
				NewCmdImage(name, f, out),
			},
		},
		{
			Message: "Manage application flows:",
			Commands: []*cobra.Command{
				NewCmdTriggers(name, f, out, errout),
				NewCmdBuildHook(name, f, out, errout),
			},
		},
		{
			Message: "Control load balancing:",
			Commands: []*cobra.Command{
				NewCmdRouteBackends(name, f, out, errout),
			},
		},
	}
	groups.Add(set)
	templates.ActsAsRootCommand(set, []string{"options"}, groups...)
	return set
}

const (
	setImageLong = `
Update existing container image(s) of resources.
`

	setImageExample = `  # Set a deployment configs's nginx container image to 'nginx:1.9.1', and its busybox container image to 'busybox'.
  %[1]s image dc/nginx busybox=busybox nginx=nginx:1.9.1

  # Update all deployments' and rc's nginx container's image to 'nginx:1.9.1'
  %[1]s image deployments,rc nginx=nginx:1.9.1 --all

  # Update image of all containers of daemonset abc to 'nginx:1.9.1'
  %[1]s image daemonset abc *=nginx:1.9.1

  # Print result (in yaml format) of updating nginx container image from local file, without hitting the server
  %[1]s image -f path/to/file.yaml nginx=nginx:1.9.1 --local -o yaml`
)

// NewCmdImage is a wrapper for the Kubernetes CLI set image command
func NewCmdImage(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := set.NewCmdImage(f.Factory, out)
	cmd.Long = setImageLong
	cmd.Example = fmt.Sprintf(setImageExample, fullName)

	flags := cmd.Flags()
	f.ImageResolutionOptions.Bind(flags)

	return cmd
}

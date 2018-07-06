package set

import (
	"fmt"

	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/kubectl/cmd/set"
	ktemplates "k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	"github.com/openshift/origin/pkg/cmd/templates"
)

var (
	setLong = ktemplates.LongDesc(`
		Configure application resources

		These commands help you make changes to existing application resources.`)
)

// NewCmdSet exposes commands for modifying objects.
func NewCmdSet(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	set := &cobra.Command{
		Use:   "set COMMAND",
		Short: "Commands that help set specific features on objects",
		Long:  setLong,
		Run:   kcmdutil.DefaultSubCommandRun(streams.ErrOut),
	}

	name := fmt.Sprintf("%s set", fullName)

	groups := ktemplates.CommandGroups{
		{
			Message: "Replication controllers, deployments, and daemon sets:",
			Commands: []*cobra.Command{
				NewCmdEnv(name, f, streams),
				NewCmdResources(name, f, streams),
				NewCmdVolume(name, f, streams),
				// TODO: this seems reasonable to upstream
				NewCmdProbe(name, f, streams),
				NewCmdDeploymentHook(name, f, streams),
				NewCmdImage(name, f, streams),
			},
		},
		{
			Message: "Manage secrets:",
			Commands: []*cobra.Command{
				NewCmdBuildSecret(name, f, streams),
			},
		},
		{
			Message: "Manage application flows:",
			Commands: []*cobra.Command{
				NewCmdImageLookup(name, fullName, f, streams),
				NewCmdTriggers(name, f, streams),
				NewCmdBuildHook(name, f, streams),
			},
		},
		{
			Message: "Control load balancing:",
			Commands: []*cobra.Command{
				NewCmdRouteBackends(name, f, streams),
			},
		},
	}
	groups.Add(set)
	templates.ActsAsRootCommand(set, []string{"options"}, groups...)
	return set
}

var (
	setImageLong = ktemplates.LongDesc(`
Update existing container image(s) of resources.`)

	setImageExample = ktemplates.Examples(`
	  # Set a deployment configs's nginx container image to 'nginx:1.9.1', and its busybox container image to 'busybox'.
	  %[1]s image dc/nginx busybox=busybox nginx=nginx:1.9.1

	  # Set a deployment configs's app container image to the image referenced by the imagestream tag 'openshift/ruby:2.3'.
	  %[1]s image dc/myapp app=openshift/ruby:2.3 --source=imagestreamtag

	  # Update all deployments' and rc's nginx container's image to 'nginx:1.9.1'
	  %[1]s image deployments,rc nginx=nginx:1.9.1 --all

	  # Update image of all containers of daemonset abc to 'nginx:1.9.1'
	  %[1]s image daemonset abc *=nginx:1.9.1

	  # Print result (in yaml format) of updating nginx container image from local file, without hitting the server
	  %[1]s image -f path/to/file.yaml nginx=nginx:1.9.1 --local -o yaml`)
)

// NewCmdImage is a wrapper for the Kubernetes CLI set image command
func NewCmdImage(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := set.NewCmdImage(f, streams)
	cmd.Long = setImageLong
	cmd.Example = fmt.Sprintf(setImageExample, fullName)

	return cmd
}

var (
	setResourcesLong = ktemplates.LongDesc(`
Specify compute resource requirements (cpu, memory) for any resource that defines a pod template. If a pod is successfully schedualed it is guaranteed the amount of resource requested, but may burst up to its specified limits.

For each compute resource, if a limit is specified and a request is omitted, the request will default to the limit.

Possible resources include (case insensitive):
"ReplicationController", "Deployment", "DaemonSet", "Job", "ReplicaSet", "DeploymentConfigs"`)

	setResourcesExample = ktemplates.Examples(`
# Set a deployments nginx container cpu limits to "200m and memory to 512Mi"

%[1]s resources deployment nginx -c=nginx --limits=cpu=200m,memory=512Mi


# Set the resource request and limits for all containers in nginx

%[1]s resources deployment nginx --limits=cpu=200m,memory=512Mi --requests=cpu=100m,memory=256Mi

# Remove the resource requests for resources on containers in nginx

%[1]s resources deployment nginx --limits=cpu=0,memory=0 --requests=cpu=0,memory=0

# Print the result (in yaml format) of updating nginx container limits from a local, without hitting the server

%[1]s resources -f path/to/file.yaml --limits=cpu=200m,memory=512Mi --local -o yaml`)
)

// NewCmdResources is a wrapper for the Kubernetes CLI set resources command
func NewCmdResources(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := set.NewCmdResources(f, streams)
	cmd.Long = setResourcesLong
	cmd.Example = fmt.Sprintf(setResourcesExample, fullName)

	return cmd
}

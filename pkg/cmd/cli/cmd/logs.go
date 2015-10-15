package cmd

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	kcmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/labels"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	logsLong = `
Print the logs for a container in a pod

If the pod has only one container, the container name is optional.`

	logsExample = `  # Returns snapshot of ruby-container logs from pod 123456-7890.
  $ %[1]s logs 123456-7890 -c ruby-container

  # Starts streaming of ruby-container logs from pod 123456-7890.
  $ %[1]s logs -f 123456-7890 -c ruby-container

  # Starts streaming the logs of the most recent build of the openldap BuildConfig.
  $ %[1]s logs -f bc/openldap`
)

type OpenShiftLogsOptions struct {
	KubeClient   *kclient.Client
	OriginClient *client.Client

	Namespace      string
	ResourceString string
	ContainerName  string
	Follow         bool
	Interactive    bool
	Previous       bool
	Out            io.Writer
}

// NewCmdLogs creates a new pod log command
func NewCmdLogs(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	o := &OpenShiftLogsOptions{
		Out:         out,
		Interactive: true,
	}

	cmd := &cobra.Command{
		Use:     "logs [-f] RESOURCE",
		Short:   "Print the logs for a resource.",
		Long:    logsLong,
		Example: fmt.Sprintf(logsExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, out, cmd, args))
			if err := o.Validate(); err != nil {
				cmdutil.CheckErr(cmdutil.UsageError(cmd, err.Error()))
			}
			cmdutil.CheckErr(o.RunLog())

		},
		Aliases: []string{"log"},
	}
	cmd.Flags().BoolVarP(&o.Follow, "follow", "f", o.Follow, "Specify if the logs should be streamed.")
	cmd.Flags().BoolVar(&o.Interactive, "interactive", o.Interactive, "If true, prompt the user for input when required. Default true.")
	cmd.Flags().BoolVarP(&o.Previous, "previous", "p", o.Previous, "If true, print the logs for the previous instance of the container in a pod if it exists.")
	cmd.Flags().StringVarP(&o.ContainerName, "container", "c", o.ContainerName, "Container name")
	return cmd
}

func (o *OpenShiftLogsOptions) Complete(f *clientcmd.Factory, out io.Writer, cmd *cobra.Command, args []string) error {
	switch len(args) {
	case 0:
		return cmdutil.UsageError(cmd, "RESOURCE is required for log")

	case 1:
		o.ResourceString = args[0]
	case 2:
		o.ResourceString = args[0]
		o.ContainerName = args[1]

	default:
		return cmdutil.UsageError(cmd, "log RESOURCE")
	}

	var err error
	o.Namespace, _, err = f.DefaultNamespace()
	if err != nil {
		return err
	}
	o.OriginClient, o.KubeClient, err = f.Clients()
	if err != nil {
		return err
	}

	return nil
}

func (o *OpenShiftLogsOptions) Validate() error {
	if len(o.ResourceString) == 0 {
		return errors.New("RESOURCE must be specified")
	}

	return nil
}

func (o *OpenShiftLogsOptions) RunLog() error {
	kLogsOptions := &kcmd.LogsOptions{
		Client: o.KubeClient,

		PodNamespace:  o.Namespace,
		ContainerName: o.ContainerName,
		Follow:        o.Follow,
		Interactive:   o.Interactive,
		Previous:      o.Previous,
		Out:           o.Out,
	}

	resourceType := "pod"
	resourceName := o.ResourceString
	tokens := strings.SplitN(o.ResourceString, "/", 2)
	if len(tokens) == 2 {
		resourceType = tokens[0]
		resourceName = tokens[1]
	}
	resourceType = strings.ToLower(resourceType)

	// if we're requesting a pod, delegate directly to kubectl logs
	if (resourceType == "pods") || (resourceType == "pod") || len(o.ContainerName) > 0 {
		kLogsOptions.PodName = resourceName
		return kLogsOptions.RunLog()
	}

	switch resourceType {
	case "bc", "buildconfig", "buildconfigs":
		buildsForBCSelector := labels.SelectorFromSet(map[string]string{buildapi.BuildConfigLabel: resourceName})
		builds, err := o.OriginClient.Builds(o.Namespace).List(buildsForBCSelector, fields.Everything())
		if err != nil {
			return err
		}
		if len(builds.Items) == 0 {
			return fmt.Errorf("no builds found for %v", o.ResourceString)
		}

		sort.Sort(sort.Reverse(buildapi.BuildSliceByCreationTimestamp(builds.Items)))

		return o.runLogsForBuild(&builds.Items[0])

	case "build", "builds":
		build, err := o.OriginClient.Builds(o.Namespace).Get(resourceName)
		if err != nil {
			return err
		}
		return o.runLogsForBuild(build)

	default:
		return fmt.Errorf("cannot display logs for resource type %v", resourceType)
	}
}

// TODO I would recommend finding the Pod in this method and delegating the log call to the upstream LogsOptions
// to take advantage of all of their extra options.
func (o *OpenShiftLogsOptions) runLogsForBuild(build *buildapi.Build) error {
	opts := buildapi.BuildLogOptions{
		Follow: o.Follow,
		NoWait: false,
	}

	readCloser, err := o.OriginClient.BuildLogs(build.Namespace).Get(build.Name, opts).Stream()
	if err != nil {
		return err
	}
	defer readCloser.Close()

	_, err = io.Copy(o.Out, readCloser)
	return err
}

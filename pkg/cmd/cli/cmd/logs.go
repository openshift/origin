package cmd

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
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
	OriginClient *client.Client

	Namespace      string
	ResourceString string

	KubeLogOptions *kcmd.LogsOptions
}

// NewCmdLogs creates a new pod log command
func NewCmdLogs(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	o := &OpenShiftLogsOptions{
		KubeLogOptions: &kcmd.LogsOptions{
			Out:  out,
			Tail: -1,
		},
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

	cmd.Flags().BoolVarP(&o.KubeLogOptions.Follow, "follow", "f", o.KubeLogOptions.Follow, "Specify if the logs should be streamed.")
	cmd.Flags().BoolVar(&o.KubeLogOptions.Timestamps, "timestamps", o.KubeLogOptions.Timestamps, "Include timestamps on each line in the log output")
	cmd.Flags().Bool("interactive", true, "If true, prompt the user for input when required. Default true.")
	cmd.Flags().MarkDeprecated("interactive", "This flag is no longer respected and there is no replacement.")
	cmd.Flags().IntVar(&o.KubeLogOptions.LimitBytes, "limit-bytes", o.KubeLogOptions.LimitBytes, "Maximum bytes of logs to return. Defaults to no limit.")
	cmd.Flags().BoolVarP(&o.KubeLogOptions.Previous, "previous", "p", o.KubeLogOptions.Previous, "If true, print the logs for the previous instance of the container in a pod if it exists.")
	cmd.Flags().IntVar(&o.KubeLogOptions.Tail, "tail", o.KubeLogOptions.Tail, "Lines of recent log file to display. Defaults to -1, showing all log lines.")
	cmd.Flags().String("since-time", "", "Only return logs after a specific date (RFC3339). Defaults to all logs. Only one of since-time / since may be used.")
	cmd.Flags().DurationVar(&o.KubeLogOptions.SinceSeconds, "since", o.KubeLogOptions.SinceSeconds, "Only return logs newer than a relative duration like 5s, 2m, or 3h. Defaults to all logs. Only one of since-time / since may be used.")
	cmd.Flags().StringVarP(&o.KubeLogOptions.ContainerName, "container", "c", o.KubeLogOptions.ContainerName, "Container name")

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
		o.KubeLogOptions.ContainerName = args[1]

	default:
		return cmdutil.UsageError(cmd, "log RESOURCE")
	}

	var err error
	o.Namespace, _, err = f.DefaultNamespace()
	if err != nil {
		return err
	}
	o.OriginClient, o.KubeLogOptions.Client, err = f.Clients()
	if err != nil {
		return err
	}

	o.KubeLogOptions.PodName = o.ResourceString
	o.KubeLogOptions.PodNamespace = o.Namespace

	sinceTime := cmdutil.GetFlagString(cmd, "since-time")
	if len(sinceTime) > 0 {
		t, err := kapi.ParseRFC3339(sinceTime, unversioned.Now)
		if err != nil {
			return err
		}
		o.KubeLogOptions.SinceTime = &t
	}

	return nil
}

func (o *OpenShiftLogsOptions) Validate() error {
	if len(o.ResourceString) == 0 {
		return errors.New("RESOURCE must be specified")
	}

	return o.KubeLogOptions.Validate()
}

func (o *OpenShiftLogsOptions) RunLog() error {
	resourceType := "pod"
	resourceName := o.ResourceString
	tokens := strings.SplitN(o.ResourceString, "/", 2)
	if len(tokens) == 2 {
		resourceType = tokens[0]
		resourceName = tokens[1]
	}
	resourceType = strings.ToLower(resourceType)

	// if we're requesting a pod, delegate directly to kubectl logs
	if (resourceType == "pods") || (resourceType == "pod") || (resourceType == "po") {
		o.KubeLogOptions.PodName = resourceName
		return o.KubeLogOptions.RunLog()
	}

	if len(o.KubeLogOptions.ContainerName) > 0 {
		return errors.New("container cannot be specified with anything besides a pod")
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
		Follow: o.KubeLogOptions.Follow,
		NoWait: false,
	}

	readCloser, err := o.OriginClient.BuildLogs(build.Namespace).Get(build.Name, opts).Stream()
	if err != nil {
		return err
	}
	defer readCloser.Close()

	_, err = io.Copy(o.KubeLogOptions.Out, readCloser)
	return err
}

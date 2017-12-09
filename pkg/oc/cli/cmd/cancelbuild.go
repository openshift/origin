package cmd

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildinternal "github.com/openshift/origin/pkg/build/client/internalversion"
	buildinternalclient "github.com/openshift/origin/pkg/build/generated/internalclientset"
	buildtypedclient "github.com/openshift/origin/pkg/build/generated/internalclientset/typed/build/internalversion"
	buildlister "github.com/openshift/origin/pkg/build/generated/listers/build/internalversion"
	buildutil "github.com/openshift/origin/pkg/build/util"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

// CancelBuildRecommendedCommandName is the recommended command name.
const CancelBuildRecommendedCommandName = "cancel-build"

var (
	cancelBuildLong = templates.LongDesc(`
		Cancel running, pending, or new builds

		This command requests a graceful shutdown of the build. There may be a delay between requesting
		the build and the time the build is terminated.`)

	cancelBuildExample = templates.Examples(`
	  # Cancel the build with the given name
	  %[1]s %[2]s ruby-build-2

	  # Cancel the named build and print the build logs
	  %[1]s %[2]s ruby-build-2 --dump-logs

	  # Cancel the named build and create a new one with the same parameters
	  %[1]s %[2]s ruby-build-2 --restart

	  # Cancel multiple builds
	  %[1]s %[2]s ruby-build-1 ruby-build-2 ruby-build-3

	  # Cancel all builds created from 'ruby-build' build configuration that are in 'new' state
	  %[1]s %[2]s bc/ruby-build --state=new`)
)

// CancelBuildOptions contains all the options for running the CancelBuild cli command.
type CancelBuildOptions struct {
	In          io.Reader
	Out, ErrOut io.Writer

	DumpLogs   bool
	Restart    bool
	States     []string
	Namespace  string
	BuildNames []string

	HasError    bool
	ReportError func(error)
	Mapper      meta.RESTMapper
	Client      buildinternalclient.Interface
	BuildClient buildtypedclient.BuildResourceInterface
	BuildLister buildlister.BuildLister

	// timeout is used by unit tests to shorten the polling period
	timeout time.Duration
}

// NewCmdCancelBuild implements the OpenShift cli cancel-build command
func NewCmdCancelBuild(name, baseName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	o := &CancelBuildOptions{}

	cmd := &cobra.Command{
		Use:        fmt.Sprintf("%s (BUILD | BUILDCONFIG)", name),
		Short:      "Cancel running, pending, or new builds",
		Long:       cancelBuildLong,
		Example:    fmt.Sprintf(cancelBuildExample, baseName, name),
		SuggestFor: []string{"builds", "stop-build"},
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args, in, out, errout))
			kcmdutil.CheckErr(o.RunCancelBuild())
		},
	}

	cmd.Flags().StringSliceVar(&o.States, "state", o.States, "Only cancel builds in this state")
	cmd.Flags().BoolVar(&o.DumpLogs, "dump-logs", o.DumpLogs, "Specify if the build logs for the cancelled build should be shown.")
	cmd.Flags().BoolVar(&o.Restart, "restart", o.Restart, "Specify if a new build should be created after the current build is cancelled.")
	return cmd
}

// Complete completes all the required options.
func (o *CancelBuildOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, in io.Reader, out, errout io.Writer) error {
	o.In = in
	o.Out = out
	o.ErrOut = errout
	o.ReportError = func(err error) {
		o.HasError = true
		fmt.Fprintf(o.ErrOut, "error: %s\n", err.Error())
	}

	if o.timeout.Seconds() == 0 {
		o.timeout = 30 * time.Second
	}

	if len(args) == 0 {
		return kcmdutil.UsageErrorf(cmd, "Must pass a name of a build or a buildconfig to cancel")
	}

	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	if len(o.States) == 0 {
		// If --state is not specified, set the default to "new", "pending" and
		// "running".
		o.States = []string{"new", "pending", "running"}
	} else {
		for _, state := range o.States {
			if len(state) > 0 && !isStateCancellable(state) {
				return kcmdutil.UsageErrorf(cmd, "The '--state' flag has invalid value. Must be one of 'new', 'pending', or 'running'")
			}
		}
	}

	config, err := f.BareClientConfig()
	if err != nil {
		return err
	}
	client, err := buildinternalclient.NewForConfig(config)
	if err != nil {
		return err
	}

	o.Namespace = namespace
	o.Client = client
	o.BuildLister = buildclient.NewClientBuildLister(client.Build())
	o.BuildClient = client.Build().Builds(namespace)
	o.Mapper, _ = f.Object()

	for _, item := range args {
		resource, name, err := cmdutil.ResolveResource(buildapi.Resource("builds"), item, o.Mapper)
		if err != nil {
			return err
		}

		switch {
		case buildapi.IsResourceOrLegacy("buildconfigs", resource):
			list, err := buildutil.BuildConfigBuilds(o.BuildLister, o.Namespace, name, nil)
			if err != nil {
				return err
			}
			for _, b := range list {
				o.BuildNames = append(o.BuildNames, b.Name)
			}
		case buildapi.IsResourceOrLegacy("builds", resource):
			o.BuildNames = append(o.BuildNames, strings.TrimSpace(name))
		default:
			return fmt.Errorf("invalid resource provided: %v", resource)
		}
	}

	return nil
}

// RunCancelBuild implements all the necessary functionality for CancelBuild.
func (o *CancelBuildOptions) RunCancelBuild() error {
	var builds []*buildapi.Build

	for _, name := range o.BuildNames {
		build, err := o.BuildClient.Get(name, metav1.GetOptions{})
		if err != nil {
			o.ReportError(fmt.Errorf("build %s/%s not found", o.Namespace, name))
			continue
		}

		stateMatch := false
		for _, state := range o.States {
			if strings.ToLower(string(build.Status.Phase)) == state {
				stateMatch = true
				break
			}
		}
		if stateMatch && !buildutil.IsBuildComplete(build) {
			builds = append(builds, build)
		}
	}

	if o.DumpLogs {
		for _, b := range builds {
			// Do not attempt to get logs from build that was not scheduled.
			if b.Status.Phase == buildapi.BuildPhaseNew {
				continue
			}
			logClient := buildinternal.NewBuildLogClient(o.Client.Build().RESTClient(), o.Namespace)
			opts := buildapi.BuildLogOptions{NoWait: true}
			response, err := logClient.Logs(b.Name, opts).Do().Raw()
			if err != nil {
				o.ReportError(fmt.Errorf("unable to fetch logs for %s/%s: %v", b.Namespace, b.Name, err))
				continue
			}
			fmt.Fprintf(o.Out, "==== Build %s/%s logs ====\n", b.Namespace, b.Name)
			fmt.Fprint(o.Out, string(response))
		}
	}

	var wg sync.WaitGroup
	for _, b := range builds {
		wg.Add(1)
		go func(build *buildapi.Build) {
			defer wg.Done()
			err := wait.Poll(500*time.Millisecond, o.timeout, func() (bool, error) {
				build.Status.Cancelled = true
				_, err := o.BuildClient.Update(build)
				switch {
				case err == nil:
					return true, nil
				case kapierrors.IsConflict(err):
					build, err = o.BuildClient.Get(build.Name, metav1.GetOptions{})
					return false, err
				}
				return true, err
			})
			if err != nil {
				o.ReportError(fmt.Errorf("build %s/%s failed to update: %v", build.Namespace, build.Name, err))
				return
			}

			// Make sure the build phase is really cancelled.
			err = wait.Poll(500*time.Millisecond, o.timeout, func() (bool, error) {
				updatedBuild, err := o.BuildClient.Get(build.Name, metav1.GetOptions{})
				if err != nil {
					return true, err
				}
				return updatedBuild.Status.Phase == buildapi.BuildPhaseCancelled, nil
			})
			if err != nil {
				o.ReportError(fmt.Errorf("build %s/%s failed to cancel: %v", build.Namespace, build.Name, err))
				return
			}

			resource, name, _ := cmdutil.ResolveResource(buildapi.Resource("builds"), build.Name, o.Mapper)
			kcmdutil.PrintSuccess(o.Mapper, false, o.Out, resource.Resource, name, false, "cancelled")
		}(b)
	}
	wg.Wait()

	if o.Restart {
		for _, b := range builds {
			request := &buildapi.BuildRequest{ObjectMeta: metav1.ObjectMeta{Namespace: b.Namespace, Name: b.Name}}
			build, err := o.BuildClient.Clone(request.Name, request)
			if err != nil {
				o.ReportError(fmt.Errorf("build %s/%s failed to restart: %v", b.Namespace, b.Name, err))
				continue
			}
			resource, name, _ := cmdutil.ResolveResource(buildapi.Resource("builds"), build.Name, o.Mapper)
			kcmdutil.PrintSuccess(o.Mapper, false, o.Out, resource.Resource, name, false, fmt.Sprintf("restarted build %q", b.Name))
		}
	}

	if o.HasError {
		return errors.New("failure during the build cancellation")
	}

	return nil
}

// isStateCancellable validates the state provided by the '--state' flag.
func isStateCancellable(state string) bool {
	cancellablePhases := []string{
		string(buildapi.BuildPhaseNew),
		string(buildapi.BuildPhasePending),
		string(buildapi.BuildPhaseRunning),
	}
	for _, p := range cancellablePhases {
		if state == strings.ToLower(p) {
			return true
		}
	}
	return false
}

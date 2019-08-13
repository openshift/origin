package cancelbuild

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/scheme"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	"github.com/openshift/api/build"
	buildv1 "github.com/openshift/api/build/v1"
	buildtv1client "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	ocbuildutil "github.com/openshift/oc/pkg/helpers/build"
	buildclientv1 "github.com/openshift/oc/pkg/helpers/build/client/v1"
	cmdutil "github.com/openshift/oc/pkg/helpers/cmd"
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
	DumpLogs   bool
	Restart    bool
	States     []string
	Namespace  string
	BuildNames []string

	HasError                bool
	ReportError             func(error)
	PrinterCancel           printers.ResourcePrinter
	PrinterCancelInProgress printers.ResourcePrinter
	PrinterRestart          printers.ResourcePrinter
	Mapper                  meta.RESTMapper
	Client                  buildtv1client.BuildV1Interface
	BuildClient             buildtv1client.BuildInterface

	// timeout is used by unit tests to shorten the polling period
	timeout time.Duration

	genericclioptions.IOStreams
}

func NewCancelBuildOptions(streams genericclioptions.IOStreams) *CancelBuildOptions {
	return &CancelBuildOptions{
		IOStreams: streams,
		States:    []string{"new", "pending", "running"},
	}
}

// NewCmdCancelBuild implements the OpenShift cli cancel-build command
func NewCmdCancelBuild(name, baseName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewCancelBuildOptions(streams)
	cmd := &cobra.Command{
		Use:        fmt.Sprintf("%s (BUILD | BUILDCONFIG)", name),
		Short:      "Cancel running, pending, or new builds",
		Long:       cancelBuildLong,
		Example:    fmt.Sprintf(cancelBuildExample, baseName, name),
		SuggestFor: []string{"builds", "stop-build"},
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.RunCancelBuild())
		},
	}

	cmd.Flags().StringSliceVar(&o.States, "state", o.States, "Only cancel builds in this state")
	cmd.Flags().BoolVar(&o.DumpLogs, "dump-logs", o.DumpLogs, "Specify if the build logs for the cancelled build should be shown.")
	cmd.Flags().BoolVar(&o.Restart, "restart", o.Restart, "Specify if a new build should be created after the current build is cancelled.")

	return cmd
}

// Complete completes all the required options.
func (o *CancelBuildOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("build or a buildconfig name is required")
	}

	o.ReportError = func(err error) {
		o.HasError = true
		fmt.Fprintf(o.ErrOut, "error: %s\n", err.Error())
	}

	var err error
	o.PrinterCancel, err = printers.NewTypeSetter(scheme.Scheme).
		WrapToPrinter(&printers.NamePrinter{Operation: "cancelled"}, nil)
	if err != nil {
		return err
	}
	o.PrinterRestart, err = printers.NewTypeSetter(scheme.Scheme).
		WrapToPrinter(&printers.NamePrinter{Operation: "restarted"}, nil)
	if err != nil {
		return err
	}
	o.PrinterCancelInProgress, err = printers.NewTypeSetter(scheme.Scheme).
		WrapToPrinter(&printers.NamePrinter{Operation: "marked for cancellation, waiting to be cancelled"}, nil)
	if err != nil {
		return err
	}

	if o.timeout.Seconds() == 0 {
		o.timeout = 30 * time.Second
	}

	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	config, err := f.ToRESTConfig()
	if err != nil {
		return err
	}

	o.Client, err = buildtv1client.NewForConfig(config)
	if err != nil {
		return err
	}

	o.BuildClient = o.Client.Builds(o.Namespace)
	o.Mapper, err = f.ToRESTMapper()
	if err != nil {
		return err
	}

	for _, item := range args {
		resource, name, err := cmdutil.ResolveResource(build.Resource("builds"), item, o.Mapper)
		if err != nil {
			return err
		}

		switch resource {
		case build.Resource("buildconfigs"):
			list, err := buildConfigBuilds(o.Client, o.Namespace, name, nil)
			if err != nil {
				return err
			}
			for _, b := range list {
				o.BuildNames = append(o.BuildNames, b.Name)
			}
		case build.Resource("builds"):
			o.BuildNames = append(o.BuildNames, strings.TrimSpace(name))
		default:
			return fmt.Errorf("invalid resource provided: %v", resource)
		}
	}

	return nil
}

type buildFilter func(*buildv1.Build) bool

// buildConfigSelector returns a label Selector which can be used to find all
// builds for a BuildConfig.
func buildConfigSelector(name string) labels.Selector {
	return labels.Set{buildv1.BuildConfigLabel: labelValue(name)}.AsSelector()
}

// labelValue returns a string to use as a value for the Build
// label in a pod. If the length of the string parameter exceeds
// the maximum label length, the value will be truncated.
func labelValue(name string) string {
	if len(name) <= validation.DNS1123LabelMaxLength {
		return name
	}
	return name[:validation.DNS1123LabelMaxLength]
}

// BuildConfigBuilds return a list of builds for the given build config.
// Optionally you can specify a filter function to select only builds that
// matches your criteria.
func buildConfigBuilds(c buildtv1client.BuildsGetter, namespace, name string, filterFunc buildFilter) ([]*buildv1.Build, error) {
	result, err := c.Builds(namespace).List(metav1.ListOptions{LabelSelector: buildConfigSelector(name).String()})
	if err != nil {
		return nil, err
	}
	builds := make([]*buildv1.Build, len(result.Items))
	for i := range result.Items {
		builds[i] = &result.Items[i]
	}
	if filterFunc == nil {
		return builds, nil
	}
	var filteredList []*buildv1.Build
	for _, b := range builds {
		if filterFunc(b) {
			filteredList = append(filteredList, b)
		}
	}
	return filteredList, nil
}

func (o *CancelBuildOptions) Validate() error {
	for _, state := range o.States {
		if len(state) > 0 && !isStateCancellable(state) {
			return fmt.Errorf("invalid --state flag value, must be one of 'new', 'pending', or 'running'")
		}
	}

	return nil
}

// RunCancelBuild implements all the necessary functionality for CancelBuild.
func (o *CancelBuildOptions) RunCancelBuild() error {
	var builds []*buildv1.Build
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

		if stateMatch && !ocbuildutil.IsTerminalPhase(build.Status.Phase) {
			builds = append(builds, build)
		}
	}

	if o.DumpLogs {
		for _, b := range builds {
			// Do not attempt to get logs from build that was not scheduled.
			if b.Status.Phase == buildv1.BuildPhaseNew {
				continue
			}
			logClient := buildclientv1.NewBuildLogClient(o.Client.RESTClient(), o.Namespace, scheme.Scheme)
			opts := buildv1.BuildLogOptions{NoWait: true}
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
		go func(build *buildv1.Build) {
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

			// ignore exit if error here; the phase verfication below is more important
			o.PrinterCancelInProgress.PrintObj(build, o.Out)

			// Make sure the build phase is really cancelled.
			timeout := o.timeout
			if build.Spec.Strategy.JenkinsPipelineStrategy != nil {
				//bump the timeout in case we have to wait for Jenkins
				//to come up so that the sync plugin can actually change
				//the phase
				timeout = timeout + (3 * time.Minute)
			}
			err = wait.Poll(500*time.Millisecond, timeout, func() (bool, error) {
				updatedBuild, err := o.BuildClient.Get(build.Name, metav1.GetOptions{})
				if err != nil {
					return true, err
				}
				return updatedBuild.Status.Phase == buildv1.BuildPhaseCancelled, nil
			})
			if err != nil {
				o.ReportError(fmt.Errorf("build %s/%s failed to cancel: %v", build.Namespace, build.Name, err))
				return
			}

			if err := o.PrinterCancel.PrintObj(build, o.Out); err != nil {
				o.ReportError(fmt.Errorf("build %s/%s failed to print: %v", build.Namespace, build.Name, err))
				return
			}
		}(b)
	}
	wg.Wait()

	if o.Restart {
		for _, b := range builds {
			request := &buildv1.BuildRequest{ObjectMeta: metav1.ObjectMeta{Namespace: b.Namespace, Name: b.Name}}
			build, err := o.BuildClient.Clone(request.Name, request)
			if err != nil {
				o.ReportError(fmt.Errorf("build %s/%s failed to restart: %v", b.Namespace, b.Name, err))
				continue
			}
			if err := o.PrinterRestart.PrintObj(b, o.Out); err != nil {
				o.ReportError(fmt.Errorf("build %s/%s failed to print: %v", build.Namespace, build.Name, err))
				continue
			}
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
		string(buildv1.BuildPhaseNew),
		string(buildv1.BuildPhasePending),
		string(buildv1.BuildPhaseRunning),
	}
	for _, p := range cancellablePhases {
		if state == strings.ToLower(p) {
			return true
		}
	}
	return false
}

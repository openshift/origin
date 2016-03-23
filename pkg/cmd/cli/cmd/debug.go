package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/fields"
	kcmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/util/interrupt"
	"k8s.io/kubernetes/pkg/util/term"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"k8s.io/kubernetes/pkg/runtime"
)

type DebugOptions struct {
	Attach kcmd.AttachOptions

	Print         func(pod *kapi.Pod, w io.Writer) error
	LogsForObject func(object, options runtime.Object) (*restclient.Request, error)

	NoStdin    bool
	ForceTTY   bool
	DisableTTY bool
	Filename   string
	Timeout    time.Duration

	Command         []string
	Annotations     map[string]string
	AsRoot          bool
	KeepLabels      bool // TODO: evaluate selecting the right labels automatically
	KeepAnnotations bool
	KeepLiveness    bool
	KeepReadiness   bool
	OneContainer    bool
	NodeName        string
	AddEnv          []kapi.EnvVar
	RemoveEnv       []string
}

const (
	debugLong = `
Launch a command shell to debug a running application

When debugging images and setup problems, it's useful to get an exact copy of a running
pod configuration and troubleshoot with a shell. Since a pod that is failing may not be
started and not accessible to 'rsh' or 'exec', the 'debug' command makes it easy to
create a carbon copy of that setup.

The default mode is to start a shell inside of the first container of the referenced pod,
replication controller, or deployment config. The started pod will be a copy of your
source pod, with labels stripped, the command changed to '/bin/sh', and readiness and
liveness checks disabled. If you just want to run a command, add '--' and a command to
run. Passing a command will not create a TTY or send STDIN by default. Other flags are
supported for altering the container or pod in common ways.

The debug pod is deleted when the the remote command completes or the user interrupts
the shell.`

	debugExample = `
  # Debug a currently running deployment
  $ %[1]s dc/test

  # Debug a specific failing container by running the env command in the 'second' container
  $ %[1]s dc/test -c second -- /bin/env

  # See the pod that would be created to debug
  $ %[1]s dc/test -o yaml`

	debugPodLabelName = "debug.openshift.io/name"

	debugPodAnnotationSourceContainer = "debug.openshift.io/source-container"
	debugPodAnnotationSourceResource  = "debug.openshift.io/source-resource"
)

// NewCmdDebug creates a command for debugging pods.
func NewCmdDebug(fullName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	options := &DebugOptions{
		Timeout: 30 * time.Second,
		Attach: kcmd.AttachOptions{
			In:    in,
			Out:   out,
			Err:   errout,
			TTY:   true,
			Stdin: true,

			Attach: &kcmd.DefaultRemoteAttach{},
		},
		LogsForObject: f.LogsForObject,
	}

	cmd := &cobra.Command{
		Use:     "debug RESOURCE/NAME [ENV1=VAL1 ...] [-c CONTAINER] [-- COMMAND]",
		Short:   "Launch a new instance of a pod for debugging",
		Long:    debugLong,
		Example: fmt.Sprintf(debugExample, fmt.Sprintf("%s debug", fullName)),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete(cmd, f, args, in, out, errout))
			kcmdutil.CheckErr(options.Validate())
			kcmdutil.CheckErr(options.Debug())
		},
	}

	// TODO: when T is deprecated use the printer, but keep these hidden
	cmd.Flags().StringP("output", "o", "", "Output format. One of: json|yaml|wide|name|go-template=...|go-template-file=...|jsonpath=...|jsonpath-file=... See golang template [http://golang.org/pkg/text/template/#pkg-overview] and jsonpath template [http://releases.k8s.io/HEAD/docs/user-guide/jsonpath.md].")
	cmd.Flags().String("output-version", "", "Output the formatted object with the given version (default api-version).")
	cmd.Flags().String("template", "", "Template string or path to template file to use when -o=go-template, -o=go-template-file. The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview].")
	cmd.MarkFlagFilename("template")
	cmd.Flags().Bool("no-headers", false, "When using the default output, don't print headers.")
	cmd.Flags().MarkHidden("no-headers")
	cmd.Flags().String("sort-by", "", "If non-empty, sort list types using this field specification.  The field specification is expressed as a JSONPath expression (e.g. 'ObjectMeta.Name'). The field in the API resource specified by this JSONPath expression must be an integer or a string.")
	cmd.Flags().MarkHidden("sort-by")
	cmd.Flags().Bool("show-all", true, "When printing, show all resources (default hide terminated pods.)")
	cmd.Flags().MarkHidden("show-all")

	cmd.Flags().BoolVarP(&options.NoStdin, "no-stdin", "I", options.NoStdin, "Bypasses passing STDIN to the container, defaults to true if no command specified")
	cmd.Flags().BoolVarP(&options.ForceTTY, "tty", "t", false, "Force a pseudo-terminal to be allocated")
	cmd.Flags().BoolVarP(&options.DisableTTY, "no-tty", "T", false, "Disable pseudo-terminal allocation")

	cmd.Flags().StringVarP(&options.Attach.ContainerName, "container", "c", "", "Container name; defaults to first container")
	cmd.Flags().BoolVar(&options.KeepAnnotations, "keep-annotations", false, "Keep the original pod annotations")
	cmd.Flags().BoolVar(&options.KeepLiveness, "keep-liveness", false, "Keep the original pod liveness probes")
	cmd.Flags().BoolVar(&options.KeepReadiness, "keep-readiness", false, "Keep the original pod readiness probes")
	cmd.Flags().BoolVar(&options.OneContainer, "one-container", false, "Run only the selected container, remove all others")
	cmd.Flags().StringVar(&options.NodeName, "node-name", "", "Set a specific node to run on - by default the pod will run on any valid node")
	cmd.Flags().BoolVar(&options.AsRoot, "as-root", false, "Try to run the container as the root user")

	cmd.Flags().StringVarP(&options.Filename, "filename", "f", "", "Filename or URL to file to read a template")
	cmd.MarkFlagFilename("filename", "yaml", "yml", "json")

	return cmd
}

func (o *DebugOptions) Complete(cmd *cobra.Command, f *clientcmd.Factory, args []string, in io.Reader, out, errout io.Writer) error {
	if i := cmd.ArgsLenAtDash(); i != -1 && i < len(args) {
		o.Command = args[i:]
		args = args[:i]
	}
	resources, envArgs, ok := cmdutil.SplitEnvironmentFromResources(args)
	if !ok {
		return kcmdutil.UsageError(cmd, "all resources must be specified before environment changes: %s", strings.Join(args, " "))
	}

	switch {
	case o.ForceTTY && o.NoStdin:
		return kcmdutil.UsageError(cmd, "you may not specify -I and -t together")
	case o.ForceTTY && o.DisableTTY:
		return kcmdutil.UsageError(cmd, "you may not specify -t and -T together")
	case o.ForceTTY:
		o.Attach.TTY = true
	case o.DisableTTY:
		o.Attach.TTY = false
	// don't default TTY to true if a command is passed
	case len(o.Command) > 0:
		o.Attach.TTY = false
		o.Attach.Stdin = false
	default:
		o.Attach.TTY = term.IsTerminal(in)
		glog.V(4).Infof("Defaulting TTY to %t", o.Attach.TTY)
	}
	if o.NoStdin {
		o.Attach.TTY = false
		o.Attach.Stdin = false
	}

	if o.Annotations == nil {
		o.Annotations = make(map[string]string)
	}

	if len(o.Command) == 0 {
		o.Command = []string{"/bin/sh"}
	}

	cmdNamespace, explicit, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	mapper, typer := f.Object()
	b := resource.NewBuilder(mapper, typer, resource.ClientMapperFunc(f.ClientForMapping), kapi.Codecs.UniversalDecoder()).
		NamespaceParam(cmdNamespace).DefaultNamespace().
		SingleResourceType().
		ResourceTypeOrNameArgs(true, resources...).
		Flatten()
	if len(o.Filename) > 0 {
		b.FilenameParam(explicit, o.Filename)
	}

	o.AddEnv, o.RemoveEnv, err = cmdutil.ParseEnv(envArgs, nil)
	if err != nil {
		return err
	}

	one := false
	infos, err := b.Do().IntoSingular(&one).Infos()
	if err != nil {
		return err
	}
	if !one {
		return fmt.Errorf("you must identify a resource with a pod template to debug")
	}

	template, err := f.ApproximatePodTemplateForObject(infos[0].Object)
	if err != nil || template == nil {
		return fmt.Errorf("cannot debug %s: %v", infos[0].Name, err)
	}
	pod := &kapi.Pod{
		ObjectMeta: template.ObjectMeta,
		Spec:       template.Spec,
	}
	pod.Name, pod.Namespace = infos[0].Name, infos[0].Namespace
	o.Attach.Pod = pod

	if len(o.Attach.ContainerName) == 0 && len(pod.Spec.Containers) > 0 {
		glog.V(4).Infof("defaulting container name to %s", pod.Spec.Containers[0].Name)
		o.Attach.ContainerName = pod.Spec.Containers[0].Name
	}

	o.Annotations[debugPodAnnotationSourceResource] = fmt.Sprintf("%s/%s", infos[0].Mapping.Resource, infos[0].Name)
	o.Annotations[debugPodAnnotationSourceContainer] = o.Attach.ContainerName

	output := kcmdutil.GetFlagString(cmd, "output")
	if len(output) != 0 {
		o.Print = func(pod *kapi.Pod, out io.Writer) error {
			return f.PrintObject(cmd, pod, out)
		}
	}

	config, err := f.ClientConfig()
	if err != nil {
		return err
	}
	o.Attach.Config = config

	_, kc, err := f.Clients()
	if err != nil {
		return err
	}
	o.Attach.Client = kc
	return nil
}
func (o DebugOptions) Validate() error {
	names := containerNames(o.Attach.Pod)
	if len(names) == 0 {
		return fmt.Errorf("the provided pod must have at least one container")
	}
	if len(o.Attach.ContainerName) == 0 {
		return fmt.Errorf("you must provide a container name to debug")
	}
	if containerForName(o.Attach.Pod, o.Attach.ContainerName) == nil {
		return fmt.Errorf("the container %q is not a valid container name; must be one of %v", o.Attach.ContainerName, names)
	}
	return nil
}

// WatchConditionFunc returns true if the condition has been reached, false if it has not been reached yet,
// or an error if the condition cannot be checked and should terminate.
type WatchConditionFunc func(event watch.Event) (bool, error)

// Until reads items from the watch until each provided condition succeeds, and then returns the last watch
// encountered. The first condition that returns an error terminates the watch (and the event is also returned).
// If no event has been received, the returned event will be nil.
// TODO: move to pkg/watch upstream
func Until(timeout time.Duration, watcher watch.Interface, conditions ...WatchConditionFunc) (*watch.Event, error) {
	ch := watcher.ResultChan()
	defer watcher.Stop()
	var after <-chan time.Time
	if timeout > 0 {
		after = time.After(timeout)
	} else {
		ch := make(chan time.Time)
		close(ch)
		after = ch
	}
	var lastEvent *watch.Event
	for _, condition := range conditions {
		for {
			select {
			case event, ok := <-ch:
				if !ok {
					return lastEvent, wait.ErrWaitTimeout
				}
				lastEvent = &event
				// TODO: check for watch expired error and retry watch from latest point?
				done, err := condition(event)
				if err != nil {
					return lastEvent, err
				}
				if done {
					return lastEvent, nil
				}
			case <-after:
				return lastEvent, wait.ErrWaitTimeout
			}
		}
	}
	return lastEvent, wait.ErrWaitTimeout
}

// ErrPodCompleted is returned by PodRunning or PodContainerRunning to indicate that
// the pod has already reached completed state.
var ErrPodCompleted = fmt.Errorf("pod ran to completion")

// TODO: move to pkg/client/conditions.go upstream
//
// Example of a running condition, will be used elsewhere
//
// PodRunning returns true if the pod is running, false if the pod has not yet reached running state,
// returns ErrPodCompleted if the pod has run to completion, or an error in any other case.
// func PodRunning(event watch.Event) (bool, error) {
// 	switch event.Type {
// 	case watch.Deleted:
// 		return false, kapierrors.NewNotFound(unversioned.GroupResource{Resource: "pods"}, "")
// 	}
// 	switch t := event.Object.(type) {
// 	case *kapi.Pod:
// 		switch t.Status.Phase {
// 		case kapi.PodRunning:
// 			return true, nil
// 		case kapi.PodFailed, kapi.PodSucceeded:
// 			return false, ErrPodCompleted
// 		}
// 	}
// 	return false, nil
// }

// PodContainerRunning returns false until the named container has ContainerStatus running (at least once),
// and will return an error if the pod is deleted, runs to completion, or the container pod is not available.
func PodContainerRunning(containerName string) WatchConditionFunc {
	return func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Deleted:
			return false, kapierrors.NewNotFound(unversioned.GroupResource{Resource: "pods"}, "")
		}
		switch t := event.Object.(type) {
		case *kapi.Pod:
			switch t.Status.Phase {
			case kapi.PodRunning, kapi.PodPending:
			case kapi.PodFailed, kapi.PodSucceeded:
				return false, ErrPodCompleted
			default:
				return false, nil
			}
			for _, s := range t.Status.ContainerStatuses {
				if s.Name != containerName {
					continue
				}
				return s.State.Running != nil, nil
			}
			return false, nil
		}
		return false, nil
	}
}

// SingleObject returns a ListOptions for watching a single object.
// TODO: move to pkg/api/helpers.go upstream.
func SingleObject(meta kapi.ObjectMeta) kapi.ListOptions {
	return kapi.ListOptions{
		FieldSelector:   fields.OneTermEqualSelector("metadata.name", meta.Name),
		ResourceVersion: meta.ResourceVersion,
	}
}

// Debug creates and runs a debugging pod.
func (o *DebugOptions) Debug() error {
	pod, originalCommand := o.transformPodForDebug(o.Annotations)
	var commandString string
	switch {
	case len(originalCommand) > 0:
		commandString = strings.Join(originalCommand, " ")
	default:
		commandString = "<image entrypoint>"
	}

	if o.Print != nil {
		return o.Print(pod, o.Attach.Out)
	}

	glog.V(5).Infof("Creating pod: %#v", pod)
	fmt.Fprintf(o.Attach.Err, "Debugging with pod/%s, original command: %s\n", pod.Name, commandString)
	pod, err := o.createPod(pod)
	if err != nil {
		return err
	}

	// ensure the pod is cleaned up on shutdown
	o.Attach.InterruptParent = interrupt.New(
		func(os.Signal) { os.Exit(1) },
		func() {
			fmt.Fprintf(o.Attach.Err, "\nRemoving debug pod ...\n")
			if err := o.Attach.Client.Pods(pod.Namespace).Delete(pod.Name, kapi.NewDeleteOptions(0)); err != nil {
				fmt.Fprintf(o.Attach.Err, "error: unable to delete the debug pod %q: %v", pod.Name, err)
			}
		},
	)
	glog.V(5).Infof("Created attach arguments: %#v", o.Attach)
	return o.Attach.InterruptParent.Run(func() error {
		w, err := o.Attach.Client.Pods(pod.Namespace).Watch(SingleObject(pod.ObjectMeta))
		if err != nil {
			return err
		}
		fmt.Fprintf(o.Attach.Err, "Waiting for pod to start ...\n")
		switch _, err := Until(o.Timeout, w, PodContainerRunning(o.Attach.ContainerName)); {
		// switch to logging output
		case err == ErrPodCompleted, !o.Attach.Stdin:
			_, err := kcmd.LogsOptions{
				Object: pod,
				Options: &kapi.PodLogOptions{
					Container: o.Attach.ContainerName,
					Follow:    true,
				},
				Out: o.Attach.Out,

				LogsForObject: o.LogsForObject,
			}.RunLogs()
			return err
		case err != nil:
			return err
		default:
			// TODO: attach can race with pod completion, allow attach to switch to logs
			return o.Attach.Run()
		}
	})
}

// transformPodForDebug alters the input pod to be debuggable
func (o *DebugOptions) transformPodForDebug(annotations map[string]string) (*kapi.Pod, []string) {
	pod := o.Attach.Pod

	// reset the container
	container := containerForName(pod, o.Attach.ContainerName)

	// identify the command to be run
	originalCommand := append(container.Command, container.Args...)
	container.Command = o.Command
	if len(originalCommand) == 0 {
		if cmd, ok := imageapi.ContainerImageEntrypointByAnnotation(pod.Annotations, o.Attach.ContainerName); ok {
			originalCommand = cmd
		}
	}
	container.Args = nil

	container.TTY = o.Attach.Stdin && o.Attach.TTY
	container.Stdin = o.Attach.Stdin
	container.StdinOnce = o.Attach.Stdin

	if !o.KeepReadiness {
		container.ReadinessProbe = nil
	}
	if !o.KeepLiveness {
		container.LivenessProbe = nil
	}

	var newEnv []kapi.EnvVar
	if len(o.RemoveEnv) > 0 {
		for i := range container.Env {
			skip := false
			for _, name := range o.RemoveEnv {
				if name == container.Env[i].Name {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
			newEnv = append(newEnv, container.Env[i])
		}
	} else {
		newEnv = container.Env
	}
	for _, env := range o.AddEnv {
		newEnv = append(newEnv, env)
	}
	container.Env = newEnv

	if o.AsRoot {
		if container.SecurityContext == nil {
			container.SecurityContext = &kapi.SecurityContext{}
		}
		container.SecurityContext.RunAsNonRoot = nil
		zero := int64(0)
		container.SecurityContext.RunAsUser = &zero
	}

	if o.OneContainer {
		pod.Spec.Containers = []kapi.Container{*container}
	}

	// reset the pod
	if pod.Annotations == nil || !o.KeepAnnotations {
		pod.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		pod.Annotations[k] = v
	}
	if o.KeepLabels {
		if pod.Labels == nil {
			pod.Labels = make(map[string]string)
		}
		pod.Labels[debugPodLabelName] = pod.Name
	} else {
		pod.Labels = map[string]string{debugPodLabelName: pod.Name}
	}
	// always clear the NodeName
	pod.Spec.NodeName = o.NodeName

	pod.ResourceVersion = ""
	pod.Spec.RestartPolicy = kapi.RestartPolicyNever
	// TODO: shorten segments, make incrementing?
	pod.Name = fmt.Sprintf("debug-%s", pod.Name)
	pod.Status = kapi.PodStatus{}
	pod.UID = ""
	pod.CreationTimestamp = unversioned.Time{}
	pod.SelfLink = ""

	return pod, originalCommand
}

// createPod creates the debug pod, and will attempt to delete an existing debug
// pod with the same name, but will return an error in any other case.
func (o *DebugOptions) createPod(pod *kapi.Pod) (*kapi.Pod, error) {
	namespace, name := pod.Namespace, pod.Name

	// create the pod
	created, err := o.Attach.Client.Pods(namespace).Create(pod)
	if err == nil || !kapierrors.IsAlreadyExists(err) {
		return created, err
	}

	// only continue if the pod has the right annotations
	existing, err := o.Attach.Client.Pods(namespace).Get(name)
	if err != nil {
		return nil, err
	}
	if existing.Annotations[debugPodAnnotationSourceResource] != o.Annotations[debugPodAnnotationSourceResource] {
		return nil, fmt.Errorf("a pod already exists named %q, please delete it before running debug", name)
	}

	// delete the existing pod
	if err := o.Attach.Client.Pods(namespace).Delete(name, kapi.NewDeleteOptions(0)); err != nil && !kapierrors.IsNotFound(err) {
		return nil, fmt.Errorf("unable to delete existing debug pod %q: %v", name, err)
	}
	return o.Attach.Client.Pods(namespace).Create(pod)
}

func containerForName(pod *kapi.Pod, name string) *kapi.Container {
	for i, c := range pod.Spec.Containers {
		if c.Name != name {
			continue
		}
		return &pod.Spec.Containers[i]
	}
	return nil
}

func containerNames(pod *kapi.Pod) []string {
	var names []string
	for _, c := range pod.Spec.Containers {
		names = append(names, c.Name)
	}
	return names
}

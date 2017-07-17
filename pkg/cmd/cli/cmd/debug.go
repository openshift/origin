package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	restclient "k8s.io/client-go/rest"
	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kcmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/util/interrupt"
	"k8s.io/kubernetes/pkg/util/term"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
	generateapp "github.com/openshift/origin/pkg/generate/app"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

type DebugOptions struct {
	Attach kcmd.AttachOptions
	Client *client.Client

	Print         func(pod *kapi.Pod, w io.Writer) error
	LogsForObject func(object, options runtime.Object, timeout time.Duration) (*restclient.Request, error)

	NoStdin    bool
	ForceTTY   bool
	DisableTTY bool
	Filename   string
	Timeout    time.Duration

	Command            []string
	Annotations        map[string]string
	AsRoot             bool
	AsNonRoot          bool
	AsUser             int64
	KeepLabels         bool // TODO: evaluate selecting the right labels automatically
	KeepAnnotations    bool
	KeepLiveness       bool
	KeepReadiness      bool
	KeepInitContainers bool
	OneContainer       bool
	NodeName           string
	AddEnv             []kapi.EnvVar
	RemoveEnv          []string
}

const (
	debugPodLabelName = "debug.openshift.io/name"

	debugPodAnnotationSourceContainer = "debug.openshift.io/source-container"
	debugPodAnnotationSourceResource  = "debug.openshift.io/source-resource"
)

var (
	debugLong = templates.LongDesc(`
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

		A common problem running containers is a security policy that prohibits you from running
		as a root user on the cluster. You can use this command to test running a pod as
		non-root (with --as-user) or to run a non-root pod as root (with --as-root).

		The debug pod is deleted when the the remote command completes or the user interrupts
		the shell.`)

	debugExample = templates.Examples(`
	  # Debug a currently running deployment
	  %[1]s dc/test

	  # Test running a deployment as a non-root user
	  %[1]s dc/test --as-user=1000000

	  # Debug a specific failing container by running the env command in the 'second' container
	  %[1]s dc/test -c second -- /bin/env

	  # See the pod that would be created to debug
	  %[1]s dc/test -o yaml`)
)

// NewCmdDebug creates a command for debugging pods.
func NewCmdDebug(fullName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	options := &DebugOptions{
		Timeout: 15 * time.Minute,
		Attach: kcmd.AttachOptions{
			StreamOptions: kcmd.StreamOptions{
				In:    in,
				Out:   out,
				Err:   errout,
				TTY:   true,
				Stdin: true,
			},

			Attach: &kcmd.DefaultRemoteAttach{},
		},
		LogsForObject: f.LogsForObject,
	}

	cmd := &cobra.Command{
		Use:     "debug RESOURCE/NAME [ENV1=VAL1 ...] [-c CONTAINER] [options] [-- COMMAND]",
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
	cmd.Flags().StringP("output", "o", "", "Output format. One of: json|yaml|wide|name|go-template=...|go-template-file=...|jsonpath=...|jsonpath-file=... See golang template [http://golang.org/pkg/text/template/#pkg-overview] and jsonpath template [http://kubernetes.io/docs/user-guide/jsonpath/].")
	cmd.Flags().String("output-version", "", "Output the formatted object with the given version (default api-version).")
	cmd.Flags().String("template", "", "Template string or path to template file to use when -o=go-template, -o=go-template-file. The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview].")
	cmd.MarkFlagFilename("template")
	cmd.Flags().Bool("no-headers", false, "If true, when using the default output, don't print headers.")
	cmd.Flags().MarkHidden("no-headers")
	cmd.Flags().String("sort-by", "", "If non-empty, sort list types using this field specification.  The field specification is expressed as a JSONPath expression (e.g. 'ObjectMeta.Name'). The field in the API resource specified by this JSONPath expression must be an integer or a string.")
	cmd.Flags().MarkHidden("sort-by")
	cmd.Flags().Bool("show-all", true, "When printing, show all resources (default hide terminated pods.)")
	cmd.Flags().MarkHidden("show-all")
	cmd.Flags().Bool("show-labels", false, "When printing, show all labels as the last column (default hide labels column)")

	cmd.Flags().BoolVarP(&options.NoStdin, "no-stdin", "I", options.NoStdin, "Bypasses passing STDIN to the container, defaults to true if no command specified")
	cmd.Flags().BoolVarP(&options.ForceTTY, "tty", "t", false, "Force a pseudo-terminal to be allocated")
	cmd.Flags().BoolVarP(&options.DisableTTY, "no-tty", "T", false, "Disable pseudo-terminal allocation")

	cmd.Flags().StringVarP(&options.Attach.ContainerName, "container", "c", "", "Container name; defaults to first container")
	cmd.Flags().BoolVar(&options.KeepAnnotations, "keep-annotations", false, "If true, keep the original pod annotations")
	cmd.Flags().BoolVar(&options.KeepLiveness, "keep-liveness", false, "If true, keep the original pod liveness probes")
	cmd.Flags().BoolVar(&options.KeepInitContainers, "keep-init-containers", true, "Run the init containers for the pod. Defaults to true.")
	cmd.Flags().BoolVar(&options.KeepReadiness, "keep-readiness", false, "If true, keep the original pod readiness probes")
	cmd.Flags().BoolVar(&options.OneContainer, "one-container", false, "If true, run only the selected container, remove all others")
	cmd.Flags().StringVar(&options.NodeName, "node-name", "", "Set a specific node to run on - by default the pod will run on any valid node")
	cmd.Flags().BoolVar(&options.AsRoot, "as-root", false, "If true, try to run the container as the root user")
	cmd.Flags().Int64Var(&options.AsUser, "as-user", -1, "Try to run the container as a specific user UID (note: admins may limit your ability to use this flag)")

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
	// since ForceTTY is defaulted to false, check if user specifically passed in "=false" flag
	case !o.ForceTTY && cmd.Flags().Changed("tty"):
		o.Attach.TTY = false
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
	b := resource.NewBuilder(mapper, f.CategoryExpander(), typer, resource.ClientMapperFunc(f.ClientForMapping), kapi.Codecs.UniversalDecoder()).
		NamespaceParam(cmdNamespace).DefaultNamespace().
		SingleResourceType().
		ResourceNames("pods", resources...).
		Flatten()
	if len(o.Filename) > 0 {
		b.FilenameParam(explicit, &resource.FilenameOptions{Recursive: false, Filenames: []string{o.Filename}})
	}

	o.AddEnv, o.RemoveEnv, err = cmdutil.ParseEnv(envArgs, nil)
	if err != nil {
		return err
	}

	one := false
	infos, err := b.Do().IntoSingleItemImplied(&one).Infos()
	if err != nil {
		return err
	}
	if !one {
		return fmt.Errorf("you must identify a resource with a pod template to debug")
	}

	template, err := f.ApproximatePodTemplateForObject(infos[0].Object)
	if err != nil && template == nil {
		return fmt.Errorf("cannot debug %s: %v", infos[0].Name, err)
	}
	if err != nil {
		glog.V(4).Infof("Unable to get exact template, but continuing with fallback: %v", err)
	}
	pod := &kapi.Pod{
		ObjectMeta: template.ObjectMeta,
		Spec:       template.Spec,
	}
	pod.Name, pod.Namespace = fmt.Sprintf("%s-debug", generateapp.MakeSimpleName(infos[0].Name)), infos[0].Namespace
	o.Attach.Pod = pod

	o.AsNonRoot = !o.AsRoot && cmd.Flag("as-root").Changed

	if len(o.Attach.ContainerName) == 0 && len(pod.Spec.Containers) > 0 {
		glog.V(4).Infof("Defaulting container name to %s", pod.Spec.Containers[0].Name)
		o.Attach.ContainerName = pod.Spec.Containers[0].Name
	}

	o.Annotations[debugPodAnnotationSourceResource] = fmt.Sprintf("%s/%s", infos[0].Mapping.Resource, infos[0].Name)
	o.Annotations[debugPodAnnotationSourceContainer] = o.Attach.ContainerName

	output := kcmdutil.GetFlagString(cmd, "output")
	if len(output) != 0 {
		o.Print = func(pod *kapi.Pod, out io.Writer) error {
			return f.PrintObject(cmd, false, mapper, pod, out)
		}
	}

	config, err := f.ClientConfig()
	if err != nil {
		return err
	}
	o.Attach.Config = config

	oc, kc, err := f.Clients()
	if err != nil {
		return err
	}
	o.Attach.PodClient = kc.Core()
	o.Client = oc
	return nil
}
func (o DebugOptions) Validate() error {
	names := containerNames(o.Attach.Pod)
	if len(names) == 0 {
		return fmt.Errorf("the provided pod must have at least one container")
	}
	if (o.AsRoot || o.AsNonRoot) && o.AsUser > 0 {
		return fmt.Errorf("you may not specify --as-root and --as-user=%d at the same time", o.AsUser)
	}
	if len(o.Attach.ContainerName) == 0 {
		return fmt.Errorf("you must provide a container name to debug")
	}
	if containerForName(o.Attach.Pod, o.Attach.ContainerName) == nil {
		return fmt.Errorf("the container %q is not a valid container name; must be one of %v", o.Attach.ContainerName, names)
	}
	return nil
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
			stderr := o.Attach.Err
			if stderr == nil {
				stderr = os.Stderr
			}
			fmt.Fprintf(stderr, "\nRemoving debug pod ...\n")
			if err := o.Attach.PodClient.Pods(pod.Namespace).Delete(pod.Name, metav1.NewDeleteOptions(0)); err != nil {
				if !kapierrors.IsNotFound(err) {
					fmt.Fprintf(stderr, "error: unable to delete the debug pod %q: %v\n", pod.Name, err)
				}
			}
		},
	)

	glog.V(5).Infof("Created attach arguments: %#v", o.Attach)
	return o.Attach.InterruptParent.Run(func() error {
		w, err := o.Attach.PodClient.Pods(pod.Namespace).Watch(metav1.SingleObject(pod.ObjectMeta))
		if err != nil {
			return err
		}
		fmt.Fprintf(o.Attach.Err, "Waiting for pod to start ...\n")

		switch containerRunningEvent, err := watch.Until(o.Timeout, w, kclient.PodContainerRunning(o.Attach.ContainerName)); {
		// api didn't error right away but the pod wasn't even created
		case kapierrors.IsNotFound(err):
			msg := fmt.Sprintf("unable to create the debug pod %q", pod.Name)
			if len(o.NodeName) > 0 {
				msg += fmt.Sprintf(" on node %q", o.NodeName)
			}
			return fmt.Errorf(msg)
			// switch to logging output
		case err == kclient.ErrPodCompleted, err == kclient.ErrContainerTerminated, !o.Attach.Stdin:
			return kcmd.LogsOptions{
				Object: pod,
				Options: &kapi.PodLogOptions{
					Container: o.Attach.ContainerName,
					Follow:    true,
				},
				Out: o.Attach.Out,

				LogsForObject: o.LogsForObject,
			}.RunLogs()
		case err != nil:
			return err
		default:
			// TODO this doesn't do us much good for remote debugging sessions, but until we get a local port
			// set up to proxy, this is what we've got.
			if podWithStatus, ok := containerRunningEvent.Object.(*kapi.Pod); ok {
				fmt.Fprintf(o.Attach.Err, "Pod IP: %s\n", podWithStatus.Status.PodIP)
			}

			// TODO: attach can race with pod completion, allow attach to switch to logs
			return o.Attach.Run()
		}
	})
}

// getContainerImageViaDeploymentConfig attempts to return an Image for a given
// Container.  It tries to walk from the Container's Pod to its DeploymentConfig
// (via the "openshift.io/deployment-config.name" annotation), then tries to
// find the ImageStream from which the DeploymentConfig is deploying, then tries
// to find a match for the Container's image in the ImageStream's Images.
func (o *DebugOptions) getContainerImageViaDeploymentConfig(pod *kapi.Pod, container *kapi.Container) (*imageapi.Image, error) {
	ref, err := imageapi.ParseDockerImageReference(container.Image)
	if err != nil {
		return nil, err
	}

	if ref.ID == "" {
		return nil, nil // ID is needed for later lookup
	}

	dcname := pod.Annotations[deployapi.DeploymentConfigAnnotation]
	if dcname == "" {
		return nil, nil // Pod doesn't appear to have been created by a DeploymentConfig
	}

	dc, err := o.Client.DeploymentConfigs(o.Attach.Pod.Namespace).Get(dcname, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	for _, trigger := range dc.Spec.Triggers {
		if trigger.Type == deployapi.DeploymentTriggerOnImageChange &&
			trigger.ImageChangeParams != nil &&
			trigger.ImageChangeParams.From.Kind == "ImageStreamTag" {

			isname, _, err := imageapi.ParseImageStreamTagName(trigger.ImageChangeParams.From.Name)
			if err != nil {
				return nil, err
			}

			namespace := trigger.ImageChangeParams.From.Namespace
			if len(namespace) == 0 {
				namespace = o.Attach.Pod.Namespace
			}

			isi, err := o.Client.ImageStreamImages(namespace).Get(isname, ref.ID)
			if err != nil {
				return nil, err
			}

			return &isi.Image, nil
		}
	}

	return nil, nil // DeploymentConfig doesn't have an ImageChange Trigger
}

// getContainerImageViaImageStreamImport attempts to return an Image for a given
// Container.  It does this by submiting a ImageStreamImport request with Import
// set to false.  The request will not succeed if the backing repository
// requires Insecure to be set to true, which cannot be hard-coded for security
// reasons.
func (o *DebugOptions) getContainerImageViaImageStreamImport(container *kapi.Container) (*imageapi.Image, error) {
	isi := &imageapi.ImageStreamImport{
		ObjectMeta: metav1.ObjectMeta{
			Name: "oc-debug",
		},
		Spec: imageapi.ImageStreamImportSpec{
			Images: []imageapi.ImageImportSpec{
				{
					From: kapi.ObjectReference{
						Kind: "DockerImage",
						Name: container.Image,
					},
				},
			},
		},
	}

	isi, err := o.Client.ImageStreams(o.Attach.Pod.Namespace).Import(isi)
	if err != nil {
		return nil, err
	}

	if len(isi.Status.Images) > 0 {
		return isi.Status.Images[0].Image, nil
	}

	return nil, nil
}

func (o *DebugOptions) getContainerImageCommand(pod *kapi.Pod, container *kapi.Container) ([]string, error) {
	if len(container.Command) > 0 {
		return container.Command, nil
	}

	image, _ := o.getContainerImageViaDeploymentConfig(pod, container)
	if image == nil {
		image, _ = o.getContainerImageViaImageStreamImport(container)
	}

	if image == nil || image.DockerImageMetadata.Config == nil {
		return nil, errors.New("error: no usable image found")
	}

	config := image.DockerImageMetadata.Config
	return append(config.Entrypoint, config.Cmd...), nil
}

// transformPodForDebug alters the input pod to be debuggable
func (o *DebugOptions) transformPodForDebug(annotations map[string]string) (*kapi.Pod, []string) {
	pod := o.Attach.Pod

	if !o.KeepInitContainers {
		pod.Spec.InitContainers = nil
	}

	// reset the container
	container := containerForName(pod, o.Attach.ContainerName)

	// identify the command to be run
	originalCommand, _ := o.getContainerImageCommand(pod, container)
	if len(originalCommand) > 0 {
		originalCommand = append(originalCommand, container.Args...)
	}

	container.Command = o.Command
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

	if container.SecurityContext == nil {
		container.SecurityContext = &kapi.SecurityContext{}
	}
	switch {
	case o.AsNonRoot:
		b := true
		container.SecurityContext.RunAsNonRoot = &b
	case o.AsRoot:
		zero := int64(0)
		container.SecurityContext.RunAsUser = &zero
		container.SecurityContext.RunAsNonRoot = nil
	case o.AsUser != -1:
		container.SecurityContext.RunAsUser = &o.AsUser
		container.SecurityContext.RunAsNonRoot = nil
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
	} else {
		pod.Labels = map[string]string{}
	}
	// always clear the NodeName
	pod.Spec.NodeName = o.NodeName

	pod.ResourceVersion = ""
	pod.Spec.RestartPolicy = kapi.RestartPolicyNever

	pod.Status = kapi.PodStatus{}
	pod.UID = ""
	pod.CreationTimestamp = metav1.Time{}
	pod.SelfLink = ""

	return pod, originalCommand
}

// createPod creates the debug pod, and will attempt to delete an existing debug
// pod with the same name, but will return an error in any other case.
func (o *DebugOptions) createPod(pod *kapi.Pod) (*kapi.Pod, error) {
	namespace, name := pod.Namespace, pod.Name

	// create the pod
	created, err := o.Attach.PodClient.Pods(namespace).Create(pod)
	if err == nil || !kapierrors.IsAlreadyExists(err) {
		return created, err
	}

	// only continue if the pod has the right annotations
	existing, err := o.Attach.PodClient.Pods(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if existing.Annotations[debugPodAnnotationSourceResource] != o.Annotations[debugPodAnnotationSourceResource] {
		return nil, fmt.Errorf("a pod already exists named %q, please delete it before running debug", name)
	}

	// delete the existing pod
	if err := o.Attach.PodClient.Pods(namespace).Delete(name, metav1.NewDeleteOptions(0)); err != nil && !kapierrors.IsNotFound(err) {
		return nil, fmt.Errorf("unable to delete existing debug pod %q: %v", name, err)
	}
	return o.Attach.PodClient.Pods(namespace).Create(pod)
}

func containerForName(pod *kapi.Pod, name string) *kapi.Container {
	for i, c := range pod.Spec.Containers {
		if c.Name == name {
			return &pod.Spec.Containers[i]
		}
	}
	for i, c := range pod.Spec.InitContainers {
		if c.Name == name {
			return &pod.Spec.InitContainers[i]
		}
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

package debug

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kappsv1 "k8s.io/api/apps/v1"
	kappsv1beta1 "k8s.io/api/apps/v1beta1"
	kappsv1beta2 "k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapiv1 "k8s.io/kubernetes/pkg/apis/core/v1"
	"k8s.io/kubernetes/pkg/kubectl"
	kcmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/resource"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"
	"k8s.io/kubernetes/pkg/kubectl/scheme"
	"k8s.io/kubernetes/pkg/kubectl/util/term"
	"k8s.io/kubernetes/pkg/util/interrupt"

	appsv1 "github.com/openshift/api/apps/v1"
	dockerv10 "github.com/openshift/api/image/docker10"
	imagev1 "github.com/openshift/api/image/v1"
	appsv1client "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	imagev1client "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	appsutil "github.com/openshift/origin/pkg/apps/util"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageutil "github.com/openshift/origin/pkg/image/util"
	"github.com/openshift/origin/pkg/oc/lib/conditions"
	generateapp "github.com/openshift/origin/pkg/oc/lib/newapp/app"
	utilenv "github.com/openshift/origin/pkg/oc/util/env"
)

const (
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

type DebugOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	Attach kcmd.AttachOptions

	CoreClient  corev1client.CoreV1Interface
	AppsClient  appsv1client.AppsV1Interface
	ImageClient imagev1client.ImageV1Interface

	Printer          printers.ResourcePrinter
	LogsForObject    polymorphichelpers.LogsForObjectFunc
	RESTClientGetter genericclioptions.RESTClientGetter

	NoStdin    bool
	ForceTTY   bool
	DisableTTY bool
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
	AddEnv             []corev1.EnvVar
	RemoveEnv          []string
	Resources          []string
	Builder            func() *resource.Builder
	Namespace          string
	ExplicitNamespace  bool
	DryRun             bool
	FullCmdName        string

	resource.FilenameOptions
	genericclioptions.IOStreams
}

func NewDebugOptions(streams genericclioptions.IOStreams) *DebugOptions {
	attachOpts := kcmd.NewAttachOptions(streams)
	attachOpts.TTY = true
	attachOpts.Stdin = true
	return &DebugOptions{
		PrintFlags:         genericclioptions.NewPrintFlags("").WithTypeSetter(scheme.Scheme),
		IOStreams:          streams,
		Timeout:            15 * time.Minute,
		KeepInitContainers: true,
		AsUser:             -1,
		Attach:             *attachOpts,
		LogsForObject:      polymorphichelpers.LogsForObjectFn,
	}
}

// NewCmdDebug creates a command for debugging pods.
func NewCmdDebug(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewDebugOptions(streams)
	cmd := &cobra.Command{
		Use:     "debug RESOURCE/NAME [ENV1=VAL1 ...] [-c CONTAINER] [flags] [-- COMMAND]",
		Short:   "Launch a new instance of a pod for debugging",
		Long:    debugLong,
		Example: fmt.Sprintf(debugExample, fmt.Sprintf("%s debug", fullName)),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(cmd, f, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.RunDebug())
		},
	}

	usage := "to read a template"
	kcmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, usage)

	// FIXME-REBASE: we need to wire jsonpath here and other printers
	cmd.Flags().Bool("no-headers", false, "If true, when using the default output, don't print headers.")
	cmd.Flags().MarkHidden("no-headers")
	cmd.Flags().String("sort-by", "", "If non-empty, sort list types using this field specification.  The field specification is expressed as a JSONPath expression (e.g. 'ObjectMeta.Name'). The field in the API resource specified by this JSONPath expression must be an integer or a string.")
	cmd.Flags().MarkHidden("sort-by")
	cmd.Flags().Bool("show-all", true, "When printing, show all resources (default hide terminated pods.)")
	cmd.Flags().MarkHidden("show-all")
	cmd.Flags().Bool("show-labels", false, "When printing, show all labels as the last column (default hide labels column)")

	cmd.Flags().BoolVarP(&o.NoStdin, "no-stdin", "I", o.NoStdin, "Bypasses passing STDIN to the container, defaults to true if no command specified")
	cmd.Flags().BoolVarP(&o.ForceTTY, "tty", "t", o.ForceTTY, "Force a pseudo-terminal to be allocated")
	cmd.Flags().BoolVarP(&o.DisableTTY, "no-tty", "T", o.DisableTTY, "Disable pseudo-terminal allocation")
	cmd.Flags().StringVarP(&o.Attach.ContainerName, "container", "c", o.Attach.ContainerName, "Container name; defaults to first container")
	cmd.Flags().BoolVar(&o.KeepAnnotations, "keep-annotations", o.KeepAnnotations, "If true, keep the original pod annotations")
	cmd.Flags().BoolVar(&o.KeepLiveness, "keep-liveness", o.KeepLiveness, "If true, keep the original pod liveness probes")
	cmd.Flags().BoolVar(&o.KeepInitContainers, "keep-init-containers", o.KeepInitContainers, "Run the init containers for the pod. Defaults to true.")
	cmd.Flags().BoolVar(&o.KeepReadiness, "keep-readiness", o.KeepReadiness, "If true, keep the original pod readiness probes")
	cmd.Flags().BoolVar(&o.OneContainer, "one-container", o.OneContainer, "If true, run only the selected container, remove all others")
	cmd.Flags().StringVar(&o.NodeName, "node-name", o.NodeName, "Set a specific node to run on - by default the pod will run on any valid node")
	cmd.Flags().BoolVar(&o.AsRoot, "as-root", o.AsRoot, "If true, try to run the container as the root user")
	cmd.Flags().Int64Var(&o.AsUser, "as-user", o.AsUser, "Try to run the container as a specific user UID (note: admins may limit your ability to use this flag)")

	o.PrintFlags.AddFlags(cmd)
	kcmdutil.AddDryRunFlag(cmd)

	return cmd
}

func (o *DebugOptions) Complete(cmd *cobra.Command, f kcmdutil.Factory, args []string) error {
	if i := cmd.ArgsLenAtDash(); i != -1 && i < len(args) {
		o.Command = args[i:]
		args = args[:i]
	}
	resources, envArgs, ok := utilenv.SplitEnvironmentFromResources(args)
	if !ok {
		return kcmdutil.UsageErrorf(cmd, "all resources must be specified before environment changes: %s", strings.Join(args, " "))
	}
	o.Resources = resources
	o.RESTClientGetter = f

	switch {
	case o.ForceTTY && o.NoStdin:
		return kcmdutil.UsageErrorf(cmd, "you may not specify -I and -t together")
	case o.ForceTTY && o.DisableTTY:
		return kcmdutil.UsageErrorf(cmd, "you may not specify -t and -T together")
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
		o.Attach.TTY = term.IsTerminal(o.In)
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

	var err error
	o.Namespace, o.ExplicitNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.Builder = f.NewBuilder

	internalAddEnv, removeEnv, err := utilenv.ParseEnv(envArgs, nil)
	if err != nil {
		return err
	}

	// convert EnvVars that we should add, from internal to external
	for _, internal := range internalAddEnv {
		external := corev1.EnvVar{}
		if err := kapiv1.Convert_core_EnvVar_To_v1_EnvVar(&internal, &external, nil); err != nil {
			return err
		}

		o.AddEnv = append(o.AddEnv, external)
	}

	o.RemoveEnv = removeEnv

	cmdParent := cmd.Parent()
	if cmdParent != nil && len(cmdParent.CommandPath()) > 0 && kcmdutil.IsSiblingCommandExists(cmd, "describe") {
		o.FullCmdName = cmdParent.CommandPath()
	}
	o.AsNonRoot = !o.AsRoot && cmd.Flag("as-root").Changed

	templateArgSpecified := o.PrintFlags.TemplatePrinterFlags != nil &&
		o.PrintFlags.TemplatePrinterFlags.TemplateArgument != nil &&
		len(*o.PrintFlags.TemplatePrinterFlags.TemplateArgument) > 0

	outputFormatSpecified := o.PrintFlags.OutputFormat != nil && len(*o.PrintFlags.OutputFormat) > 0

	// TODO: below should be turned into a method on PrintFlags
	if outputFormatSpecified || templateArgSpecified {
		if o.DryRun {
			o.PrintFlags.Complete("%s (dry run)")
		}
		o.Printer, err = o.PrintFlags.ToPrinter()
		if err != nil {
			return err
		}
	}

	config, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.Attach.Config = config

	o.CoreClient, err = corev1client.NewForConfig(config)
	if err != nil {
		return err
	}

	o.AppsClient, err = appsv1client.NewForConfig(config)
	if err != nil {
		return err
	}

	o.ImageClient, err = imagev1client.NewForConfig(config)
	if err != nil {
		return err
	}

	return nil
}

func (o DebugOptions) Validate() error {
	if (o.AsRoot || o.AsNonRoot) && o.AsUser > 0 {
		return fmt.Errorf("you may not specify --as-root and --as-user=%d at the same time", o.AsUser)
	}
	return nil
}

// Debug creates and runs a debugging pod.
func (o *DebugOptions) RunDebug() error {
	b := o.Builder().
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		NamespaceParam(o.Namespace).DefaultNamespace().
		SingleResourceType().
		ResourceNames("pods", o.Resources...).
		Flatten()
	if len(o.FilenameOptions.Filenames) > 0 {
		b.FilenameParam(o.ExplicitNamespace, &o.FilenameOptions)
	}
	one := false
	infos, err := b.Do().IntoSingleItemImplied(&one).Infos()
	if err != nil {
		return err
	}
	if !one {
		return fmt.Errorf("you must identify a resource with a pod template to debug")
	}

	templateV1, err := o.approximatePodTemplateForObject(infos[0].Object)
	if err != nil && templateV1 == nil {
		return fmt.Errorf("cannot debug %s: %v", infos[0].Name, err)
	}
	if err != nil {
		glog.V(4).Infof("Unable to get exact template, but continuing with fallback: %v", err)
	}
	template := &corev1.PodTemplateSpec{}
	if err := legacyscheme.Scheme.Convert(templateV1, template, nil); err != nil {
		return err
	}
	pod := &corev1.Pod{
		ObjectMeta: template.ObjectMeta,
		Spec:       template.Spec,
	}
	pod.Name, pod.Namespace = fmt.Sprintf("%s-debug", generateapp.MakeSimpleName(infos[0].Name)), infos[0].Namespace
	o.Attach.Pod = pod

	if len(o.Attach.ContainerName) == 0 && len(pod.Spec.Containers) > 0 {
		if len(o.FullCmdName) > 0 {
			fmt.Fprintf(o.ErrOut, "Defaulting container name to %s.\n", pod.Spec.Containers[0].Name)
			fmt.Fprintf(o.ErrOut, "Use '%s describe pod/%s -n %s' to see all of the containers in this pod.\n", o.FullCmdName, pod.Name, pod.Namespace)
			fmt.Fprintf(o.ErrOut, "\n")
		}

		glog.V(4).Infof("Defaulting container name to %s", pod.Spec.Containers[0].Name)
		o.Attach.ContainerName = pod.Spec.Containers[0].Name
	}

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

	o.Annotations[debugPodAnnotationSourceResource] = fmt.Sprintf("%s/%s", infos[0].Mapping.Resource, infos[0].Name)
	o.Annotations[debugPodAnnotationSourceContainer] = o.Attach.ContainerName

	pod, originalCommand := o.transformPodForDebug(o.Annotations)
	var commandString string
	switch {
	case len(originalCommand) > 0:
		commandString = strings.Join(originalCommand, " ")
	default:
		commandString = "<image entrypoint>"
	}

	if o.Printer != nil {
		return o.Printer.PrintObj(pod, o.Out)
	}

	glog.V(5).Infof("Creating pod: %#v", pod)
	fmt.Fprintf(o.ErrOut, "Debugging with pod/%s, original command: %s\n", pod.Name, commandString)
	pod, err = o.createPod(pod)
	if err != nil {
		return err
	}

	// ensure the pod is cleaned up on shutdown
	o.Attach.InterruptParent = interrupt.New(
		func(os.Signal) { os.Exit(1) },
		func() {
			stderr := o.ErrOut
			if stderr == nil {
				stderr = os.Stderr
			}
			fmt.Fprintf(stderr, "\nRemoving debug pod ...\n")
			if err := o.CoreClient.Pods(pod.Namespace).Delete(pod.Name, metav1.NewDeleteOptions(0)); err != nil {
				if !kapierrors.IsNotFound(err) {
					fmt.Fprintf(stderr, "error: unable to delete the debug pod %q: %v\n", pod.Name, err)
				}
			}
		},
	)

	glog.V(5).Infof("Created attach arguments: %#v", o.Attach)
	return o.Attach.InterruptParent.Run(func() error {
		w, err := o.CoreClient.Pods(pod.Namespace).Watch(metav1.SingleObject(pod.ObjectMeta))
		if err != nil {
			return err
		}
		fmt.Fprintf(o.ErrOut, "Waiting for pod to start ...\n")

		switch containerRunningEvent, err := watch.Until(o.Timeout, w, conditions.PodContainerRunning(o.Attach.ContainerName)); {
		// api didn't error right away but the pod wasn't even created
		case kapierrors.IsNotFound(err):
			msg := fmt.Sprintf("unable to create the debug pod %q", pod.Name)
			if len(o.NodeName) > 0 {
				msg += fmt.Sprintf(" on node %q", o.NodeName)
			}
			return fmt.Errorf(msg)
			// switch to logging output
		case err == kubectl.ErrPodCompleted, err == kubectl.ErrContainerTerminated, !o.Attach.Stdin:
			return kcmd.LogsOptions{
				Object: pod,
				Options: &corev1.PodLogOptions{
					Container: o.Attach.ContainerName,
					Follow:    true,
				},
				RESTClientGetter: o.RESTClientGetter,
				ConsumeRequestFn: kcmd.DefaultConsumeRequestFn,
				IOStreams:        o.IOStreams,
				LogsForObject:    o.LogsForObject,
			}.RunLogs()
		case err != nil:
			return err
		default:
			// TODO this doesn't do us much good for remote debugging sessions, but until we get a local port
			// set up to proxy, this is what we've got.
			if podWithStatus, ok := containerRunningEvent.Object.(*kapi.Pod); ok {
				fmt.Fprintf(o.Attach.ErrOut, "Pod IP: %s\n", podWithStatus.Status.PodIP)
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
func (o *DebugOptions) getContainerImageViaDeploymentConfig(pod *corev1.Pod, container *corev1.Container) (*imagev1.Image, error) {
	ref, err := imageapi.ParseDockerImageReference(container.Image)
	if err != nil {
		return nil, err
	}

	if ref.ID == "" {
		return nil, nil // ID is needed for later lookup
	}

	dcname := pod.Annotations[appsv1.DeploymentConfigAnnotation]
	if dcname == "" {
		return nil, nil // Pod doesn't appear to have been created by a DeploymentConfig
	}

	dc, err := o.AppsClient.DeploymentConfigs(o.Attach.Pod.Namespace).Get(dcname, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	for _, trigger := range dc.Spec.Triggers {
		if trigger.Type == appsv1.DeploymentTriggerOnImageChange &&
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

			isi, err := o.ImageClient.ImageStreamImages(namespace).Get(imageapi.JoinImageStreamImage(isname, ref.ID), metav1.GetOptions{})
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
func (o *DebugOptions) getContainerImageViaImageStreamImport(container *corev1.Container) (*imagev1.Image, error) {
	isi := &imagev1.ImageStreamImport{
		ObjectMeta: metav1.ObjectMeta{
			Name: "oc-debug",
		},
		Spec: imagev1.ImageStreamImportSpec{
			Images: []imagev1.ImageImportSpec{
				{
					From: corev1.ObjectReference{
						Kind: "DockerImage",
						Name: container.Image,
					},
				},
			},
		},
	}

	isi, err := o.ImageClient.ImageStreamImports(o.Attach.Pod.Namespace).Create(isi)
	if err != nil {
		return nil, err
	}

	if len(isi.Status.Images) > 0 {
		return isi.Status.Images[0].Image, nil
	}

	return nil, nil
}

func (o *DebugOptions) getContainerImageCommand(pod *corev1.Pod, container *corev1.Container) ([]string, error) {
	if len(container.Command) > 0 {
		return container.Command, nil
	}
	image, err := o.getContainerImageViaDeploymentConfig(pod, container)
	if err != nil {
		image, err = o.getContainerImageViaImageStreamImport(container)
		if err != nil {
			return nil, err
		}
	}

	if image == nil {
		return nil, fmt.Errorf("error: no usable image found")
	}

	if err := imageutil.ImageWithMetadata(image); err != nil {
		return nil, err
	}
	dockerImage, ok := image.DockerImageMetadata.Object.(*dockerv10.DockerImage)
	if !ok {
		return nil, err
	}

	return append(dockerImage.Config.Entrypoint, dockerImage.Config.Cmd...), nil
}

// transformPodForDebug alters the input pod to be debuggable
func (o *DebugOptions) transformPodForDebug(annotations map[string]string) (*corev1.Pod, []string) {
	pod := o.Attach.Pod

	if !o.KeepInitContainers {
		pod.Spec.InitContainers = nil
	}

	// reset the container
	container := containerForName(pod, o.Attach.ContainerName)

	// identify the command to be run
	originalCommand, _ := o.getContainerImageCommand(pod, container)
	if len(container.Command) > 0 {
		originalCommand = container.Command
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

	var newEnv []corev1.EnvVar
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
		container.SecurityContext = &corev1.SecurityContext{}
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
		pod.Spec.Containers = []corev1.Container{*container}
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
	pod.Spec.RestartPolicy = corev1.RestartPolicyNever

	pod.Status = corev1.PodStatus{}
	pod.UID = ""
	pod.CreationTimestamp = metav1.Time{}
	pod.SelfLink = ""

	// clear pod ownerRefs
	pod.ObjectMeta.OwnerReferences = []v1.OwnerReference{}

	return pod, originalCommand
}

// createPod creates the debug pod, and will attempt to delete an existing debug
// pod with the same name, but will return an error in any other case.
func (o *DebugOptions) createPod(pod *corev1.Pod) (*corev1.Pod, error) {
	namespace, name := pod.Namespace, pod.Name

	// create the pod
	created, err := o.CoreClient.Pods(namespace).Create(pod)
	if err == nil || !kapierrors.IsAlreadyExists(err) {
		return created, err
	}

	// only continue if the pod has the right annotations
	existing, err := o.CoreClient.Pods(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if existing.Annotations[debugPodAnnotationSourceResource] != o.Annotations[debugPodAnnotationSourceResource] {
		return nil, fmt.Errorf("a pod already exists named %q, please delete it before running debug", name)
	}

	// delete the existing pod
	if err := o.CoreClient.Pods(namespace).Delete(name, metav1.NewDeleteOptions(0)); err != nil && !kapierrors.IsNotFound(err) {
		return nil, fmt.Errorf("unable to delete existing debug pod %q: %v", name, err)
	}
	return o.CoreClient.Pods(namespace).Create(pod)
}

func containerForName(pod *corev1.Pod, name string) *corev1.Container {
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

func containerNames(pod *corev1.Pod) []string {
	var names []string
	for _, c := range pod.Spec.Containers {
		names = append(names, c.Name)
	}
	return names
}

func (o *DebugOptions) approximatePodTemplateForObject(object runtime.Object) (*corev1.PodTemplateSpec, error) {
	switch t := object.(type) {
	case *imagev1.ImageStreamTag:
		// create a minimal pod spec that uses the image referenced by the istag without any introspection
		// it possible that we could someday do a better job introspecting it
		return &corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyNever,
				Containers: []corev1.Container{
					{Name: "container-00", Image: t.Image.DockerImageReference},
				},
			},
		}, nil
	case *imagev1.ImageStreamImage:
		// create a minimal pod spec that uses the image referenced by the istag without any introspection
		// it possible that we could someday do a better job introspecting it
		return &corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyNever,
				Containers: []corev1.Container{
					{Name: "container-00", Image: t.Image.DockerImageReference},
				},
			},
		}, nil
	case *appsv1.DeploymentConfig:
		fallback := t.Spec.Template

		latestDeploymentName := appsutil.LatestDeploymentNameForConfig(t)
		deployment, err := o.CoreClient.ReplicationControllers(t.Namespace).Get(latestDeploymentName, metav1.GetOptions{})
		if err != nil {
			return fallback, err
		}

		fallback = deployment.Spec.Template

		pods, err := o.CoreClient.Pods(deployment.Namespace).List(metav1.ListOptions{LabelSelector: labels.SelectorFromSet(deployment.Spec.Selector).String()})
		if err != nil {
			return fallback, err
		}

		// If we have any pods available, find the newest
		// pod with regards to our most recent deployment.
		// If the fallback PodTemplateSpec is nil, prefer
		// the newest pod available.
		for i := range pods.Items {
			pod := &pods.Items[i]
			if fallback == nil || pod.CreationTimestamp.Before(&fallback.CreationTimestamp) {
				fallback = &corev1.PodTemplateSpec{
					ObjectMeta: pod.ObjectMeta,
					Spec:       pod.Spec,
				}
			}
		}
		return fallback, nil

	case *corev1.Pod:
		return &corev1.PodTemplateSpec{
			ObjectMeta: t.ObjectMeta,
			Spec:       t.Spec,
		}, nil

	// ReplicationController
	case *corev1.ReplicationController:
		return t.Spec.Template, nil

	// ReplicaSet
	case *extensionsv1beta1.ReplicaSet:
		return &t.Spec.Template, nil
	case *kappsv1beta2.ReplicaSet:
		return &t.Spec.Template, nil
	case *kappsv1.ReplicaSet:
		return &t.Spec.Template, nil

	// Deployment
	case *extensionsv1beta1.Deployment:
		return &t.Spec.Template, nil
	case *kappsv1beta1.Deployment:
		return &t.Spec.Template, nil
	case *kappsv1beta2.Deployment:
		return &t.Spec.Template, nil
	case *kappsv1.Deployment:
		return &t.Spec.Template, nil

	// DaemonSet
	case *extensionsv1beta1.DaemonSet:
		return &t.Spec.Template, nil
	case *kappsv1beta2.DaemonSet:
		return &t.Spec.Template, nil
	case *kappsv1.DaemonSet:
		return &t.Spec.Template, nil

	// Job
	case *batchv1.Job:
		return &t.Spec.Template, nil
	}

	return nil, fmt.Errorf("unable to extract pod template from type %v", reflect.TypeOf(object))
}

package mustgather

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/kubectl/cmd/logs"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"
	"k8s.io/kubernetes/pkg/kubectl/scheme"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	imagev1client "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	"github.com/openshift/library-go/pkg/image/imageutil"
	"github.com/openshift/library-go/pkg/operator/resource/retry"

	"github.com/openshift/oc/pkg/cli/rsync"
)

var (
	mustGatherLong = templates.LongDesc(`
		Launch a pod to gather debugging information

		This command will launch a pod in a temporary namespace on your cluster that gathers 
		debugging information and then downloads the gathered information.

		Experimental: This command is under active development and may change without notice.
	`)

	mustGatherExample = templates.Examples(`
		# gather information using the default plug-in image and command, writing into ./must-gather.local.<rand>
		  oc adm must-gather

		# gather information with a specific local folder to copy to
		  oc adm must-gather --dest-dir=/local/directory

		# gather information using multiple plug-in images 
		  oc adm must-gather --image=quay.io/kubevirt/must-gather --image=quay.io/openshift/origin-must-gather

		# gather information using a specific image stream plug-in 
		  oc adm must-gather --image-stream=openshift/must-gather:latest

		# gather information using a specific image, command, and pod-dir
		  oc adm must-gather --image=my/image:tag --source-dir=/pod/directory -- myspecial-command.sh
	`)
)

func NewMustGatherCommand(f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewMustGatherOptions(streams)
	cmd := &cobra.Command{
		Use:     "must-gather",
		Short:   "Launch a new instance of a pod for gathering debug information",
		Long:    mustGatherLong,
		Example: mustGatherExample,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVar(&o.NodeName, "node-name", o.NodeName, "Set a specific node to use - by default a random master will be used")
	cmd.Flags().StringSliceVar(&o.Images, "image", o.Images, "Specify a must-gather plugin image to run. If not specified, OpenShift's default must-gather image will be used.")
	cmd.Flags().StringSliceVar(&o.ImageStreams, "image-stream", o.ImageStreams, "Specify an image stream (namespace/name:tag) containing a must-gather plugin image to run.")
	cmd.Flags().StringVar(&o.DestDir, "dest-dir", o.DestDir, "Set a specific directory on the local machine to write gathered data to.")
	cmd.Flags().StringVar(&o.SourceDir, "source-dir", o.SourceDir, "Set the specific directory on the pod copy the gathered data from.")
	cmd.Flags().BoolVar(&o.Keep, "keep", o.Keep, "Do not delete temporary resources when command completes.")
	cmd.Flags().MarkHidden("keep")

	return cmd
}

func NewMustGatherOptions(streams genericclioptions.IOStreams) *MustGatherOptions {
	return &MustGatherOptions{
		SourceDir: "/must-gather/",
		IOStreams: streams,
	}
}

func (o *MustGatherOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	o.RESTClientGetter = f
	var err error
	if o.Config, err = f.ToRESTConfig(); err != nil {
		return err
	}
	if o.Client, err = kubernetes.NewForConfig(o.Config); err != nil {
		return err
	}
	if o.ImageClient, err = imagev1client.NewForConfig(o.Config); err != nil {
		return err
	}
	if i := cmd.ArgsLenAtDash(); i != -1 && i < len(args) {
		o.Command = args[i:]
	} else {
		o.Command = args
	}
	if len(o.DestDir) == 0 {
		o.DestDir = fmt.Sprintf("must-gather.local.%06d", rand.Int63())
	}
	if err := o.completeImages(); err != nil {
		return err
	}
	o.PrinterCreated, err = printers.NewTypeSetter(scheme.Scheme).WrapToPrinter(&printers.NamePrinter{Operation: "created"}, nil)
	if err != nil {
		return err
	}
	o.PrinterDeleted, err = printers.NewTypeSetter(scheme.Scheme).WrapToPrinter(&printers.NamePrinter{Operation: "deleted"}, nil)
	if err != nil {
		return err
	}
	o.RsyncRshCmd = rsync.DefaultRsyncRemoteShellToUse(cmd.Parent())
	return nil
}

func (o *MustGatherOptions) completeImages() error {
	for _, imageStream := range o.ImageStreams {
		if image, err := o.resolveImageStreamTagString(imageStream); err == nil {
			o.Images = append(o.Images, image)
		} else {
			return fmt.Errorf("unable to resolve image stream '%v': %v", imageStream, err)
		}
	}
	if len(o.Images) == 0 {
		var image string
		var err error
		if image, err = o.resolveImageStreamTag("openshift", "must-gather", "latest"); err != nil {
			o.log("%v\n", err)
			image = "quay.io/openshift/origin-must-gather:latest"
		}
		o.Images = append(o.Images, image)
	}
	o.log("Using must-gather plugin-in image: %s", strings.Join(o.Images, ", "))
	return nil
}

func (o *MustGatherOptions) resolveImageStreamTagString(s string) (string, error) {
	namespace, name, tag := parseImageStreamTagString(s)
	if len(namespace) == 0 {
		return "", fmt.Errorf("expected namespace/name:tag")
	}
	return o.resolveImageStreamTag(namespace, name, tag)
}

func parseImageStreamTagString(s string) (string, string, string) {
	var namespace, nameAndTag string
	parts := strings.SplitN(s, "/", 2)
	switch len(parts) {
	case 2:
		namespace = parts[0]
		nameAndTag = parts[1]
	case 1:
		nameAndTag = parts[0]
	}
	name, tag, _ := imageutil.SplitImageStreamTag(nameAndTag)
	return namespace, name, tag
}

func (o *MustGatherOptions) resolveImageStreamTag(namespace, name, tag string) (string, error) {
	imageStream, err := o.ImageClient.ImageStreams(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	var image string
	var ok bool
	if image, ok = imageutil.ResolveLatestTaggedImage(imageStream, tag); !ok {
		return "", fmt.Errorf("unable to resolve the imagestream tag %s/%s:%s", namespace, name, tag)
	}
	return image, nil
}

type MustGatherOptions struct {
	genericclioptions.IOStreams

	Config           *rest.Config
	Client           kubernetes.Interface
	ImageClient      imagev1client.ImageV1Interface
	RESTClientGetter genericclioptions.RESTClientGetter

	NodeName     string
	DestDir      string
	SourceDir    string
	Images       []string
	ImageStreams []string
	Command      []string
	Keep         bool

	RsyncRshCmd string

	PrinterCreated printers.ResourcePrinter
	PrinterDeleted printers.ResourcePrinter
}

func (o *MustGatherOptions) Validate() error {
	if len(o.Images) == 0 {
		return fmt.Errorf("missing an image")
	}
	return nil
}

// Run creates and runs a must-gather pod.d
func (o *MustGatherOptions) Run() error {
	var err error

	// create namespace
	ns, err := o.Client.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "openshift-must-gather-",
			Labels: map[string]string{
				"openshift.io/run-level": "0",
			},
			Annotations: map[string]string{
				"oc.openshift.io/command": "oc adm must-gather",
			},
		},
	})
	if err != nil {
		return err
	}
	o.PrinterCreated.PrintObj(ns, o.Out)
	if !o.Keep {
		defer func() {
			if err := o.Client.CoreV1().Namespaces().Delete(ns.Name, nil); err != nil {
				fmt.Printf("%v", err)
				return
			}
			o.PrinterDeleted.PrintObj(ns, o.Out)
		}()
	}

	clusterRoleBinding, err := o.Client.RbacV1().ClusterRoleBindings().Create(o.newClusterRoleBinding(ns.Name))
	if err != nil {
		return err
	}
	o.PrinterCreated.PrintObj(clusterRoleBinding, o.Out)
	if !o.Keep {
		defer func() {
			if err := o.Client.RbacV1().ClusterRoleBindings().Delete(clusterRoleBinding.Name, &metav1.DeleteOptions{}); err != nil {
				fmt.Printf("%v", err)
				return
			}
			o.PrinterDeleted.PrintObj(clusterRoleBinding, o.Out)
		}()
	}

	// create pods
	var pods []*corev1.Pod
	for _, image := range o.Images {
		pod, err := o.Client.CoreV1().Pods(ns.Name).Create(o.newPod(o.NodeName, image))
		if err != nil {
			return err
		}
		o.log("[%s] pod for plug-in image %s created", pod.Name, image)
		pods = append(pods, pod)
	}

	var wg sync.WaitGroup
	wg.Add(len(pods))
	errs := make(chan error, len(pods))
	for _, pod := range pods {
		go func(pod *corev1.Pod) {
			defer wg.Done()

			// wait for gather container to be running (gather is running)
			if err := o.waitForGatherContainerRunning(pod); err != nil {
				o.log("[%s] gather did not start: %s", pod.Name, err)
				errs <- fmt.Errorf("gather did not start for pod %s: %s", pod.Name, err)
				return
			}
			// stream gather container logs
			if err := o.getGatherContainerLogs(pod); err != nil {
				o.log("[%s] gather logs unavailable: %v", pod.Name, err)
			}

			// wait for pod to be running (gather has completed)
			o.log("[%s] waiting for gather to complete ", pod.Name)
			if err := o.waitForPodRunning(pod); err != nil {
				o.log("[%s] gather never finished: %v", pod.Name, err)
				errs <- fmt.Errorf("gather never finished for pod %s: %s", pod.Name, err)
				return
			}

			// copy the gathered files into the local destination dir
			o.log("[%s] downloading gather output", pod.Name)
			if err := o.copyFilesFromPod(pod, len(pods) > 1); err != nil {
				o.log("[%s] gather output not downloaded: %v\n", pod.Name, err)
				errs <- fmt.Errorf("unable to download output from pod %s: %s", pod.Name, err)
				return
			}
		}(pod)
	}
	wg.Wait()
	return aggregateErrorOrNil(errs)
}

func aggregateErrorOrNil(errs <-chan error) error {
	c := len(errs)
	if c == 0 {
		return nil
	}
	var arr []error
	for i := 0; i < c; i++ {
		arr = append(arr, <-errs)
	}
	return errors.NewAggregate(arr)
}

func (o *MustGatherOptions) log(format string, a ...interface{}) {
	fmt.Fprintf(o.Out, format+"\n", a...)
}

func (o *MustGatherOptions) copyFilesFromPod(pod *corev1.Pod, createSubDir bool) error {
	streams := o.IOStreams
	streams.Out = newPrefixWriter(streams.Out, pod.Name)
	destDir := o.DestDir
	if createSubDir {
		destDir = path.Join(o.DestDir, pod.Name)
		if err := os.MkdirAll(destDir, 0775); err != nil {
			return err
		}
	}
	rsyncOptions := &rsync.RsyncOptions{
		Namespace:     pod.Namespace,
		Source:        &rsync.PathSpec{PodName: pod.Name, Path: path.Clean(o.SourceDir) + "/"},
		ContainerName: "copy",
		Destination:   &rsync.PathSpec{PodName: "", Path: destDir},
		Client:        o.Client,
		Config:        o.Config,
		RshCmd:        fmt.Sprintf("%s --namespace=%s", o.RsyncRshCmd, pod.Namespace),
		IOStreams:     streams,
	}
	rsyncOptions.Strategy = rsync.NewDefaultCopyStrategy(rsyncOptions)
	return rsyncOptions.RunRsync()
}

func (o *MustGatherOptions) getGatherContainerLogs(pod *corev1.Pod) error {
	return (&logs.LogsOptions{
		Namespace:   pod.Namespace,
		ResourceArg: pod.Name,
		Options: &corev1.PodLogOptions{
			Follow:    true,
			Container: pod.Spec.InitContainers[0].Name,
		},
		RESTClientGetter: o.RESTClientGetter,
		Object:           pod,
		ConsumeRequestFn: consumeRequestFn(pod.Name),
		LogsForObject:    polymorphichelpers.LogsForObjectFn,
		IOStreams:        genericclioptions.IOStreams{Out: o.Out},
	}).RunLogs()
}

func consumeRequestFn(prefix string) func(rest.ResponseWrapper, io.Writer) error {
	return func(response rest.ResponseWrapper, out io.Writer) error {
		return logs.DefaultConsumeRequest(response, newPrefixWriter(out, prefix))
	}
}

func newPrefixWriter(out io.Writer, prefix string) io.Writer {
	reader, writer := io.Pipe()
	scanner := bufio.NewScanner(reader)
	go func() {
		for scanner.Scan() {
			fmt.Fprintf(out, "[%s] %s\n", prefix, scanner.Text())
		}
	}()
	return writer
}

func (o *MustGatherOptions) waitForPodRunning(pod *corev1.Pod) error {
	phase := pod.Status.Phase
	err := wait.PollImmediate(time.Second, 10*time.Minute, func() (bool, error) {
		var err error
		if pod, err = o.Client.CoreV1().Pods(pod.Namespace).Get(pod.Name, metav1.GetOptions{}); err != nil {
			return false, nil
		}
		phase = pod.Status.Phase
		return phase != corev1.PodPending, nil
	})
	if err != nil {
		return err
	}
	if phase != corev1.PodRunning {
		return fmt.Errorf("pod is not running: %v\n", phase)
	}
	return nil
}

func (o *MustGatherOptions) waitForGatherContainerRunning(pod *corev1.Pod) error {
	return wait.PollImmediate(time.Second, 10*time.Minute, func() (bool, error) {
		var err error
		if pod, err = o.Client.CoreV1().Pods(pod.Namespace).Get(pod.Name, metav1.GetOptions{}); err == nil {
			if len(pod.Status.InitContainerStatuses) == 0 {
				return false, nil
			}
			state := pod.Status.InitContainerStatuses[0].State
			if state.Waiting != nil && state.Waiting.Reason == "ErrImagePull" {
				return true, fmt.Errorf("unable to pull image: %v: %v", state.Waiting.Reason, state.Waiting.Message)
			}
			running := state.Running != nil
			terminated := state.Terminated != nil
			return running || terminated, nil
		}
		if retry.IsHTTPClientError(err) {
			return false, nil
		}
		return false, err
	})
}

func (o *MustGatherOptions) newClusterRoleBinding(ns string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "must-gather-",
			Annotations: map[string]string{
				"oc.openshift.io/command": "oc adm must-gather",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "default",
				Namespace: ns,
			},
		},
	}
}

// newPod creates a pod with 2 containers with a shared volume mount:
// - gather: init containers that run gather command
// - copy: no-op container we can exec into
func (o *MustGatherOptions) newPod(node, image string) *corev1.Pod {
	zero := int64(0)
	ret := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "must-gather-",
			Labels: map[string]string{
				"app": "must-gather",
			},
		},
		Spec: corev1.PodSpec{
			NodeName:      node,
			RestartPolicy: corev1.RestartPolicyNever,
			Volumes: []corev1.Volume{
				{
					Name: "must-gather-output",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:    "gather",
					Image:   image,
					Command: []string{"/usr/bin/gather"},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "must-gather-output",
							MountPath: path.Clean(o.SourceDir),
							ReadOnly:  false,
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:    "copy",
					Image:   image,
					Command: []string{"/bin/bash", "-c", "trap : TERM INT; sleep infinity & wait"},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "must-gather-output",
							MountPath: path.Clean(o.SourceDir),
							ReadOnly:  false,
						},
					},
				},
			},
			TerminationGracePeriodSeconds: &zero,
			Tolerations: []corev1.Toleration{
				{
					Operator: "Exists",
				},
			},
		},
	}
	if len(o.Command) > 0 {
		ret.Spec.InitContainers[0].Command = o.Command
	}
	return ret
}

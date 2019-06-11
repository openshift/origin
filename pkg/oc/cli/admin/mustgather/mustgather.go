package mustgather

import (
	"fmt"
	"math/rand"
	"path"
	"time"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	v1 "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	"github.com/openshift/library-go/pkg/image/imageutil"
	"github.com/openshift/library-go/pkg/operator/resource/retry"
	"github.com/openshift/oc/pkg/cli/rsync"
)

var (
	mustGatherLong = templates.LongDesc(`
		Launch a pod to gather debugging information

		This command will launch a pod in a temporary namespace on your
		cluster that gathers debugging information, using a copy of the active
		client config context, and then downloads the gathered information.

		Experimental: This command is under active development and may change without notice.
	`)

	mustGatherExample = templates.Examples(`
		# gather default information using the default image and command, writing into ./must-gather.local.<rand>
		  oc adm must-gather

		# gather default information with a specific local folder to copy to
		  oc adm must-gather --dest-dir=/local/directory

		# gather default information using a specific image, command, and pod-dir
		  oc adm must-gather --image=my/image:tag --source-dir=/pod/directory -- myspecial-command.sh
	`)
)

func NewMustGatherCommand(f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewMustGatherOptions(streams)
	rsyncCommand := rsync.NewCmdRsync(rsync.RsyncRecommendedName, "", f, streams)
	cmd := &cobra.Command{
		Use:     "must-gather",
		Short:   "Launch a new instance of a pod for gathering debug information",
		Long:    mustGatherLong,
		Example: mustGatherExample,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Run(rsyncCommand))
		},
	}

	cmd.Flags().StringVar(&o.NodeName, "node-name", o.NodeName, "Set a specific node to use - by default a random master will be used")
	cmd.Flags().StringVar(&o.Image, "image", o.Image, "Set a specific image to use, by default the OpenShift's must-gather image will be used.")
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
	if i := cmd.ArgsLenAtDash(); i != -1 && i < len(args) {
		o.Command = args[i:]
	} else {
		o.Command = args
	}
	o.RESTClientGetter = f
	var err error
	if o.Config, err = f.ToRESTConfig(); err != nil {
		return err
	}
	if o.Client, err = kubernetes.NewForConfig(o.Config); err != nil {
		return err
	}
	if len(o.DestDir) == 0 {
		o.DestDir = fmt.Sprintf("must-gather.local.%06d", rand.Int63())
	}
	if len(o.Image) == 0 {
		if o.Image, err = o.resolveMustGatherImage(); err != nil {
			o.Image = "quay.io/openshift/origin-must-gather:latest"
			fmt.Fprintf(o.Out, "%v\n", err)
		}
	}
	fmt.Fprintf(o.Out, "Using image: %s\n", o.Image)
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

func (o *MustGatherOptions) resolveMustGatherImage() (string, error) {
	imageClient, err := v1.NewForConfig(o.Config)
	if err != nil {
		return "", err
	}
	imageStream, err := imageClient.ImageStreams("openshift").Get("must-gather", metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	var image string
	var ok bool
	if image, ok = imageutil.ResolveLatestTaggedImage(imageStream, "latest"); !ok {
		return "", fmt.Errorf("unable to resolve the openshift imagestream tag must-gather:latest")
	}
	return image, nil
}

type MustGatherOptions struct {
	genericclioptions.IOStreams

	Config           *rest.Config
	Client           kubernetes.Interface
	RESTClientGetter genericclioptions.RESTClientGetter

	NodeName  string
	DestDir   string
	SourceDir string
	Image     string
	Command   []string
	Keep      bool

	RsyncRshCmd string

	PrinterCreated printers.ResourcePrinter
	PrinterDeleted printers.ResourcePrinter
}

// Run creates and runs a must-gather pod.d
func (o *MustGatherOptions) Run(rsyncCmd *cobra.Command) error {
	if len(o.Image) == 0 {
		return fmt.Errorf("missing an image")
	}

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

	// create pod
	pod, err := o.Client.CoreV1().Pods(ns.Name).Create(o.newPod(o.NodeName))
	if err != nil {
		return err
	}

	// wait for gather container to be running (gather is running)
	if err := o.waitForGatherContainerRunning(pod); err != nil {
		return err
	}

	// stream gather container logs
	if err := o.getInitContainerLogs(pod); err != nil {
		fmt.Fprintf(o.Out, "container logs unavailable: %v\n", err)
	}

	// wait for pod to be running (gather has completed)
	if err := o.waitForPodRunning(pod); err != nil {
		return err
	}

	// copy the gathered files into the local destination dir
	err = o.copyFilesFromPod(pod)
	return err
}

func (o *MustGatherOptions) copyFilesFromPod(pod *corev1.Pod) error {
	rsyncOptions := &rsync.RsyncOptions{
		Namespace:     pod.Namespace,
		Source:        &rsync.PathSpec{PodName: pod.Name, Path: path.Clean(o.SourceDir) + "/"},
		ContainerName: "copy",
		Destination:   &rsync.PathSpec{PodName: "", Path: o.DestDir},
		Client:        o.Client,
		Config:        o.Config,
		RshCmd:        fmt.Sprintf("%s --namespace=%s", o.RsyncRshCmd, pod.Namespace),
		IOStreams:     o.IOStreams,
	}
	rsyncOptions.Strategy = rsync.NewDefaultCopyStrategy(rsyncOptions)
	return rsyncOptions.RunRsync()

}

func (o *MustGatherOptions) getInitContainerLogs(pod *corev1.Pod) error {
	return (&logs.LogsOptions{
		Namespace:   pod.Namespace,
		ResourceArg: pod.Name,
		Options: &corev1.PodLogOptions{
			Follow:    true,
			Container: pod.Spec.InitContainers[0].Name,
		},
		RESTClientGetter: o.RESTClientGetter,
		Object:           pod,
		ConsumeRequestFn: logs.DefaultConsumeRequest,
		LogsForObject:    polymorphichelpers.LogsForObjectFn,
		IOStreams:        genericclioptions.IOStreams{Out: o.Out},
	}).RunLogs()
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
// - gather: init container that runs gather command
// - copy: no-op container we can exec into
func (o *MustGatherOptions) newPod(node string) *corev1.Pod {
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
					Image:   o.Image,
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
					Image:   o.Image,
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

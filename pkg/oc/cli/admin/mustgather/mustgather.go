package mustgather

import (
	"fmt"
	"math/rand"

	"k8s.io/client-go/rest"

	"k8s.io/client-go/tools/clientcmd"

	"k8s.io/client-go/kubernetes"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/clientcmd/api/latest"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

var (
	mustGatherLong = templates.LongDesc(`
		Launch a pod to gather debugging information

		This command will
          1. Create a temporary namespace
          2. Create a secret an upload the local kubeconfig content into it
          3. Look up the image to use for gathering information
          4. Create a pod that runs the 'gather' command in that image
          5. Wait for completion of that command
          6. Copy the gathered data locally
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

type MustGatherFlags struct {
	ConfigFlags *genericclioptions.ConfigFlags

	NodeName string
	DestDir  string
	Image    string

	genericclioptions.IOStreams
}

func NewMustGatherFlags(streams genericclioptions.IOStreams) *MustGatherFlags {
	return &MustGatherFlags{
		ConfigFlags: genericclioptions.NewConfigFlags(),
		IOStreams:   streams,
	}
}

func NewMustGatherCommand(restClientGetter genericclioptions.RESTClientGetter, streams genericclioptions.IOStreams) *cobra.Command {
	f := NewMustGatherFlags(streams)
	cmd := &cobra.Command{
		Use:     "must-gather",
		Short:   "Launch a new instance of a pod for gathering debug information",
		Long:    mustGatherLong,
		Example: mustGatherExample,
		Run: func(cmd *cobra.Command, args []string) {
			o, err := f.Complete(cmd, restClientGetter, args)
			kcmdutil.CheckErr(err)

			kcmdutil.CheckErr(o.RunMustGather())
		},
	}

	cmd.Flags().StringVar(&f.NodeName, "node-name", f.NodeName, "Set a specific node to use - by default a random master will be used")
	cmd.Flags().StringVar(&f.Image, "image", f.Image, "Set a specific to use - by default the image will be looked up for OpenShift's must-gather")
	cmd.Flags().StringVar(&f.DestDir, "dest-dir", f.DestDir, "Set a specific directory on the local machine to write gathered data to.")

	return cmd
}

func (f *MustGatherFlags) Complete(cmd *cobra.Command, restClientGetter genericclioptions.RESTClientGetter, args []string) (*MustGatherOptions, error) {
	o := &MustGatherOptions{
		NodeName:  f.NodeName,
		Image:     f.Image,
		IOStreams: f.IOStreams,
	}
	if i := cmd.ArgsLenAtDash(); i != -1 && i < len(args) {
		o.Command = args[i:]
	} else {
		o.Command = args
	}

	clientConfig, err := restClientGetter.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	o.KubeClient, err = kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	o.KubeConfig = restClientGetter.ToRawKubeConfigLoader()

	if len(o.Command) == 0 {
		o.Command = []string{"/bin/sh"}
	}

	o.DestDir = f.DestDir
	if len(o.DestDir) == 0 {
		o.DestDir = fmt.Sprintf("must-gather.local.%06d", rand.Int63())

	}

	return o, nil
}

type MustGatherOptions struct {
	ClientConfig *rest.Config
	KubeClient   kubernetes.Interface

	NodeName   string
	DestDir    string
	Image      string
	Command    []string
	KubeConfig clientcmd.ClientConfig

	genericclioptions.IOStreams
}

// MustGather creates and runs a mustGatherging pod.
func (o *MustGatherOptions) RunMustGather() error {
	if len(o.ClientConfig.BearerToken) != 0 {
		return fmt.Errorf("cannot run must-gather with a token based kubeconfig")
	}
	if len(o.Image) != 0 {
		return fmt.Errorf("missing an image")
	}
	ns, err := o.KubeClient.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "openshift-must-gather-",
			Labels: map[string]string{
				"openshift.io/run-level": "0",
			},
		},
	})
	if err != nil {
		return err
	}
	fmt.Fprint(o.Out, "Created ns/%s\n", ns.Name)

	kubeConfigSecret, err := o.newKubeConfigSecret(ns.Name)
	if err != nil {
		return err
	}
	kubeConfigSecret, err = o.KubeClient.CoreV1().Secrets(kubeConfigSecret.Name).Create(kubeConfigSecret)
	if err != nil {
		return err
	}

	err = o.KubeClient.CoreV1().Namespaces().Delete(ns.Name, nil)
	if err != nil {
		return err
	}
	fmt.Fprint(o.Out, "Deleted ns/%s\n", ns.Name)
	return nil
}

func (o *MustGatherOptions) newPod(node, ns string) *corev1.Pod {
	zero := int64(0)
	ret := &corev1.Pod{
		Spec: corev1.PodSpec{
			NodeName:      node,
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:    "must-gather",
					Image:   o.Image,
					Command: []string{"/bin/bash", "-xec"},
				},
			},
			TerminationGracePeriodSeconds: &zero,
		},
	}

	return ret
}

func (o *MustGatherOptions) newKubeConfigSecret(ns string) (*corev1.Secret, error) {
	rawKubeConfig, err := o.KubeConfig.RawConfig()
	if err != nil {
		return nil, err
	}
	if err := clientcmdapi.MinifyConfig(&rawKubeConfig); err != nil {
		return nil, err
	}
	if err := clientcmdapi.FlattenConfig(&rawKubeConfig); err != nil {
		return nil, err
	}
	externalKubeConfig, err := latest.Scheme.ConvertToVersion(&rawKubeConfig, latest.ExternalVersion)
	if err != nil {
		return nil, err
	}
	kubeConfigBytes, err := runtime.Encode(latest.Codec, externalKubeConfig)
	if err != nil {
		return nil, err
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "kubeconfig"},
		Data: map[string][]byte{
			"kubeconfig": kubeConfigBytes,
		},
	}, nil
}

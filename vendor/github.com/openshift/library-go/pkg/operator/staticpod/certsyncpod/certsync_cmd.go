package certsyncpod

import (
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/openshift/library-go/pkg/config/client"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/revision"
)

type CertSyncControllerOptions struct {
	KubeConfigFile string
	Namespace      string
	DestinationDir string

	configMaps []revision.RevisionResource
	secrets    []revision.RevisionResource
}

func NewCertSyncControllerCommand(configmaps, secrets []revision.RevisionResource) *cobra.Command {
	o := &CertSyncControllerOptions{
		configMaps: configmaps,
		secrets:    secrets,
	}

	cmd := &cobra.Command{
		Use: "cert-syncer --kubeconfig=kubeconfigfile",
		Run: func(cmd *cobra.Command, args []string) {
			r, err := o.Complete()
			if err != nil {
				glog.Fatal(err)
			}
			r.Run(1, make(chan struct{}))
		},
	}

	cmd.Flags().StringVar(&o.DestinationDir, "destination-dir", o.DestinationDir, "Directory to write to")
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", o.Namespace, "Namespace to read from")
	cmd.Flags().StringVar(&o.KubeConfigFile, "kubeconfig", o.KubeConfigFile, "Location of the master configuration file to run from.")

	return cmd
}

func (o *CertSyncControllerOptions) Complete() (*CertSyncController, error) {
	kubeConfig, err := client.GetKubeConfigOrInClusterConfig(o.KubeConfigFile, nil)
	if err != nil {
		return nil, err
	}
	protoKubeConfig := rest.CopyConfig(kubeConfig)
	protoKubeConfig.AcceptContentTypes = "application/vnd.kubernetes.protobuf,application/json"
	protoKubeConfig.ContentType = "application/vnd.kubernetes.protobuf"

	// This kube client use protobuf, do not use it for CR
	kubeClient, err := kubernetes.NewForConfig(protoKubeConfig)
	if err != nil {
		return nil, err
	}
	kubeInformers := informers.NewSharedInformerFactoryWithOptions(kubeClient, 10*time.Minute, informers.WithNamespace(o.Namespace))

	eventRecorder := events.NewKubeRecorder(kubeClient.CoreV1().Events(o.Namespace), "cert-syncer",
		&corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Pod",
			Namespace:  os.Getenv("POD_NAMESPACE"),
			Name:       os.Getenv("POD_NAME"),
		})

	return NewCertSyncController(
		o.DestinationDir,
		o.Namespace,
		o.configMaps,
		o.secrets,
		kubeInformers,
		eventRecorder,
	)
}

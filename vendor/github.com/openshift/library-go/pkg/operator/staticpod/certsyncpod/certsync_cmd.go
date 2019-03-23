package certsyncpod

import (
	"io/ioutil"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/openshift/library-go/pkg/config/client"
	"github.com/openshift/library-go/pkg/controller/fileobserver"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/revision"
)

type CertSyncControllerOptions struct {
	KubeConfigFile string
	Namespace      string
	DestinationDir string

	configMaps []revision.RevisionResource
	secrets    []revision.RevisionResource

	kubeClient kubernetes.Interface
}

func NewCertSyncControllerCommand(configmaps, secrets []revision.RevisionResource) *cobra.Command {
	o := &CertSyncControllerOptions{
		configMaps: configmaps,
		secrets:    secrets,
	}

	cmd := &cobra.Command{
		Use: "cert-syncer --kubeconfig=kubeconfigfile",
		Run: func(cmd *cobra.Command, args []string) {
			if err := o.Complete(); err != nil {
				glog.Fatal(err)
			}
			if err := o.Run(); err != nil {
				glog.Fatal(err)
			}
		},
	}

	cmd.Flags().StringVar(&o.DestinationDir, "destination-dir", o.DestinationDir, "Directory to write to")
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", o.Namespace, "Namespace to read from")
	cmd.Flags().StringVar(&o.KubeConfigFile, "kubeconfig", o.KubeConfigFile, "Location of the master configuration file to run from.")

	return cmd
}

func (o *CertSyncControllerOptions) Run() error {
	// When the kubeconfig content change, commit suicide to reload its content.
	observer, err := fileobserver.NewObserver(500 * time.Millisecond)
	if err != nil {
		return err
	}

	initialContent, _ := ioutil.ReadFile(o.KubeConfigFile)
	observer.AddReactor(fileobserver.ExitOnChangeReactor, map[string][]byte{o.KubeConfigFile: initialContent}, o.KubeConfigFile)

	stopCh := make(chan struct{})
	go observer.Run(stopCh)

	kubeInformers := informers.NewSharedInformerFactoryWithOptions(o.kubeClient, 10*time.Minute, informers.WithNamespace(o.Namespace))
	go kubeInformers.Start(stopCh)

	eventRecorder := events.NewKubeRecorder(o.kubeClient.CoreV1().Events(o.Namespace), "cert-syncer",
		&corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Pod",
			Namespace:  os.Getenv("POD_NAMESPACE"),
			Name:       os.Getenv("POD_NAME"),
		})

	controller, err := NewCertSyncController(
		o.DestinationDir,
		o.Namespace,
		o.configMaps,
		o.secrets,
		kubeInformers,
		eventRecorder,
	)
	if err != nil {
		return err
	}
	go controller.Run(1, stopCh)

	<-stopCh
	glog.Infof("Shutting down certificate syncer")

	return nil
}

func (o *CertSyncControllerOptions) Complete() error {
	kubeConfig, err := client.GetKubeConfigOrInClusterConfig(o.KubeConfigFile, nil)
	if err != nil {
		return err
	}

	protoKubeConfig := rest.CopyConfig(kubeConfig)
	protoKubeConfig.AcceptContentTypes = "application/vnd.kubernetes.protobuf,application/json"
	protoKubeConfig.ContentType = "application/vnd.kubernetes.protobuf"

	// This kube client use protobuf, do not use it for CR
	kubeClient, err := kubernetes.NewForConfig(protoKubeConfig)
	if err != nil {
		return err
	}
	o.kubeClient = kubeClient

	return nil
}

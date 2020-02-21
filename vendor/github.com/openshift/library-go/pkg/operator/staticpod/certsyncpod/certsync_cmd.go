package certsyncpod

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/klog"

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

	kubeClient            kubernetes.Interface
	tlsServerNameOverride string
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
				klog.Fatal(err)
			}
			if err := o.Run(); err != nil {
				klog.Fatal(err)
			}
		},
	}

	cmd.Flags().StringVar(&o.DestinationDir, "destination-dir", o.DestinationDir, "Directory to write to")
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", o.Namespace, "Namespace to read from (default to 'POD_NAMESPACE' environment variable)")
	cmd.Flags().StringVar(&o.KubeConfigFile, "kubeconfig", o.KubeConfigFile, "Location of the master configuration file to run from.")
	cmd.Flags().StringVar(&o.tlsServerNameOverride, "tls-server-name-override", o.tlsServerNameOverride, "Server name override used by TLS to negotiate the serving cert via SNI.")

	return cmd
}

func (o *CertSyncControllerOptions) Run() error {
	// When the kubeconfig content change, commit suicide to reload its content.
	observer, err := fileobserver.NewObserver(500 * time.Millisecond)
	if err != nil {
		return err
	}

	stopCh := make(chan struct{})

	// Make a context that is cancelled when stopCh is closed
	// TODO: Replace stopCh with regular context.
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		<-stopCh
	}()

	initialContent, _ := ioutil.ReadFile(o.KubeConfigFile)
	observer.AddReactor(fileobserver.TerminateOnChangeReactor(func() {
		close(stopCh)
	}), map[string][]byte{o.KubeConfigFile: initialContent}, o.KubeConfigFile)

	kubeInformers := informers.NewSharedInformerFactoryWithOptions(o.kubeClient, 10*time.Minute, informers.WithNamespace(o.Namespace))

	eventRecorder := events.NewKubeRecorder(o.kubeClient.CoreV1().Events(o.Namespace), "cert-syncer",
		&corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Pod",
			Namespace:  os.Getenv("POD_NAMESPACE"),
			Name:       os.Getenv("POD_NAME"),
		})

	controller := NewCertSyncController(
		o.DestinationDir,
		o.Namespace,
		o.configMaps,
		o.secrets,
		o.kubeClient,
		kubeInformers,
		eventRecorder,
	)

	// start everything. WithInformers start after they have been requested.
	go controller.Run(ctx, 1)
	go observer.Run(stopCh)
	go kubeInformers.Start(stopCh)

	<-stopCh
	klog.Infof("Shutting down certificate syncer")

	return nil
}

func (o *CertSyncControllerOptions) Complete() error {
	kubeConfig, err := client.GetKubeConfigOrInClusterConfig(o.KubeConfigFile, nil)
	if err != nil {
		return err
	}

	if len(o.Namespace) == 0 && len(os.Getenv("POD_NAMESPACE")) > 0 {
		o.Namespace = os.Getenv("POD_NAMESPACE")
	}

	protoKubeConfig := rest.CopyConfig(kubeConfig)
	protoKubeConfig.AcceptContentTypes = "application/vnd.kubernetes.protobuf,application/json"
	protoKubeConfig.ContentType = "application/vnd.kubernetes.protobuf"

	if len(o.tlsServerNameOverride) > 0 {
		protoKubeConfig.TLSClientConfig.ServerName = o.tlsServerNameOverride
	}

	// This kube client use protobuf, do not use it for CR
	kubeClient, err := kubernetes.NewForConfig(protoKubeConfig)
	if err != nil {
		return err
	}
	o.kubeClient = kubeClient

	return nil
}

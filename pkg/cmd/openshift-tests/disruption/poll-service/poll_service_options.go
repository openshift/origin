package poll_service

import (
	"context"
	"io"
	"math/rand"
	"os"

	"github.com/openshift/origin/pkg/clioptions/iooptions"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/sirupsen/logrus"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
)

type PollServiceOptions struct {
	KubeClient kubernetes.Interface
	Namespace  string
	ClusterIP  string
	Port       uint16

	BackendPrefix     string
	OutputFile        string
	MyNodeName        string
	StopConfigMapName string

	OriginalOutFile io.Writer
	CloseFn         iooptions.CloseFunc
	genericclioptions.IOStreams
}

func (o *PollServiceOptions) Run(ctx context.Context) error {
	logInst := logrus.New()
	logInst.SetOutput(o.IOStreams.Out)
	logger := logInst.WithFields(logrus.Fields{
		"backendPrefix": o.BackendPrefix,
		"node":          o.MyNodeName,
		"namespace":     o.Namespace,
		"clusterIP":     o.ClusterIP,
		"port":          o.Port,
		"uid":           rand.Intn(100000000),
	})

	logger.Infof("Initializing to watch clusterIP %s:%d", o.ClusterIP, o.Port)

	logger.Info("reading startingContent from %s", o.OutputFile)
	startingContent, err := os.ReadFile(o.OutputFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(startingContent) > 0 {
		logger.Info("replaying startingContent")
		//print starting content to the log so that we can simply scrape the log to find all entries at the end
		o.OriginalOutFile.Write(startingContent)
		logger.Info("done replaying startingContent")
	}

	recorder := monitor.WrapWithJSONLRecorder(monitor.NewRecorder(), o.IOStreams.Out, nil)

	kubeInformers := informers.NewSharedInformerFactory(o.KubeClient, 0)
	namespacedScopedCoreInformers := coreinformers.New(kubeInformers, o.Namespace, nil)

	cleanupFinished := make(chan struct{})
	podToServiceChecker := NewPollServiceWatcher(
		o.BackendPrefix,
		o.MyNodeName,
		o.Namespace,
		o.ClusterIP,
		o.Port,
		recorder,
		o.IOStreams.Out,
		o.StopConfigMapName,
		namespacedScopedCoreInformers.ConfigMaps(),
		logger,
	)

	go podToServiceChecker.Run(ctx, cleanupFinished)
	go kubeInformers.Start(ctx.Done())

	logger.Info("Watching configmaps...")

	<-ctx.Done()

	// now wait for the watchers to shutdown
	logger.Info("Waiting for watchers to close...")
	// TODO add time interrupt too
	<-cleanupFinished
	logger.Info("Exiting...")

	return nil
}

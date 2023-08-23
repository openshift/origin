package poll_service

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/openshift/origin/pkg/clioptions/iooptions"
	"github.com/openshift/origin/pkg/monitor"
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
	fmt.Fprintf(o.Out, "Initializing to watch clusterIP %s:%d\n", o.ClusterIP, o.Port)

	startingContent, err := os.ReadFile(o.OutputFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(startingContent) > 0 {
		//print starting content to the log so that we can simply scrape the log to find all entries at the end
		o.OriginalOutFile.Write(startingContent)
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
	)

	go podToServiceChecker.Run(ctx, cleanupFinished)
	go kubeInformers.Start(ctx.Done())

	fmt.Fprintf(o.Out, "Watching configmaps...\n")

	<-ctx.Done()

	// now wait for the watchers to shutdown
	fmt.Fprintf(o.Out, "Waiting for watchers to close...\n")
	// TODO add time interrupt too
	<-cleanupFinished
	fmt.Fprintf(o.Out, "Exiting...\n")

	return nil
}

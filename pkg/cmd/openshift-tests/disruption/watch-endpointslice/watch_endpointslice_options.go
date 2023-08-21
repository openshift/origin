package watch_endpointslice

import (
	"context"
	"fmt"
	"io"
	"os"

	coreinformers "k8s.io/client-go/informers/core/v1"

	"github.com/openshift/origin/pkg/clioptions/iooptions"
	"github.com/openshift/origin/pkg/monitor"
	"k8s.io/client-go/informers"

	discoveryinformers "k8s.io/client-go/informers/discovery/v1"

	"k8s.io/client-go/kubernetes"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type WatchEndpointSliceOptions struct {
	KubeClient kubernetes.Interface
	Namespace  string

	OutputFile         string
	BackendPrefix      string
	ServiceName        string
	MyNodeName         string
	Scheme             string
	Path               string
	ExpectedStatusCode int
	StopConfigMapName  string

	OriginalOutFile io.Writer
	CloseFn         iooptions.CloseFunc
	genericclioptions.IOStreams
}

func (o *WatchEndpointSliceOptions) Run(ctx context.Context) error {
	fmt.Fprintf(o.Out, "Initializing to watch -n %v service/%v\n", o.Namespace, o.ServiceName)

	startingContent, err := os.ReadFile(o.OutputFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(startingContent) > 0 {
		// print starting content to the log so that we can simply scrape the log to find all entries at the end.
		o.OriginalOutFile.Write(startingContent)
	}

	recorder := monitor.WrapWithJSONLRecorder(monitor.NewRecorder(), o.IOStreams.Out, nil)

	kubeInformers := informers.NewSharedInformerFactory(o.KubeClient, 0)
	namespaceScopedEndpointSliceInformers := discoveryinformers.New(kubeInformers, o.Namespace, nil)
	namespaceScopedCoreInformers := coreinformers.New(kubeInformers, o.Namespace, nil)

	cleanupFinished := make(chan struct{})
	podToPodChecker := NewEndpointWatcher(
		o.BackendPrefix,
		o.Namespace,
		o.ServiceName,
		o.StopConfigMapName,
		o.MyNodeName,
		o.Scheme,
		o.Path,
		o.ExpectedStatusCode,
		recorder,
		o.IOStreams.Out,
		namespaceScopedEndpointSliceInformers.EndpointSlices(),
		namespaceScopedCoreInformers.ConfigMaps(),
	)
	go podToPodChecker.Run(ctx, cleanupFinished)

	go kubeInformers.Start(ctx.Done())

	fmt.Fprintf(o.Out, "Watching endpoints....\n")

	<-ctx.Done()

	// now wait for the watchers to shut down
	fmt.Fprintf(o.Out, "Waiting for watchers to close....\n")
	// TODO add time interrupt too
	<-cleanupFinished
	fmt.Fprintf(o.Out, "Exiting....\n")

	return nil
}

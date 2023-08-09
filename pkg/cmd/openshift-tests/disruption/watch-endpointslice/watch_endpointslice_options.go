package watch_endpointslice

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

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

	OutputFile    string
	BackendPrefix string
	ServiceName   string

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

	podToPodChecker := NewEndpointWatcher(
		o.BackendPrefix,
		o.ServiceName,
		recorder,
		namespaceScopedEndpointSliceInformers.EndpointSlices())
	go podToPodChecker.Run(ctx)

	go kubeInformers.Start(ctx.Done())

	fmt.Fprintf(o.Out, "Watching endpoints....\n")

	<-ctx.Done()

	// now wait for the watchers to shut down
	fmt.Fprintf(o.Out, "Waiting 10s for watchers to close....\n")
	time.Sleep(10 * time.Second)
	fmt.Fprintf(o.Out, "Exiting....\n")

	return nil
}

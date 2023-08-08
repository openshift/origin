package watch_endpointslice

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/openshift/origin/pkg/monitor"
	"k8s.io/client-go/informers"

	discoveryinformers "k8s.io/client-go/informers/discovery/v1"

	"k8s.io/client-go/kubernetes"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type WatchEndpointSliceOptions struct {
	KubeClient kubernetes.Interface
	Namespace  string

	RecordedDisruptionFile string

	outFile io.WriteCloser

	genericclioptions.IOStreams
}

func (o *WatchEndpointSliceOptions) Run(ctx context.Context) error {
	startingContent, err := os.ReadFile(o.RecordedDisruptionFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(startingContent) > 0 {
		// print starting content to the log so that we can simply scrape the log to find all entries at the end.
		fmt.Fprint(o.Out, startingContent)
	}
	outFile, err := os.OpenFile(o.RecordedDisruptionFile, os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer outFile.Close()
	o.outFile = outFile

	// TODO make a recorder impl that records to an io.Writer
	recorder := monitor.NewRecorder()

	kubeInformers := informers.NewSharedInformerFactory(o.KubeClient, 0)
	namespaceScopedEndpointSliceInformers := discoveryinformers.New(kubeInformers, o.Namespace, nil)

	podToPodChecker := NewEndpointWatcher(
		"pod-to-pod-",
		"pod-to-pod-target",
		recorder,
		namespaceScopedEndpointSliceInformers.EndpointSlices())
	go podToPodChecker.Run(ctx)

	podToHostChecker := NewEndpointWatcher(
		"pod-to-host-",
		"pod-to-host-target",
		recorder,
		namespaceScopedEndpointSliceInformers.EndpointSlices())
	go podToHostChecker.Run(ctx)

	go kubeInformers.Start(ctx.Done())

	<-ctx.Done()

	// now wait for the watchers to shut down
	fmt.Fprintf(o.Out, "Waiting 70s for watchers to close....")
	time.Sleep(70 * time.Second)
	fmt.Fprintf(o.Out, "Exiting....")

	return nil
}

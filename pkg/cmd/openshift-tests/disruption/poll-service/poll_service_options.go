package poll_service

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/openshift/origin/pkg/clioptions/iooptions"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type PollServiceOptions struct {
	OutputFile string
	ClusterIP  string
	Port       string

	OriginalOutFile io.Writer
	CloseFn         iooptions.CloseFunc

	genericclioptions.IOStreams
}

func (o *PollServiceOptions) Run(ctx context.Context) error {
	fmt.Fprintf(o.Out, "Initializing to watch clusterIP %s:%s\n", o.ClusterIP, o.Port)

	startingContent, err := os.ReadFile(o.OutputFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(startingContent) > 0 {
		// print starting content to the log so that we can simply scrape the log to find all entries at the end
		o.OriginalOutFile.Write(startingContent)
	}

	//
	//recorder := monitor.WrapWithJSONLRecorder(monitor.NewRecorder(), o.IOStreams.Out, nil)

	cleanupFinished := make(chan struct{})
	servicePoller := NewServicePoller(o.ClusterIP, o.Port, o.IOStreams.Out)

	/*
		something that watches the endpoint

		//no need to start the informers
	*/
	go servicePoller.Run(ctx, cleanupFinished)
	fmt.Fprintf(o.Out, "Watching Service...\n")

	<-ctx.Done()

	fmt.Fprintf(o.Out, "Waiting to shutdown")

	<-cleanupFinished
	fmt.Fprintf(o.Out, "Exiting...\n")
	//fmt.Printf("YEAHHHH\n")
	//fmt.Printf("HMMMM %s\n", os.Getenv(ServiceClusterIPENV))
	return nil
}

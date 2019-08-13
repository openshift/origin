package recorder

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

type Options struct {
	RepositoryPath string
	Out, ErrOut    io.Writer
}

func (o Options) Validate() error {
	if len(o.RepositoryPath) == 0 {
		return fmt.Errorf("repository path must be specified")
	}
	return nil
}

func (o Options) Run() error {
	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()
	abortCh := make(chan os.Signal)
	go func() {
		<-abortCh
		fmt.Fprintf(o.ErrOut, "Interrupted, terminating\n")
		cancelFn()
		sig := <-abortCh
		fmt.Fprintf(o.ErrOut, "Interrupted twice, exiting (%s)\n", sig)
		switch sig {
		case syscall.SIGINT:
			os.Exit(130)
		default:
			os.Exit(0)
		}
	}()
	signal.Notify(abortCh, syscall.SIGINT, syscall.SIGTERM)

	err := o.Start(ctx)
	if err != nil {
		return err
	}

	<-ctx.Done()
	return nil
}

func (o Options) Start(ctx context.Context) error {
	cfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{})
	clusterConfig, err := cfg.ClientConfig()
	if err != nil {
		return fmt.Errorf("could not load client configuration: %v", err)
	}
	dynamicClient, err := dynamic.NewForConfig(clusterConfig)
	if err != nil {
		return err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(clusterConfig)
	if err != nil {
		return err
	}
	kubeClient, err := apiextensionsclient.NewForConfig(clusterConfig)
	if err != nil {
		return err
	}

	gitStore, err := NewGitStorage(o.RepositoryPath)
	if err != nil {
		return err
	}
	historyRecorder, err := New(dynamicClient, kubeClient, discoveryClient, gitStore)
	if err != nil {
		return err
	}

	historyRecorder.AddMonitoredCustomResourceGroup(schema.GroupVersion{
		Group:   "config.openshift.io", // Track everything under *.config.openshift.io
		Version: "v1",
	})

	go historyRecorder.Run(ctx.Done())

	return nil
}

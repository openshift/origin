package operator

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/openshift/origin/pkg/resourcewatch/controller/configmonitor"
	"github.com/openshift/origin/pkg/resourcewatch/storage"

	"github.com/openshift/origin/pkg/monitor"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/klog/v2"
)

// this doesn't appear to handle restarts cleanly.  To do so it would need to compare the resource version that it is applying
// to the resource version present and it would need to handle unobserved deletions properly.  both are possible, neither is easy.
func RunResourceWatch() error {
	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()
	abortCh := make(chan os.Signal, 2)
	go func() {
		<-abortCh
		klog.Errorf("Interrupted, terminating")
		cancelFn()
		sig := <-abortCh
		klog.Errorf("Interrupted twice, exiting (%s)\n", sig)
		switch sig {
		case syscall.SIGINT:
			os.Exit(130)
		default:
			os.Exit(0)
		}
	}()
	signal.Notify(abortCh, syscall.SIGINT, syscall.SIGTERM)

	kubeConfig, err := monitor.GetMonitorRESTConfig()
	if err != nil {
		klog.Errorf("Failed to get kubeconfig with error %v", err)
		return err
	}

	dynamicClient, err := dynamic.NewForConfig(kubeConfig)
	if err != nil {
		klog.Errorf("Failed to create dynamic client with error %v", err)
		return err
	}

	repositoryPath := "/repository"
	if repositoryPathEnv := os.Getenv("REPOSITORY_PATH"); len(repositoryPathEnv) > 0 {
		repositoryPath = repositoryPathEnv
	}

	gitStorage, err := storage.NewGitStorage(repositoryPath)
	if err != nil {
		klog.Errorf("Failed to create git storage with error %v", err)
		return err
	}

	dynamicInformer := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)

	resourcesToWatch := []schema.GroupVersionResource{
		configResource("apiservers"),
		configResource("authentications"),
		configResource("builds"),
		configResource("clusteroperators"),
		configResource("clusterversions"),
		configResource("consoles"),
		configResource("dnses"),
		configResource("featuregates"),
		configResource("imagecontentpolicies"),
		configResource("images"),
		configResource("infrastructures"),
		configResource("ingresses"),
		configResource("networks"),
		configResource("nodes"),
		configResource("oauths"),
		configResource("operatorhubs"),
		configResource("projects"),
		configResource("proxies"),
		configResource("schedulers"),
		appResource("deployments"),
		appResource("daemonsets"),
		appResource("statefulsets"),
		appResource("replicasets"),
		resource("events.k8s.io", "v1", "events"),
		resource("policy", "v1", "poddisruptionbudgets"),
		coreResource("pods"),
		coreResource("nodes"),
		coreResource("replicationcontrollers"),
		coreResource("services"),
	}

	configmonitor.WireResourceInformersToGitRepo(
		dynamicInformer,
		gitStorage,
		resourcesToWatch,
	)

	dynamicInformer.Start(ctx.Done())

	klog.Infof("Started all informers")

	<-ctx.Done()

	return nil
}

func configResource(resource string) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "config.openshift.io",
		Version:  "v1",
		Resource: resource,
	}
}

func coreResource(resource string) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: resource,
	}
}

func resource(group, version, resource string) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}
}

func appResource(resource string) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: resource,
	}
}

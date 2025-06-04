package operator

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/openshift/origin/pkg/clioptions/clusterinfo"
	"github.com/openshift/origin/pkg/resourcewatch/controller/configmonitor"
	"github.com/openshift/origin/pkg/resourcewatch/storage"
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

	kubeConfig, err := clusterinfo.GetMonitorRESTConfig()
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
		// provide high level details of configuration that feeds operator behavior
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

		// operator resources provide low level details about how what operators are doing
		operatorResource("authentications"),
		operatorResource("cloudcredentials"),
		operatorResource("clustercsidrivers"),
		operatorResource("configs"),
		operatorResource("consoles"),
		operatorResource("csisnapshotcontrollers"),
		operatorResource("dnses"),
		operatorResource("etcds"),
		operatorResource("imagecontentsourcepolicies"),
		operatorResource("insightsoperators"),
		operatorResource("kubeapiservers"),
		operatorResource("kubecontrollermanagers"),
		operatorResource("kubeschedulers"),
		operatorResource("kubestorageversionmigrators"),
		operatorResource("networks"),
		operatorResource("openshiftapiservers"),
		operatorResource("openshiftcontrollermanagers"),
		operatorResource("servicecas"),
		operatorResource("storages"),

		// machine resources are required to reason about the happenings of nodes
		resource("machine.openshift.io", "v1", "controlplanemachinesets"),
		resource("machine.openshift.io", "v1beta1", "machinehealthchecks"),
		resource("machine.openshift.io", "v1beta1", "machines"),
		resource("machine.openshift.io", "v1beta1", "machinesets"),

		// describes the behavior of operand rollouts
		appResource("deployments"),
		appResource("daemonsets"),
		appResource("statefulsets"),
		appResource("replicasets"),

		// describe notable happenings
		resource("events.k8s.io", "v1", "events"),

		// describes the behavior of node drains
		resource("policy", "v1", "poddisruptionbudgets"),

		// describes the behavior of admission during the run
		resource("admissionregistration.k8s.io", "v1", "validatingadmissionpolicies"),
		resource("admissionregistration.k8s.io", "v1", "validatingadmissionpolicybindings"),

		// describes the behavior of aggregated apiservers
		resource("apiregistration.k8s.io", "v1", "apiservices"),

		// describes behavior of service endpoints
		resource("discovery.k8s.io", "v1", "endpointslices"),

		coreResource("pods"),
		coreResource("namespaces"),
		coreResource("nodes"),
		coreResource("replicationcontrollers"),
		coreResource("services"),
		coreResource("serviceaccounts"),
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

func operatorResource(resource string) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "operator.openshift.io",
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

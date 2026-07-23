package observe

import (
	"context"
	"os"
	"strconv"
	"sync"

	"github.com/go-logr/logr"
	"github.com/openshift/origin/pkg/clioptions/clusterinfo"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

const (
	EnvEnableResourceMonitorTests    = "ENABLE_RESOURCE_MONITOR_TESTS"
	EnvEnableResourceEventCollection = "ENABLE_RESOURCE_EVENT_COLLECTION"
)

func Source(log logr.Logger) (ObservationSource, error) {
	monitorEnabled, _ := strconv.ParseBool(os.Getenv(EnvEnableResourceMonitorTests))
	eventCollectionEnabled, _ := strconv.ParseBool(os.Getenv(EnvEnableResourceEventCollection))

	resourcesToWatch := resourcesToWatch(monitorEnabled, eventCollectionEnabled)
	if len(resourcesToWatch) == 0 {
		log.Info("Resource watch disabled: set ENABLE_RESOURCE_MONITOR_TESTS and/or ENABLE_RESOURCE_EVENT_COLLECTION to enable")
		return func(ctx context.Context, log logr.Logger, resourceC chan<- *ResourceObservation) chan struct{} {
			finished := make(chan struct{})
			close(finished)
			return finished
		}, nil
	}

	kubeConfig, err := clusterinfo.GetMonitorRESTConfig()
	if err != nil {
		log.Error(err, "Failed to get kubeconfig")
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(kubeConfig)
	if err != nil {
		klog.Errorf("Failed to create dynamic client with error %v", err)
		return nil, err
	}

	return func(ctx context.Context, log logr.Logger, resourceC chan<- *ResourceObservation) chan struct{} {
		finished := make(chan struct{})

		observers := sync.WaitGroup{}
		for _, resource := range resourcesToWatch {
			observers.Add(1)
			go func(resource schema.GroupVersionResource) {
				defer observers.Done()

				ObserveResource(ctx, log, dynamicClient, resource, resourceC)
			}(resource)
		}

		log.Info("Started all informers")

		go func() {
			observers.Wait()
			log.Info("All informers finished")
			close(finished)
		}()
		return finished
	}, nil
}

func resourcesToWatch(monitorEnabled, eventCollectionEnabled bool) []schema.GroupVersionResource {
	var resources []schema.GroupVersionResource
	if monitorEnabled {
		resources = append(resources, monitorResources()...)
	}
	if eventCollectionEnabled {
		resources = append(resources, eventResources()...)
	}
	return resources
}

func monitorResources() []schema.GroupVersionResource {
	return []schema.GroupVersionResource{
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

		resource("machine.openshift.io", "v1", "controlplanemachinesets"),
		resource("machine.openshift.io", "v1beta1", "machinehealthchecks"),
		resource("machine.openshift.io", "v1beta1", "machines"),
		resource("machine.openshift.io", "v1beta1", "machinesets"),

		resource("apiextensions.k8s.io", "v1", "customresourcedefinitions"),

		appResource("deployments"),
		appResource("daemonsets"),
		appResource("statefulsets"),
		appResource("replicasets"),

		resource("policy", "v1", "poddisruptionbudgets"),

		resource("admissionregistration.k8s.io", "v1", "validatingadmissionpolicies"),
		resource("admissionregistration.k8s.io", "v1", "validatingadmissionpolicybindings"),

		resource("apiregistration.k8s.io", "v1", "apiservices"),

		resource("discovery.k8s.io", "v1", "endpointslices"),

		coreResource("pods"),
		coreResource("namespaces"),
		coreResource("nodes"),
		coreResource("replicationcontrollers"),
		coreResource("services"),
		coreResource("serviceaccounts"),
	}
}

func eventResources() []schema.GroupVersionResource {
	return []schema.GroupVersionResource{
		resource("events.k8s.io", "v1", "events"),
	}
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

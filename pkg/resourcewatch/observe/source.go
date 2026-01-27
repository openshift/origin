package observe

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	"github.com/openshift/origin/pkg/clioptions/clusterinfo"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

func Source(log logr.Logger) (ObservationSource, error) {
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

		// describes the behavior of api changes rollouts
		resource("apiextensions.k8s.io", "v1", "customresourcedefinitions"),

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

		// storage objects
		coreResource("persistentvolumes"),
		coreResource("persistentvolumeclaims"),
		resource("groupsnapshot.storage.k8s.io", "v1beta2", "volumegroupsnapshots"),
		resource("groupsnapshot.storage.k8s.io", "v1beta2", "volumegroupsnapshotcontents"),
		resource("snapshot.storage.k8s.io", "v1", "volumesnapshots"),
		resource("snapshot.storage.k8s.io", "v1", "volumesnapshotcontents"),
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

		// Close the finished channel when all observers have exited.
		go func() {
			observers.Wait()
			log.Info("All informers finished")
			close(finished)
		}()
		return finished
	}, nil
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

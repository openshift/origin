package operator

import (
	"context"
	"os"

	"k8s.io/client-go/dynamic/dynamicinformer"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/openshift/library-go/pkg/controller/controllercmd"

	"github.com/openshift/origin/pkg/monitor/resourcewatch/controller/configmonitor"
	"github.com/openshift/origin/pkg/monitor/resourcewatch/storage"
)

// this doesn't appear to handle restarts cleanly.  To do so it would need to compare the resource version that it is applying
// to the resource version present and it would need to handle unobserved deletions properly.  both are possible, neither is easy.
func RunOperator(ctx context.Context, controllerCtx *controllercmd.ControllerContext) error {
	dynamicClient, err := dynamic.NewForConfig(controllerCtx.KubeConfig)
	if err != nil {
		return err
	}

	repositoryPath := "/repository"
	if repositoryPathEnv := os.Getenv("REPOSITORY_PATH"); len(repositoryPathEnv) > 0 {
		repositoryPath = repositoryPathEnv
	}

	gitStorage, err := storage.NewGitStorage(repositoryPath)
	if err != nil {
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
		resource("policy", "v1", "PodDisruptionBudget"),
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

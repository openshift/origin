package factory

import (
	"errors"
	"time"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/api"
	controller "github.com/openshift/origin/pkg/build/controller"
	strategy "github.com/openshift/origin/pkg/build/controller/strategy"
	osclient "github.com/openshift/origin/pkg/client"
)

type BuildControllerFactory struct {
	Client              *osclient.Client
	KubeClient          *kclient.Client
	DockerBuildStrategy *strategy.DockerBuildStrategy
	STIBuildStrategy    *strategy.STIBuildStrategy

	buildStore cache.Store
}

func (factory *BuildControllerFactory) Create() *controller.BuildController {
	factory.buildStore = cache.NewStore()
	cache.NewReflector(&buildLW{client: factory.Client}, &buildapi.Build{}, factory.buildStore).Run()

	buildQueue := cache.NewFIFO()
	cache.NewReflector(&buildLW{client: factory.Client}, &buildapi.Build{}, buildQueue).Run()

	// Kubernetes does not currently synchronize Pod status in storage with a Pod's container
	// states. Because of this, we can't receive events related to container (and thus Pod)
	// state changes, such as Running -> Terminated. As a workaround, populate the FIFO with
	// a polling implementation which relies on client calls to list Pods - the Get/List
	// REST implementations will populate the synchronized container/pod status on-demand.
	//
	// TODO: Find a way to get watch events for Pod/container status updates. The polling
	// strategy is horribly inefficient and should be addressed upstream somehow.
	podQueue := cache.NewFIFO()
	cache.NewPoller(factory.pollPods, 10*time.Second, podQueue).Run()

	return &controller.BuildController{
		BuildStore:   factory.buildStore,
		BuildUpdater: factory.Client,
		PodCreator:   ClientPodCreator{factory.KubeClient},
		NextBuild: func() *buildapi.Build {
			return buildQueue.Pop().(*buildapi.Build)
		},
		NextPod: func() *kapi.Pod {
			return podQueue.Pop().(*kapi.Pod)
		},
		BuildStrategy: &typeBasedFactoryStrategy{
			DockerBuildStrategy: factory.DockerBuildStrategy,
			STIBuildStrategy:    factory.STIBuildStrategy,
		},
	}
}

// pollPods lists pods for all builds in the buildStore which are pending or running and
// returns an enumerator for cache.Poller. The poll scope is narrowed for efficiency.
func (factory *BuildControllerFactory) pollPods() (cache.Enumerator, error) {
	list := &kapi.PodList{}

	for _, obj := range factory.buildStore.List() {
		build := obj.(*buildapi.Build)

		switch build.Status {
		case buildapi.BuildStatusPending, buildapi.BuildStatusRunning:
			pod, err := factory.KubeClient.Pods(build.Namespace).Get(build.PodName)
			if err != nil {
				glog.V(2).Infof("Couldn't find pod %s for build %s: %#v", build.PodName, build.Name, err)
				continue
			}
			list.Items = append(list.Items, *pod)
		}
	}

	return &podEnumerator{list}, nil
}

// podEnumerator allows a cache.Poller to enumerate items in an api.PodList
type podEnumerator struct {
	*kapi.PodList
}

// Len returns the number of items in the pod list.
func (pe *podEnumerator) Len() int {
	if pe.PodList == nil {
		return 0
	}
	return len(pe.Items)
}

// Get returns the item (and ID) with the particular index.
func (pe *podEnumerator) Get(index int) (string, interface{}) {
	return pe.Items[index].Name, &pe.Items[index]
}

type typeBasedFactoryStrategy struct {
	DockerBuildStrategy *strategy.DockerBuildStrategy
	STIBuildStrategy    *strategy.STIBuildStrategy
}

func (f *typeBasedFactoryStrategy) CreateBuildPod(build *buildapi.Build) (*kapi.Pod, error) {
	switch build.Parameters.Strategy.Type {
	case buildapi.DockerBuildStrategyType:
		return f.DockerBuildStrategy.CreateBuildPod(build)
	case buildapi.STIBuildStrategyType:
		return f.STIBuildStrategy.CreateBuildPod(build)
	default:
		return nil, errors.New("No strategy defined for type")
	}
}

// buildLW is a ListWatcher implementation for Builds.
type buildLW struct {
	client osclient.Interface
}

// List lists all Builds.
func (lw *buildLW) List() (runtime.Object, error) {
	return lw.client.ListBuilds(kapi.NewContext(), labels.Everything())
}

// Watch watches all Builds.
func (lw *buildLW) Watch(resourceVersion string) (watch.Interface, error) {
	return lw.client.WatchBuilds(kapi.NewContext(), labels.Everything(), labels.Everything(), "0")
}

// ClientPodCreator is a podCreator which delegates to the Kubernetes client interface.
type ClientPodCreator struct {
	KubeClient kclient.Interface
}

// CreatePod creates a pod using the Kubernetes client.
func (c ClientPodCreator) CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return c.KubeClient.Pods(namespace).Create(pod)
}

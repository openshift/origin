package configmonitor

import (
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/klog/v2"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

type resourceObserverEventHandler interface {
	OnAdd(gvr schema.GroupVersionResource, obj interface{})
	OnUpdate(gvr schema.GroupVersionResource, _, obj interface{})
	OnDelete(gvr schema.GroupVersionResource, obj interface{})
}

// this is an unusual controller. it really wants an pure watch stream, but that change is too big to reason about at
// the moment.  For the moment we'll allow it have synchronous handling of informer notifications.  This has severe consequences
// for cache correctness and latency, but it keeps me from having rip out more logic than I want to.
// It doesn't logically need to run because there is no sync method.  it's all handled by the gitStorage.
// if you ask for a resource that doesn't exist, it will simply repeated error until it appears while watching all the other types.
func WireResourceInformersToGitRepo(
	dynamicInformerFactory dynamicinformer.DynamicSharedInformerFactory,
	gitStorage resourceObserverEventHandler,
	resourcesToWatch []schema.GroupVersionResource,
) {
	for i := range resourcesToWatch {
		resourceToWatch := resourcesToWatch[i]
		// we got mapping, lets run the dynamicInformer for the config and install GIT storageHandler event handlers
		dynamicInformer := dynamicInformerFactory.ForResource(resourceToWatch).Informer()

		dynamicInformer.AddEventHandler(
			cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					gitStorage.OnAdd(resourceToWatch, obj)
				},
				UpdateFunc: func(oldObj, newObj interface{}) {
					gitStorage.OnUpdate(resourceToWatch, oldObj, newObj)
				},
				DeleteFunc: func(obj interface{}) {
					gitStorage.OnDelete(resourceToWatch, obj)
				},
			},
		)
		klog.Infof("Added event handler for resource %s", resourceToWatch.String())
	}
}

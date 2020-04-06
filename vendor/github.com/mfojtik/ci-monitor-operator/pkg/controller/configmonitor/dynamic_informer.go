package configmonitor

import (
	"context"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

type dynamicConfigInformer struct {
	informer  cache.SharedIndexInformer
	hasSynced cache.InformerSynced

	groupVersionResource schema.GroupVersionResource
	configKind           string
}

func newDynamicConfigInformer(kind string, configResource schema.GroupVersionResource, client dynamic.Interface, resourceHandlers ...cache.ResourceEventHandler) *dynamicConfigInformer {
	observer := &dynamicConfigInformer{
		informer:             dynamicinformer.NewDynamicSharedInformerFactory(client, defaultResyncDuration).ForResource(configResource).Informer(),
		configKind:           kind,
		groupVersionResource: configResource,
	}
	observer.hasSynced = observer.informer.HasSynced
	for _, handler := range resourceHandlers {
		observer.informer.AddEventHandler(handler)
	}
	return observer
}

func (c dynamicConfigInformer) isKind(kind schema.GroupVersionKind) bool {
	return schema.GroupVersionKind{
		Group:   c.groupVersionResource.Group,
		Version: c.groupVersionResource.Version,
		Kind:    c.configKind,
	} == kind
}

func (c *dynamicConfigInformer) run(ctx context.Context) {
	c.informer.Run(ctx.Done())
}

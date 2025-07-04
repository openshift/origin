package observe

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

type resourceMeta struct {
	resourceVersions map[string]struct{}
	lastObserved     *unstructured.Unstructured
}

// ObserveResource monitors a Kubernetes resource for changes
func ObserveResource(ctx context.Context, log logr.Logger, client *dynamic.DynamicClient, gvr schema.GroupVersionResource, resourceC chan<- *ResourceObservation) {
	log = log.WithName("ObserveResource").WithValues("group", gvr.Group, "version", gvr.Version, "resource", gvr.Resource)

	resourceClient := client.Resource(gvr)

	observedResources := make(map[types.UID]*resourceMeta)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := listAndWatchResource(ctx, log, resourceClient, gvr, observedResources, resourceC); err != nil {
			log.Error(err, "failed to list and watch resource")
		}
	}
}

func listAndWatchResource(ctx context.Context, log logr.Logger, client dynamic.NamespaceableResourceInterface, gvr schema.GroupVersionResource, observedResources map[types.UID]*resourceMeta, resourceC chan<- *ResourceObservation) error {
	listResourceVersion, err := listResource(ctx, log, client, gvr, observedResources, resourceC)
	if err != nil {
		// List returns a NotFound error if the resource doesn't exist. We
		// expect this to happen during cluster installation before CRDs are
		// admitted. Poll at 5 second intervals if this happens to avoid
		// spamming api-server or the logs.
		if apierrors.IsNotFound(err) {
			log.Info("Resource not found, polling")
			time.Sleep(5 * time.Second)
			return nil
		}
		return err
	}

	log.Info("Watching resource")

	resourceWatch, err := client.Watch(ctx, metav1.ListOptions{ResourceVersion: listResourceVersion})
	if err != nil {
		return fmt.Errorf("failed to watch resource: %w", err)
	}

	resultChan := resourceWatch.ResultChan()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case observation, ok := <-resultChan:
			if !ok {
				log.Info("Resource watch closed")
				return nil
			}

			object, ok := observation.Object.(*unstructured.Unstructured)
			if !ok {
				return fmt.Errorf("failed to cast observation object to unstructured: %T", observation.Object)
			}

			switch observation.Type {
			case watch.Added:
			case watch.Modified:
				emitUpdate(observedResources, gvr, object, resourceC)
			case watch.Deleted:
				emitDelete(observedResources, gvr, object, resourceC)
			default:
				log.Info("Unhandled watch event", "type", observation.Type)
			}
		}
	}
}

func listResource(ctx context.Context, log logr.Logger, client dynamic.NamespaceableResourceInterface, gvr schema.GroupVersionResource, observedResources map[types.UID]*resourceMeta, resourceC chan<- *ResourceObservation) (string, error) {
	log.Info("Listing resource")

	resourceList, err := client.List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list resource: %w", err)
	}

	listedUIDs := make(map[types.UID]struct{}, len(resourceList.Items))

	// Emit an Added observation for each resource in the listing
	for _, resource := range resourceList.Items {
		listedUIDs[resource.GetUID()] = struct{}{}
		emitUpdate(observedResources, gvr, &resource, resourceC)
	}

	// Infer that any known resource we haven't seen in the listing was deleted
	for uid, observedMeta := range observedResources {
		if _, ok := listedUIDs[uid]; !ok {
			emitInferredDelete(observedResources, observedMeta.lastObserved, gvr, uid, resourceC)
		}
	}

	return resourceList.GetResourceVersion(), nil
}

func emitUpdate(observedResources map[types.UID]*resourceMeta, gvr schema.GroupVersionResource, resource *unstructured.Unstructured, resourceC chan<- *ResourceObservation) {
	observationType := ObservationTypeUpdate

	observedMeta, ok := observedResources[resource.GetUID()]
	if ok {
		// Don't emit an update for a resource version we've already seen
		if _, ok := observedMeta.resourceVersions[resource.GetResourceVersion()]; ok {
			return
		}
	} else {
		observedMeta = &resourceMeta{
			resourceVersions: make(map[string]struct{}),
		}
		observedResources[resource.GetUID()] = observedMeta
		observationType = ObservationTypeAdd
	}

	resourceC <- &ResourceObservation{
		Group:           gvr.Group,
		Version:         gvr.Version,
		Resource:        gvr.Resource,
		UID:             resource.GetUID(),
		ObservationType: observationType,
		Object:          resource,
		OldObject:       observedMeta.lastObserved,
		ObservationTime: time.Now(),
	}

	observedMeta.resourceVersions[resource.GetResourceVersion()] = struct{}{}
	observedMeta.lastObserved = resource
}

func emitDelete(observedResources map[types.UID]*resourceMeta, gvr schema.GroupVersionResource, resource *unstructured.Unstructured, resourceC chan<- *ResourceObservation) {
	delete(observedResources, resource.GetUID())

	resourceC <- &ResourceObservation{
		Group:           gvr.Group,
		Version:         gvr.Version,
		Resource:        gvr.Resource,
		UID:             resource.GetUID(),
		ObservationType: ObservationTypeDelete,
		Object:          resource,
		ObservationTime: time.Now(),
	}
}

func emitInferredDelete(observedResources map[types.UID]*resourceMeta, lastObserved *unstructured.Unstructured, gvr schema.GroupVersionResource, uid types.UID, resourceC chan<- *ResourceObservation) {
	delete(observedResources, uid)

	// Copy a limited amount of data from the last observed object to a tombstone
	tombstone := unstructured.Unstructured{}
	tombstone.SetGroupVersionKind(lastObserved.GroupVersionKind())
	tombstone.SetName(lastObserved.GetName())
	tombstone.SetNamespace(lastObserved.GetNamespace())
	tombstone.SetUID(uid)

	resourceC <- &ResourceObservation{
		Group:           gvr.Group,
		Version:         gvr.Version,
		Resource:        gvr.Resource,
		UID:             uid,
		ObservationType: ObservationTypeDelete,
		ObservationTime: time.Now(),
		Object:          &tombstone,
	}
}

package observe

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
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

var (
	errWatchClosed      = errors.New("resource watch closed")
	errWatchErrorEvent  = errors.New("resource watch error event")
	errUnexpectedObject = errors.New("unexpected watch object type")
)

const (
	notFoundRetryDelay = 5 * time.Second
	minRetryDelay      = 500 * time.Millisecond
	maxRetryDelay      = 30 * time.Second
)

// ObserveResource monitors a Kubernetes resource for changes
func ObserveResource(ctx context.Context, log logr.Logger, client *dynamic.DynamicClient, gvr schema.GroupVersionResource, resourceC chan<- *ResourceObservation) {
	log = log.WithName("ObserveResource").WithValues("group", gvr.Group, "version", gvr.Version, "resource", gvr.Resource)

	resourceClient := client.Resource(gvr)

	observedResources := make(map[types.UID]*resourceMeta)
	retryAttempt := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := listAndWatchResource(ctx, log, resourceClient, gvr, observedResources, resourceC); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}

			retryDelay := nextRetryDelay(err, retryAttempt)
			log.Error(err, "failed to list and watch resource", "retryReason", retryReason(err), "retryDelay", retryDelay)

			if !waitForRetry(ctx, retryDelay) {
				return
			}

			retryAttempt++
			continue
		}

		// If a watch cycle ends cleanly, start retries from the base delay.
		retryAttempt = 0
	}
}

func nextRetryDelay(err error, retryAttempt int) time.Duration {
	if apierrors.IsNotFound(err) {
		return notFoundRetryDelay
	}

	backoff := minRetryDelay
	for i := 0; i < retryAttempt; i++ {
		if backoff >= maxRetryDelay/2 {
			backoff = maxRetryDelay
			break
		}
		backoff *= 2
	}
	if backoff > maxRetryDelay {
		backoff = maxRetryDelay
	}

	jitter := backoff / 4
	if jitter > 0 {
		jitterDelta := time.Duration(rand.Int63n(int64(2*jitter)+1)) - jitter
		backoff += jitterDelta
	}
	if backoff < minRetryDelay {
		backoff = minRetryDelay
	}
	if backoff > maxRetryDelay {
		backoff = maxRetryDelay
	}
	return backoff
}

func waitForRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func retryReason(err error) string {
	switch {
	case apierrors.IsNotFound(err):
		return "listNotFound"
	case errors.Is(err, errWatchClosed):
		return "watchClosed"
	case errors.Is(err, errWatchErrorEvent):
		return "watchError"
	case errors.Is(err, errUnexpectedObject):
		return "decodeError"
	default:
		return "listOrWatchError"
	}
}

func listAndWatchResource(ctx context.Context, log logr.Logger, client dynamic.NamespaceableResourceInterface, gvr schema.GroupVersionResource, observedResources map[types.UID]*resourceMeta, resourceC chan<- *ResourceObservation) error {
	listResourceVersion, err := listResource(ctx, log, client, gvr, observedResources, resourceC)
	if err != nil {
		// List returns a NotFound error if the resource doesn't exist. We
		// expect this to happen during cluster installation before CRDs are
		// admitted.
		if apierrors.IsNotFound(err) {
			log.Info("Resource not found")
		}
		return err
	}

	log.Info("Watching resource")

	resourceWatch, err := client.Watch(ctx, metav1.ListOptions{ResourceVersion: listResourceVersion})
	if err != nil {
		return fmt.Errorf("failed to watch resource: %w", err)
	}
	defer resourceWatch.Stop()

	resultChan := resourceWatch.ResultChan()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case observation, ok := <-resultChan:
			if !ok {
				return errWatchClosed
			}

			switch observation.Type {
			case watch.Bookmark:
				// Bookmarks are periodic progress notifications; no state change to emit.
				continue
			case watch.Error:
				status, ok := observation.Object.(*metav1.Status)
				if !ok {
					return fmt.Errorf("%w: %T", errWatchErrorEvent, observation.Object)
				}
				return fmt.Errorf("%w: reason=%s message=%s", errWatchErrorEvent, status.Reason, status.Message)
			case watch.Added, watch.Modified, watch.Deleted:
				// handled below
			default:
				log.Info("Unhandled watch event", "type", observation.Type)
				continue
			}

			object, ok := observation.Object.(*unstructured.Unstructured)
			if !ok {
				return fmt.Errorf("%w: %T", errUnexpectedObject, observation.Object)
			}

			switch observation.Type {
			case watch.Added, watch.Modified:
				emitUpdate(observedResources, gvr, object, resourceC)
			case watch.Deleted:
				emitDelete(observedResources, gvr, object, resourceC)
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

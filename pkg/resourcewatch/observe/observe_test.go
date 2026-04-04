package observe

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

type fakeNamespaceableResource struct {
	listFn  func(ctx context.Context, opts v1.ListOptions) (*unstructured.UnstructuredList, error)
	watchFn func(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
}

func (f *fakeNamespaceableResource) Namespace(string) dynamic.ResourceInterface {
	return f
}

func (f *fakeNamespaceableResource) Create(context.Context, *unstructured.Unstructured, v1.CreateOptions, ...string) (*unstructured.Unstructured, error) {
	panic("not implemented")
}

func (f *fakeNamespaceableResource) Update(context.Context, *unstructured.Unstructured, v1.UpdateOptions, ...string) (*unstructured.Unstructured, error) {
	panic("not implemented")
}

func (f *fakeNamespaceableResource) UpdateStatus(context.Context, *unstructured.Unstructured, v1.UpdateOptions) (*unstructured.Unstructured, error) {
	panic("not implemented")
}

func (f *fakeNamespaceableResource) Delete(context.Context, string, v1.DeleteOptions, ...string) error {
	panic("not implemented")
}

func (f *fakeNamespaceableResource) DeleteCollection(context.Context, v1.DeleteOptions, v1.ListOptions) error {
	panic("not implemented")
}

func (f *fakeNamespaceableResource) Get(context.Context, string, v1.GetOptions, ...string) (*unstructured.Unstructured, error) {
	panic("not implemented")
}

func (f *fakeNamespaceableResource) List(ctx context.Context, opts v1.ListOptions) (*unstructured.UnstructuredList, error) {
	return f.listFn(ctx, opts)
}

func (f *fakeNamespaceableResource) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return f.watchFn(ctx, opts)
}

func (f *fakeNamespaceableResource) Patch(context.Context, string, types.PatchType, []byte, v1.PatchOptions, ...string) (*unstructured.Unstructured, error) {
	panic("not implemented")
}

func (f *fakeNamespaceableResource) Apply(context.Context, string, *unstructured.Unstructured, v1.ApplyOptions, ...string) (*unstructured.Unstructured, error) {
	panic("not implemented")
}

func (f *fakeNamespaceableResource) ApplyStatus(context.Context, string, *unstructured.Unstructured, v1.ApplyOptions) (*unstructured.Unstructured, error) {
	panic("not implemented")
}

type trackingWatch struct {
	resultC chan watch.Event
	stopped bool
}

func newTrackingWatch() *trackingWatch {
	return &trackingWatch{
		resultC: make(chan watch.Event, 4),
	}
}

func (w *trackingWatch) Stop() {
	if w.stopped {
		return
	}
	w.stopped = true
	close(w.resultC)
}

func (w *trackingWatch) ResultChan() <-chan watch.Event {
	return w.resultC
}

func TestListAndWatchResource_StopsWatchAndHandlesErrorEvent(t *testing.T) {
	t.Parallel()

	resourceWatch := newTrackingWatch()
	resourceWatch.resultC <- watch.Event{
		Type: watch.Error,
		Object: &v1.Status{
			Reason:  v1.StatusReasonExpired,
			Message: "resource version too old",
		},
	}

	client := &fakeNamespaceableResource{
		listFn: func(context.Context, v1.ListOptions) (*unstructured.UnstructuredList, error) {
			return &unstructured.UnstructuredList{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"resourceVersion": "123",
					},
				},
			}, nil
		},
		watchFn: func(context.Context, v1.ListOptions) (watch.Interface, error) {
			return resourceWatch, nil
		},
	}

	err := listAndWatchResource(context.Background(), klog.NewKlogr(), client, schema.GroupVersionResource{Resource: "pods"}, map[types.UID]*resourceMeta{}, make(chan *ResourceObservation, 8))
	if !errors.Is(err, errWatchErrorEvent) {
		t.Fatalf("expected watch error event, got: %v", err)
	}
	if !strings.Contains(err.Error(), "resource version too old") {
		t.Fatalf("expected status message in error, got: %v", err)
	}
	if !resourceWatch.stopped {
		t.Fatalf("expected watch.Stop() to be called")
	}
}

func TestNextRetryDelay_BackoffAndNotFound(t *testing.T) {
	t.Parallel()

	notFoundWrapped := apierrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "deployments"}, "example")
	if got := nextRetryDelay(notFoundWrapped, 5); got != notFoundRetryDelay {
		t.Fatalf("expected not found retry delay %v, got %v", notFoundRetryDelay, got)
	}

	first := nextRetryDelay(errors.New("watch failed"), 0)
	if first < minRetryDelay || first > minRetryDelay+minRetryDelay/2 {
		t.Fatalf("attempt 0 delay out of range: %v", first)
	}

	second := nextRetryDelay(errors.New("watch failed"), 1)
	if second < minRetryDelay*2-minRetryDelay/2 || second > minRetryDelay*2+minRetryDelay/2 {
		t.Fatalf("attempt 1 delay out of range: %v", second)
	}

	maxed := nextRetryDelay(errors.New("watch failed"), 50)
	if maxed < minRetryDelay || maxed > maxRetryDelay {
		t.Fatalf("attempt 50 delay out of range [%v, %v]: %v", minRetryDelay, maxRetryDelay, maxed)
	}
}

func TestListAndWatchResource_AddedEventEmitsObservation(t *testing.T) {
	t.Parallel()

	resourceWatch := newTrackingWatch()
	resourceWatch.resultC <- watch.Event{
		Type: watch.Added,
		Object: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"name":            "new-pod",
					"uid":             "uid-1234",
					"resourceVersion": "456",
				},
			},
		},
	}

	client := &fakeNamespaceableResource{
		listFn: func(context.Context, v1.ListOptions) (*unstructured.UnstructuredList, error) {
			return &unstructured.UnstructuredList{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"resourceVersion": "100",
					},
				},
			}, nil
		},
		watchFn: func(context.Context, v1.ListOptions) (watch.Interface, error) {
			return resourceWatch, nil
		},
	}

	resourceC := make(chan *ResourceObservation, 8)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case obs := <-resourceC:
				if obs.ObservationType == ObservationTypeAdd && string(obs.UID) == "uid-1234" {
					cancel()
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	gvr := schema.GroupVersionResource{Resource: "pods"}
	_ = listAndWatchResource(ctx, klog.NewKlogr(), client, gvr, map[types.UID]*resourceMeta{}, resourceC)

	if ctx.Err() == nil {
		t.Fatalf("expected context to be cancelled after receiving Added observation")
	}
	if !resourceWatch.stopped {
		t.Fatalf("expected watch.Stop() to be called")
	}
}

func TestWaitForRetry_CancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	ok := waitForRetry(ctx, 2*time.Second)
	if ok {
		t.Fatalf("expected waitForRetry to abort on cancelled context")
	}
	if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
		t.Fatalf("expected prompt cancellation, elapsed=%v", elapsed)
	}
}

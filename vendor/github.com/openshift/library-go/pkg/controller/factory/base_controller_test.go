package factory

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"k8s.io/client-go/tools/cache"

	"github.com/openshift/library-go/pkg/operator/events/eventstesting"
)

type fakeInformer struct {
	hasSyncedDelay       time.Duration
	eventHandler         cache.ResourceEventHandler
	addEventHandlerCount int
	hasSyncedCount       int
	sync.Mutex
}

func (f *fakeInformer) AddEventHandler(handler cache.ResourceEventHandler) {
	f.Lock()
	defer func() { f.addEventHandlerCount++; f.Unlock() }()
	f.eventHandler = handler
}

func (f *fakeInformer) HasSynced() bool {
	f.Lock()
	defer func() { f.hasSyncedCount++; f.Unlock() }()
	time.Sleep(f.hasSyncedDelay)
	return true
}

func TestBaseController_Run(t *testing.T) {
	informer := &fakeInformer{hasSyncedDelay: 200 * time.Millisecond}
	controllerCtx, cancel := context.WithCancel(context.Background())
	syncCount := 0
	postStartHookSyncCount := 0
	postStartHookDone := false

	c := &baseController{
		name:         "test",
		cachesToSync: []cache.InformerSynced{informer.HasSynced},
		sync: func(ctx context.Context, syncCtx SyncContext) error {
			defer func() { syncCount++ }()
			defer t.Logf("Sync() call with %q", syncCtx.QueueKey())
			if syncCtx.QueueKey() == "postStartHookKey" {
				postStartHookSyncCount++
				return fmt.Errorf("test error")
			}
			return nil
		},
		syncContext: NewSyncContext("test", eventstesting.NewTestingEventRecorder(t)),
		resyncEvery: 200 * time.Millisecond,
		postStartHooks: []PostStartHook{func(ctx context.Context, syncContext SyncContext) error {
			defer func() {
				postStartHookDone = true
			}()
			syncContext.Queue().Add("postStartHookKey")
			<-ctx.Done()
			t.Logf("post start hook terminated")
			return nil
		}},
	}

	time.AfterFunc(1*time.Second, func() {
		cancel()
	})
	c.Run(controllerCtx, 1)

	informer.Lock()
	if informer.hasSyncedCount == 0 {
		t.Errorf("expected HasSynced() called at least once, got 0")
	}
	informer.Unlock()
	if syncCount == 0 {
		t.Errorf("expected at least one sync call, got 0")
	}
	if postStartHookSyncCount == 0 {
		t.Errorf("expected the post start hook queue key, got none")
	}
	if !postStartHookDone {
		t.Errorf("expected the post start hook to be terminated when context is cancelled")
	}
}

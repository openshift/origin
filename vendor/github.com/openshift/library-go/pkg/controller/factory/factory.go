package factory

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"

	"github.com/openshift/library-go/pkg/operator/events"
)

// DefaultQueueKey is the queue key used for string trigger based controllers.
const DefaultQueueKey = "key"

// Factory is generator that generate standard Kubernetes controllers.
// Factory is really generic and should be only used for simple controllers that does not require special stuff..
type Factory struct {
	sync                  SyncFunc
	syncContext           SyncContext
	resyncInterval        time.Duration
	informers             []Informer
	informerQueueKeys     []informersWithQueueKey
	postStartHooks        []PostStartHook
	namespaceInformers    []*namespaceInformer
	cachesToSync          []cache.InformerSynced
	interestingNamespaces sets.String
}

// Informer represents any structure that allow to register event handlers and informs if caches are synced.
// Any SharedInformer will comply.
type Informer interface {
	AddEventHandler(handler cache.ResourceEventHandler)
	HasSynced() bool
}

type namespaceInformer struct {
	informer   Informer
	namespaces sets.String
}

type informersWithQueueKey struct {
	informers  []Informer
	queueKeyFn ObjectQueueKeyFunc
}

// PostStartHook specify a function that will run after controller is started.
// The context is cancelled when the controller is asked to shutdown and the post start hook should terminate as well.
// The syncContext allow access to controller queue and event recorder.
type PostStartHook func(ctx context.Context, syncContext SyncContext) error

// ObjectQueueKeyFunc is used to make a string work queue key out of the runtime object that is passed to it.
// This can extract the "namespace/name" if you need to or just return "key" if you building controller that only use string
// triggers.
type ObjectQueueKeyFunc func(runtime.Object) string

// New return new factory instance.
func New() *Factory {
	return &Factory{}
}

// Sync is used to set the controller synchronization function. This function is the core of the controller and is
// usually hold the main controller logic.
func (f *Factory) WithSync(syncFn SyncFunc) *Factory {
	f.sync = syncFn
	return f
}

// WithInformers is used to register event handlers and get the caches synchronized functions.
// Pass informers you want to use to react to changes on resources. If informer event is observed, then the Sync() function
// is called.
func (f *Factory) WithInformers(informers ...Informer) *Factory {
	f.informers = append(f.informers, informers...)
	return f
}

// WithInformersQueueKeyFunc is used to register event handlers and get the caches synchronized functions.
// Pass informers you want to use to react to changes on resources. If informer event is observed, then the Sync() function
// is called.
// Pass the queueKeyFn you want to use to transform the informer runtime.Object into string key used by work queue.
func (f *Factory) WithInformersQueueKeyFunc(queueKeyFn ObjectQueueKeyFunc, informers ...Informer) *Factory {
	f.informerQueueKeys = append(f.informerQueueKeys, informersWithQueueKey{
		informers:  informers,
		queueKeyFn: queueKeyFn,
	})
	return f
}

// WithPostStartHooks allows to register functions that will run asynchronously after the controller is started via Run command.
func (f *Factory) WithPostStartHooks(hooks ...PostStartHook) *Factory {
	f.postStartHooks = append(f.postStartHooks, hooks...)
	return f
}

// WithNamespaceInformer is used to register event handlers and get the caches synchronized functions.
// The sync function will only trigger when the object observed by this informer is a namespace and its name matches the interestingNamespaces.
// Do not use this to register non-namespace informers.
func (f *Factory) WithNamespaceInformer(informer Informer, interestingNamespaces ...string) *Factory {
	f.namespaceInformers = append(f.namespaceInformers, &namespaceInformer{
		informer:   informer,
		namespaces: sets.NewString(interestingNamespaces...),
	})
	return f
}

// ResyncEvery will cause the Sync() function to be called periodically, regardless of informers.
// This is useful when you want to refresh every N minutes or you fear that your informers can be stucked.
// If this is not called, no periodical resync will happen.
// Note: The controller context passed to Sync() function in this case does not contain the object metadata or object itself.
//       This can be used to detect periodical resyncs, but normal Sync() have to be cautious about `nil` objects.
func (f *Factory) ResyncEvery(interval time.Duration) *Factory {
	f.resyncInterval = interval
	return f
}

// WithSyncContext allows to specify custom, existing sync context for this factory.
// This is useful during unit testing where you can override the default event recorder or mock the runtime objects.
// If this function not called, a SyncContext is created by the factory automatically.
func (f *Factory) WithSyncContext(ctx SyncContext) *Factory {
	f.syncContext = ctx
	return f
}

// Controller produce a runnable controller.
func (f *Factory) ToController(name string, eventRecorder events.Recorder) Controller {
	if f.sync == nil {
		panic("WithSync() must be used before calling ToController()")
	}

	var ctx SyncContext
	if f.syncContext != nil {
		ctx = f.syncContext
	} else {
		ctx = NewSyncContext(name, eventRecorder)
	}

	c := &baseController{
		name:         name,
		sync:         f.sync,
		resyncEvery:  f.resyncInterval,
		cachesToSync: append([]cache.InformerSynced{}, f.cachesToSync...),
		syncContext:  ctx,
	}

	for i := range f.informerQueueKeys {
		for d := range f.informerQueueKeys[i].informers {
			informer := f.informerQueueKeys[i].informers[d]
			queueKeyFn := f.informerQueueKeys[i].queueKeyFn
			informer.AddEventHandler(c.syncContext.(syncContext).eventHandler(queueKeyFn, sets.NewString()))
			c.cachesToSync = append(c.cachesToSync, informer.HasSynced)
		}
	}

	for i := range f.informers {
		f.informers[i].AddEventHandler(c.syncContext.(syncContext).eventHandler(func(runtime.Object) string {
			return DefaultQueueKey
		}, sets.NewString()))
		c.cachesToSync = append(c.cachesToSync, f.informers[i].HasSynced)
	}

	for i := range f.namespaceInformers {
		f.namespaceInformers[i].informer.AddEventHandler(c.syncContext.(syncContext).eventHandler(func(runtime.Object) string {
			return DefaultQueueKey
		}, f.namespaceInformers[i].namespaces))
		c.cachesToSync = append(c.cachesToSync, f.namespaceInformers[i].informer.HasSynced)
	}

	return c
}

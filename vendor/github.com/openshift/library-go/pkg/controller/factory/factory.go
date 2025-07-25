package factory

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/robfig/cron"
	"k8s.io/apimachinery/pkg/runtime"
	errorutil "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"

	"github.com/openshift/library-go/pkg/operator/events"
	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
)

// DefaultQueueKey is the queue key used for string trigger based controllers.
const DefaultQueueKey = "key"

// DefaultQueueKeysFunc returns a slice with a single element - the DefaultQueueKey
func DefaultQueueKeysFunc(_ runtime.Object) []string {
	return []string{DefaultQueueKey}
}

// Factory is generator that generate standard Kubernetes controllers.
// Factory is really generic and should be only used for simple controllers that does not require special stuff..
type Factory struct {
	sync                   SyncFunc
	syncContext            SyncContext
	syncDegradedClient     operatorv1helpers.OperatorClient
	resyncInterval         time.Duration
	resyncSchedules        []string
	informers              []filteredInformers
	informerQueueKeys      []informersWithQueueKey
	bareInformers          []Informer
	postStartHooks         []PostStartHook
	namespaceInformers     []*namespaceInformer
	cachesToSync           []cache.InformerSynced
	controllerInstanceName string
}

// Informer represents any structure that allow to register event handlers and informs if caches are synced.
// Any SharedInformer will comply.
type Informer interface {
	AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error)
	HasSynced() bool
}

type namespaceInformer struct {
	informer Informer
	nsFilter EventFilterFunc
}

type informersWithQueueKey struct {
	informers  []Informer
	filter     EventFilterFunc
	queueKeyFn ObjectQueueKeysFunc
}

type filteredInformers struct {
	informers []Informer
	filter    EventFilterFunc
}

// PostStartHook specify a function that will run after controller is started.
// The context is cancelled when the controller is asked to shutdown and the post start hook should terminate as well.
// The syncContext allow access to controller queue and event recorder.
type PostStartHook func(ctx context.Context, syncContext SyncContext) error

// ObjectQueueKeyFunc is used to make a string work queue key out of the runtime object that is passed to it.
// This can extract the "namespace/name" if you need to or just return "key" if you building controller that only use string
// triggers.
// DEPRECATED: use ObjectQueueKeysFunc instead
type ObjectQueueKeyFunc func(runtime.Object) string

// ObjectQueueKeysFunc is used to make a string work queue keys out of the runtime object that is passed to it.
// This can extract the "namespace/name" if you need to or just return "key" if you building controller that only use string
// triggers.
type ObjectQueueKeysFunc func(runtime.Object) []string

// EventFilterFunc is used to filter informer events to prevent Sync() from being called
type EventFilterFunc func(obj interface{}) bool

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
	f.WithFilteredEventsInformers(nil, informers...)
	return f
}

// WithFilteredEventsInformers is used to register event handlers and get the caches synchronized functions.
// Pass the informers you want to use to react to changes on resources. If informer event is observed, then the Sync() function
// is called.
// Pass filter to filter out events that should not trigger Sync() call.
func (f *Factory) WithFilteredEventsInformers(filter EventFilterFunc, informers ...Informer) *Factory {
	f.informers = append(f.informers, filteredInformers{
		informers: informers,
		filter:    filter,
	})
	return f
}

// WithBareInformers allow to register informer that already has custom event handlers registered and no additional
// event handlers will be added to this informer.
// The controller will wait for the cache of this informer to be synced.
// The existing event handlers will have to respect the queue key function or the sync() implementation will have to
// count with custom queue keys.
func (f *Factory) WithBareInformers(informers ...Informer) *Factory {
	f.bareInformers = append(f.bareInformers, informers...)
	return f
}

// WithInformersQueueKeyFunc is used to register event handlers and get the caches synchronized functions.
// Pass informers you want to use to react to changes on resources. If informer event is observed, then the Sync() function
// is called.
// Pass the queueKeyFn you want to use to transform the informer runtime.Object into string key used by work queue.
func (f *Factory) WithInformersQueueKeyFunc(queueKeyFn ObjectQueueKeyFunc, informers ...Informer) *Factory {
	f.informerQueueKeys = append(f.informerQueueKeys, informersWithQueueKey{
		informers: informers,
		queueKeyFn: func(o runtime.Object) []string {
			return []string{queueKeyFn(o)}
		},
	})
	return f
}

// WithFilteredEventsInformersQueueKeyFunc is used to register event handlers and get the caches synchronized functions.
// Pass informers you want to use to react to changes on resources. If informer event is observed, then the Sync() function
// is called.
// Pass the queueKeyFn you want to use to transform the informer runtime.Object into string key used by work queue.
// Pass filter to filter out events that should not trigger Sync() call.
func (f *Factory) WithFilteredEventsInformersQueueKeyFunc(queueKeyFn ObjectQueueKeyFunc, filter EventFilterFunc, informers ...Informer) *Factory {
	f.informerQueueKeys = append(f.informerQueueKeys, informersWithQueueKey{
		informers: informers,
		filter:    filter,
		queueKeyFn: func(o runtime.Object) []string {
			return []string{queueKeyFn(o)}
		},
	})
	return f
}

// WithInformersQueueKeysFunc is used to register event handlers and get the caches synchronized functions.
// Pass informers you want to use to react to changes on resources. If informer event is observed, then the Sync() function
// is called.
// Pass the queueKeyFn you want to use to transform the informer runtime.Object into string key used by work queue.
func (f *Factory) WithInformersQueueKeysFunc(queueKeyFn ObjectQueueKeysFunc, informers ...Informer) *Factory {
	f.informerQueueKeys = append(f.informerQueueKeys, informersWithQueueKey{
		informers:  informers,
		queueKeyFn: queueKeyFn,
	})
	return f
}

// WithFilteredEventsInformersQueueKeysFunc is used to register event handlers and get the caches synchronized functions.
// Pass informers you want to use to react to changes on resources. If informer event is observed, then the Sync() function
// is called.
// Pass the queueKeyFn you want to use to transform the informer runtime.Object into string key used by work queue.
// Pass filter to filter out events that should not trigger Sync() call.
func (f *Factory) WithFilteredEventsInformersQueueKeysFunc(queueKeyFn ObjectQueueKeysFunc, filter EventFilterFunc, informers ...Informer) *Factory {
	f.informerQueueKeys = append(f.informerQueueKeys, informersWithQueueKey{
		informers:  informers,
		filter:     filter,
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
		informer: informer,
		nsFilter: namespaceChecker(interestingNamespaces),
	})
	return f
}

// ResyncEvery will cause the Sync() function to be called periodically, regardless of informers.
// This is useful when you want to refresh every N minutes or you fear that your informers can be stucked.
// If this is not called, no periodical resync will happen.
// Note: The controller context passed to Sync() function in this case does not contain the object metadata or object itself.
//
//	This can be used to detect periodical resyncs, but normal Sync() have to be cautious about `nil` objects.
func (f *Factory) ResyncEvery(interval time.Duration) *Factory {
	f.resyncInterval = interval
	return f
}

// ResyncSchedule allows to supply a Cron syntax schedule that will be used to schedule the sync() call runs.
// This allows more fine-tuned controller scheduling than ResyncEvery.
// Examples:
//
// factory.New().ResyncSchedule("@every 1s").ToController()     // Every second
// factory.New().ResyncSchedule("@hourly").ToController()       // Every hour
// factory.New().ResyncSchedule("30 * * * *").ToController()	// Every hour on the half hour
//
// Note: The controller context passed to Sync() function in this case does not contain the object metadata or object itself.
//
//	This can be used to detect periodical resyncs, but normal Sync() have to be cautious about `nil` objects.
func (f *Factory) ResyncSchedule(schedules ...string) *Factory {
	f.resyncSchedules = append(f.resyncSchedules, schedules...)
	return f
}

// WithSyncContext allows to specify custom, existing sync context for this factory.
// This is useful during unit testing where you can override the default event recorder or mock the runtime objects.
// If this function not called, a SyncContext is created by the factory automatically.
func (f *Factory) WithSyncContext(ctx SyncContext) *Factory {
	f.syncContext = ctx
	return f
}

// WithSyncDegradedOnError encapsulate the controller sync() function, so when this function return an error, the operator client
// is used to set the degraded condition to (eg. "ControllerFooDegraded"). The degraded condition name is set based on the controller name.
func (f *Factory) WithSyncDegradedOnError(operatorClient operatorv1helpers.OperatorClient) *Factory {
	f.syncDegradedClient = operatorClient
	return f
}

// WithControllerInstanceName specifies the controller instance.
// Useful when the same controller is used multiple times.
func (f *Factory) WithControllerInstanceName(controllerInstanceName string) *Factory {
	f.controllerInstanceName = controllerInstanceName
	return f
}

type informerHandleTuple struct {
	informer Informer
	filter   uintptr
}

// Controller produce a runnable controller.
func (f *Factory) ToController(name string, eventRecorder events.Recorder) Controller {
	if f.sync == nil {
		panic(fmt.Errorf("WithSync() must be used before calling ToController() in %q", name))
	}

	var ctx SyncContext
	if f.syncContext != nil {
		ctx = f.syncContext
	} else {
		ctx = NewSyncContext(name, eventRecorder)
	}

	var cronSchedules []cron.Schedule
	if len(f.resyncSchedules) > 0 {
		var errors []error
		for _, schedule := range f.resyncSchedules {
			if s, err := cron.ParseStandard(schedule); err != nil {
				errors = append(errors, err)
			} else {
				cronSchedules = append(cronSchedules, s)
			}
		}
		if err := errorutil.NewAggregate(errors); err != nil {
			panic(fmt.Errorf("failed to parse controller schedules for %q: %v", name, err))
		}
	}

	c := &baseController{
		name:                   name,
		controllerInstanceName: f.controllerInstanceName,
		syncDegradedClient:     f.syncDegradedClient,
		sync:                   f.sync,
		resyncEvery:            f.resyncInterval,
		resyncSchedules:        cronSchedules,
		cachesToSync:           append([]cache.InformerSynced{}, f.cachesToSync...),
		syncContext:            ctx,
		postStartHooks:         f.postStartHooks,
		cacheSyncTimeout:       defaultCacheSyncTimeout,
	}

	// avoid adding an informer more than once
	informerQueueKeySet := sets.New[informerHandleTuple]()
	for i := range f.informerQueueKeys {
		for d := range f.informerQueueKeys[i].informers {
			informer := f.informerQueueKeys[i].informers[d]
			queueKeyFn := f.informerQueueKeys[i].queueKeyFn
			tuple := informerHandleTuple{
				informer: informer,
				filter:   reflect.ValueOf(f.informerQueueKeys[i].filter).Pointer(),
			}
			if !informerQueueKeySet.Has(tuple) {
				sets.Insert(informerQueueKeySet, tuple)
				informer.AddEventHandler(c.syncContext.(syncContext).eventHandler(queueKeyFn, f.informerQueueKeys[i].filter))
			}
			c.cachesToSync = append(c.cachesToSync, informer.HasSynced)
		}
	}

	// avoid adding an informer more than once
	informerSet := sets.New[informerHandleTuple]()
	for i := range f.informers {
		for d := range f.informers[i].informers {
			informer := f.informers[i].informers[d]
			tuple := informerHandleTuple{
				informer: informer,
				filter:   reflect.ValueOf(f.informers[i].filter).Pointer(),
			}
			if !informerSet.Has(tuple) {
				sets.Insert(informerSet, tuple)
				informer.AddEventHandler(c.syncContext.(syncContext).eventHandler(DefaultQueueKeysFunc, f.informers[i].filter))
			}
			c.cachesToSync = append(c.cachesToSync, informer.HasSynced)
		}
	}

	for i := range f.bareInformers {
		c.cachesToSync = append(c.cachesToSync, f.bareInformers[i].HasSynced)
	}

	for i := range f.namespaceInformers {
		f.namespaceInformers[i].informer.AddEventHandler(c.syncContext.(syncContext).eventHandler(DefaultQueueKeysFunc, f.namespaceInformers[i].nsFilter))
		c.cachesToSync = append(c.cachesToSync, f.namespaceInformers[i].informer.HasSynced)
	}

	return c
}

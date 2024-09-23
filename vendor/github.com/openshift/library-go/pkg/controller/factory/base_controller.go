package factory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/management"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
)

// SyntheticRequeueError can be returned from sync() in case of forcing a sync() retry artificially.
// This can be also done by re-adding the key to queue, but this is cheaper and more convenient.
var SyntheticRequeueError = errors.New("synthetic requeue request")

var defaultCacheSyncTimeout = 10 * time.Minute

// baseController represents generic Kubernetes controller boiler-plate
type baseController struct {
	name               string
	cachesToSync       []cache.InformerSynced
	sync               func(ctx context.Context, controllerContext SyncContext) error
	syncContext        SyncContext
	syncDegradedClient operatorv1helpers.OperatorClient
	resyncEvery        time.Duration
	resyncSchedules    []cron.Schedule
	postStartHooks     []PostStartHook
	cacheSyncTimeout   time.Duration
}

var _ Controller = &baseController{}

func (c baseController) Name() string {
	return c.name
}

type scheduledJob struct {
	queue workqueue.RateLimitingInterface
	name  string
}

func newScheduledJob(name string, queue workqueue.RateLimitingInterface) cron.Job {
	return &scheduledJob{
		queue: queue,
		name:  name,
	}
}

func (s *scheduledJob) Run() {
	klog.V(4).Infof("Triggering scheduled %q controller run", s.name)
	s.queue.Add(DefaultQueueKey)
}

func waitForNamedCacheSync(controllerName string, stopCh <-chan struct{}, cacheSyncs ...cache.InformerSynced) error {
	klog.Infof("Waiting for caches to sync for %s", controllerName)

	if !cache.WaitForCacheSync(stopCh, cacheSyncs...) {
		return fmt.Errorf("unable to sync caches for %s", controllerName)
	}

	klog.Infof("Caches are synced for %s ", controllerName)

	return nil
}

func (c *baseController) Run(ctx context.Context, workers int) {
	// HandleCrash recovers panics
	defer utilruntime.HandleCrash(c.degradedPanicHandler)

	// give caches 10 minutes to sync
	cacheSyncCtx, cacheSyncCancel := context.WithTimeout(ctx, c.cacheSyncTimeout)
	defer cacheSyncCancel()
	err := waitForNamedCacheSync(c.name, cacheSyncCtx.Done(), c.cachesToSync...)
	if err != nil {
		select {
		case <-ctx.Done():
			// Exit gracefully because the controller was requested to stop.
			return
		default:
			// If caches did not sync after 10 minutes, it has taken oddly long and
			// we should provide feedback. Since the control loops will never start,
			// it is safer to exit with a good message than to continue with a dead loop.
			// TODO: Consider making this behavior configurable.
			klog.Exit(err)
		}
	}

	var workerWg sync.WaitGroup
	defer func() {
		defer klog.Infof("All %s workers have been terminated", c.name)
		workerWg.Wait()
	}()

	// queueContext is used to track and initiate queue shutdown
	queueContext, queueContextCancel := context.WithCancel(context.TODO())

	for i := 1; i <= workers; i++ {
		klog.Infof("Starting #%d worker of %s controller ...", i, c.name)
		workerWg.Add(1)
		go func() {
			defer func() {
				klog.Infof("Shutting down worker of %s controller ...", c.name)
				workerWg.Done()
			}()
			c.runWorker(queueContext)
		}()
	}

	// if scheduled run is requested, run the cron scheduler
	if c.resyncSchedules != nil {
		scheduler := cron.New()
		for _, s := range c.resyncSchedules {
			scheduler.Schedule(s, newScheduledJob(c.name, c.syncContext.Queue()))
		}
		scheduler.Start()
		defer scheduler.Stop()
	}

	// runPeriodicalResync is independent from queue
	if c.resyncEvery > 0 {
		workerWg.Add(1)
		if c.resyncEvery < 60*time.Second {
			// Warn about too fast resyncs as they might drain the operators QPS.
			// This event is cheap as it is only emitted on operator startup.
			c.syncContext.Recorder().Warningf("FastControllerResync", "Controller %q resync interval is set to %s which might lead to client request throttling", c.name, c.resyncEvery)
		}
		go func() {
			defer workerWg.Done()
			wait.UntilWithContext(ctx, func(ctx context.Context) { c.syncContext.Queue().Add(DefaultQueueKey) }, c.resyncEvery)
		}()
	}

	// run post-start hooks (custom triggers, etc.)
	if len(c.postStartHooks) > 0 {
		var hookWg sync.WaitGroup
		defer func() {
			hookWg.Wait() // wait for the post-start hooks
			klog.Infof("All %s post start hooks have been terminated", c.name)
		}()
		for i := range c.postStartHooks {
			hookWg.Add(1)
			go func(index int) {
				defer hookWg.Done()
				if err := c.postStartHooks[index](ctx, c.syncContext); err != nil {
					klog.Warningf("%s controller post start hook error: %v", c.name, err)
				}
			}(i)
		}
	}

	// Handle controller shutdown

	<-ctx.Done()                     // wait for controller context to be cancelled
	c.syncContext.Queue().ShutDown() // shutdown the controller queue first
	queueContextCancel()             // cancel the queue context, which tell workers to initiate shutdown

	// Wait for all workers to finish their job.
	// at this point the Run() can hang and caller have to implement the logic that will kill
	// this controller (SIGKILL).
	klog.Infof("Shutting down %s ...", c.name)
}

func (c *baseController) Sync(ctx context.Context, syncCtx SyncContext) error {
	return c.sync(ctx, syncCtx)
}

// runWorker runs a single worker
// The worker is asked to terminate when the passed context is cancelled and is given terminationGraceDuration time
// to complete its shutdown.
func (c *baseController) runWorker(queueCtx context.Context) {
	wait.UntilWithContext(
		queueCtx,
		func(queueCtx context.Context) {
			defer utilruntime.HandleCrash(c.degradedPanicHandler)
			for {
				select {
				case <-queueCtx.Done():
					return
				default:
					c.processNextWorkItem(queueCtx)
				}
			}
		},
		1*time.Second)
}

// reconcile wraps the sync() call and if operator client is set, it handle the degraded condition if sync() returns an error.
func (c *baseController) reconcile(ctx context.Context, syncCtx SyncContext) error {
	err := c.sync(ctx, syncCtx)
	degradedErr := c.reportDegraded(ctx, err)
	if apierrors.IsNotFound(degradedErr) && management.IsOperatorRemovable() {
		// The operator tolerates missing CR, therefore don't report it up.
		return err
	}
	return degradedErr
}

// degradedPanicHandler will go degraded on failures, then we should catch potential panics and covert them into bad status.
func (c *baseController) degradedPanicHandler(panicVal interface{}) {
	if c.syncDegradedClient == nil {
		// if we don't have a client for reporting degraded condition, then let the existing panic handler do the work
		return
	}
	_ = c.reportDegraded(context.TODO(), fmt.Errorf("panic caught:\n%v", panicVal))
}

// reportDegraded updates status with an indication of degraded-ness
func (c *baseController) reportDegraded(ctx context.Context, reportedError error) error {
	if c.syncDegradedClient == nil {
		return reportedError
	}
	if reportedError != nil {
		_, _, updateErr := v1helpers.UpdateStatus(ctx, c.syncDegradedClient, v1helpers.UpdateConditionFn(operatorv1.OperatorCondition{
			Type:    c.name + "Degraded",
			Status:  operatorv1.ConditionTrue,
			Reason:  "SyncError",
			Message: reportedError.Error(),
		}))
		if updateErr != nil {
			klog.Warningf("Updating status of %q failed: %v", c.Name(), updateErr)
		}
		return reportedError
	}
	_, _, updateErr := v1helpers.UpdateStatus(ctx, c.syncDegradedClient,
		v1helpers.UpdateConditionFn(operatorv1.OperatorCondition{
			Type:   c.name + "Degraded",
			Status: operatorv1.ConditionFalse,
			Reason: "AsExpected",
		}))
	return updateErr
}

func (c *baseController) processNextWorkItem(queueCtx context.Context) {
	key, quit := c.syncContext.Queue().Get()
	if quit {
		return
	}
	defer c.syncContext.Queue().Done(key)

	syncCtx := c.syncContext.(syncContext)
	var ok bool
	syncCtx.queueKey, ok = key.(string)
	if !ok {
		utilruntime.HandleError(fmt.Errorf("%q controller failed to process key %q (not a string)", c.name, key))
		return
	}

	if err := c.reconcile(queueCtx, syncCtx); err != nil {
		if err == SyntheticRequeueError {
			// logging this helps detecting wedged controllers with missing pre-requirements
			klog.V(5).Infof("%q controller requested synthetic requeue with key %q", c.name, key)
		} else {
			if klog.V(4).Enabled() || key != "key" {
				utilruntime.HandleError(fmt.Errorf("%q controller failed to sync %q, err: %w", c.name, key, err))
			} else {
				utilruntime.HandleError(fmt.Errorf("%s reconciliation failed: %w", c.name, err))
			}
		}
		c.syncContext.Queue().AddRateLimited(key)
		return
	}

	c.syncContext.Queue().Forget(key)
}

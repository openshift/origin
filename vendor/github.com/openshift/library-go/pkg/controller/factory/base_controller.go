package factory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
)

// baseController represents generic Kubernetes controller boiler-plate
type baseController struct {
	name         string
	cachesToSync []cache.InformerSynced
	sync         func(ctx context.Context, controllerContext SyncContext) error
	resyncEvery  time.Duration
	syncContext  SyncContext
}

var _ Controller = &baseController{}

func (c *baseController) Run(ctx context.Context, workers int) {
	// HandleCrash recovers panics
	defer utilruntime.HandleCrash()
	if !cache.WaitForNamedCacheSync(c.name, ctx.Done(), c.cachesToSync...) {
		panic("timeout waiting for informer cache") // this will be recovered using HandleCrash()
	}

	var workerWaitGroup sync.WaitGroup
	defer func() {
		defer klog.Infof("All %s workers have been terminated", c.name)
		workerWaitGroup.Wait()
	}()

	// queueContext is used to track and initiate queue shutdown
	queueContext, queueContextCancel := context.WithCancel(context.TODO())

	for i := 1; i <= workers; i++ {
		klog.Infof("Starting #%d worker of %s controller ...", i, c.name)
		workerWaitGroup.Add(1)
		go func() {
			defer func() {
				klog.Infof("Shutting down worker of %s controller ...", c.name)
				workerWaitGroup.Done()
			}()
			c.runWorker(queueContext)
		}()
	}

	// runPeriodicalResync is independent from queue
	if c.resyncEvery > 0 {
		workerWaitGroup.Add(1)
		go func() {
			defer workerWaitGroup.Done()
			c.runPeriodicalResync(ctx, c.resyncEvery)
		}()
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

// QueueKey return queue key for given name.
func QueueKey(name string) string {
	return strings.ToLower(name) + "Key"
}

func (c *baseController) runPeriodicalResync(ctx context.Context, interval time.Duration) {
	go wait.UntilWithContext(ctx, func(ctx context.Context) {
		c.syncContext.Queue().Add(QueueKey(c.name))
	}, interval)
}

// runWorker runs a single worker
// The worker is asked to terminate when the passed context is cancelled and is given terminationGraceDuration time
// to complete its shutdown.
func (c *baseController) runWorker(queueCtx context.Context) {
	var workerWaitGroup sync.WaitGroup
	workerWaitGroup.Add(1)
	go func() {
		defer workerWaitGroup.Done()
		for {
			select {
			case <-queueCtx.Done():
				return
			default:
				c.processNextWorkItem(queueCtx)
			}
		}
	}()
	workerWaitGroup.Wait()
}

func (c *baseController) processNextWorkItem(queueCtx context.Context) {
	syncObject, quit := c.syncContext.Queue().Get()
	if quit {
		return
	}
	defer c.syncContext.Queue().Done(syncObject)

	runtimeObject, _ := syncObject.(runtime.Object)
	if err := c.sync(queueCtx, c.syncContext.(syncContext).withRuntimeObject(runtimeObject)); err != nil {
		utilruntime.HandleError(fmt.Errorf("%s controller failed to sync %+v with: %w", c.name, syncObject, err))
		c.syncContext.Queue().AddRateLimited(syncObject)
		return
	}

	c.syncContext.Queue().Forget(syncObject)
}

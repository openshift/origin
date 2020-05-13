package failure_detector

import (
	"context"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
)

// processFunc a function that processes a batch of EndpointSamples
type processFunc func(objs []*EndpointSample)

// processor retrieves EndpointSamples from the exposed channel and calls out to processFunc for processing
type processor struct {
	batchKeyFn KeyFunc
	queue      endPointSampleBatchQueue
	processFn  processFunc
	collectCh  chan *EndpointSample
}

// newProcessor creates a processor that adds EndpointSamples to the given queue under a key derived from the given batchKeyFn function and calls out to the given processFn function for processing
func newProcessor(batchKeyFn KeyFunc, processFn processFunc, queue endPointSampleBatchQueue) *processor {
	return &processor{
		batchKeyFn: batchKeyFn,
		queue:      queue,
		processFn:  processFn,
		collectCh:  make(chan *EndpointSample, 1000),
	}
}

// run starts the processor that
//  - runs one worker for collecting EndpointSamples from the exposed channel and adding them to the queue
//  - runs the given number of workers that takes the collected data off the queue and calls out to the defined processFunc
func (p *processor) run(ctx context.Context, workers int) {
	// TODO: shutdown the queue
	// defer p.queue.Shutdown()

	for i := 0; i < workers; i++ {
		go wait.Until(p.worker, time.Second, ctx.Done())
	}

	go wait.Until(p.collector(ctx), time.Second, ctx.Done())

	<-ctx.Done()
}

func (p *processor) worker() {
	defer utilruntime.HandleCrash()
	for p.processNextWorkItem() {
	}
}

func (p *processor) processNextWorkItem() bool {
	key, items := p.queue.Get()
	defer p.queue.Done(key)

	// sync
	p.processFn(items)

	return true
}

// collector adds collected EndpointSamples to the internal queue for processing
func (p *processor) collector(ctx context.Context) func() {
	return func() {
		defer utilruntime.HandleCrash()
		for {
			select {
			case <-ctx.Done():
				return
			case endpointSample := <-p.collectCh:
				p.queue.Add(p.batchKeyFn(endpointSample), endpointSample)
			}
		}
	}
}

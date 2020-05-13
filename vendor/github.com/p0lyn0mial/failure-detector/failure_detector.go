package failure_detector

import (
	"context"
	"net/url"
	"sync/atomic"
	"time"

	batchqueue "github.com/p0lyn0mial/batch-working-queue"
	ttlstore "github.com/p0lyn0mial/ttl-cache"

	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/sets"
)

// failureDetector is receiving endpoint samples and maintains endpoint status according to logic implemented by a policy evaluator
type failureDetector struct {
	// endpointSampleKeyFn maps collected sample (EndpointSample) for a Service to the internal store
	endpointSampleKeyFn KeyFunc

	//processor retrieves EndpointSamples from the exposed channel and calls out to processBatch() function for processing
	processor *processor

	// store holds WeightedEndpointStatusStore (samples) per Service (Namespace/Service)
	store map[string]WeightedEndpointStatusStore

	// readOnlyStore holds a copy of the store that is safe for concurrent (read) access
	readOnlyStore atomic.Value

	// createStoreFn a helper function for creating the WeightedEndpointStatusStore store
	createStoreFn NewStoreFunc

	// policyEvaluatorFn an external policy function for assessing the endpoints
	policyEvaluatorFn EvaluateFunc
}

func NewDefaultFailureDetector() *failureDetector {
	createNewStoreFn := func(ttl time.Duration) WeightedEndpointStatusStore {
		return newEndpointStore(ttlstore.New(ttl, clock.RealClock{}))
	}
	queue := newEndPointSampleBatchQueue(batchqueue.New())
	return newFailureDetector(EndpointSampleToServiceKeyFunction, SimpleWeightedEndpointStatusEvaluator, createNewStoreFn, queue)
}

func newFailureDetector(endpointSampleKeyKeyFn KeyFunc, policyEvaluator EvaluateFunc, createStoreFn NewStoreFunc, queue endPointSampleBatchQueue) *failureDetector {
	fd := &failureDetector{}
	processor := newProcessor(endpointSampleKeyKeyFn, fd.processBatch, queue)
	fd.processor = processor
	fd.store = map[string]WeightedEndpointStatusStore{}
	fd.endpointSampleKeyFn = endpointSampleKeyKeyFn
	fd.createStoreFn = createStoreFn
	fd.policyEvaluatorFn = policyEvaluator
	return fd
}

// processBatch starts processing the retrieved EndPointSamples
// first samples are added to the internal store
// then it calls out to external policy function for assessing
// finally it propagates the changes to external read-only store
func (fd *failureDetector) processBatch(endpointSamples []*EndpointSample) {
	if len(endpointSamples) == 0 {
		return
	}
	batchKey := fd.endpointSampleKeyFn(endpointSamples[0])
	endpointsStore := fd.store[batchKey]
	if endpointsStore == nil {
		endpointsStore = fd.createStoreFn(60 * time.Second)
	}

	visitedEndpointsKey := sets.NewString()
	for _, endpointSample := range endpointSamples {
		endpointKey, sample := convertToKeySample(endpointSample)
		endpoint := endpointsStore.Get(endpointKey)
		if endpoint == nil {
			// the max number of samples we are going to store and process per endpoint is 10 (it could be configurable)
			endpoint = newWeightedEndpoint(10, endpointSample.URL)
		}
		if !visitedEndpointsKey.Has(endpointKey) {
			visitedEndpointsKey.Insert(endpointKey)
		}
		endpoint.Add(sample)
		endpointsStore.Add(endpointKeyFunction(endpoint), endpoint)
	}

	hasChanged := false
	for _, visitedEndpointKey := range visitedEndpointsKey.UnsortedList() {
		endpoint := endpointsStore.Get(visitedEndpointKey)
		if fd.policyEvaluatorFn(endpoint) {
			hasChanged = true
			endpointsStore.Add(endpointKeyFunction(endpoint), endpoint)
		}
	}

	fd.store[batchKey] = endpointsStore
	if hasChanged {
		fd.propagateChangesToReadOnlyStore()
	}
}

func (fd *failureDetector) Run(ctx context.Context) {
	// if you ever change the number of workers then you need to provide a thread-safe store
	fd.processor.run(ctx, 1)
}

// Collector exposes a chan for collecting EndpointSamples
func (fd *failureDetector) Collector() chan<- *EndpointSample {
	return fd.processor.collectCh
}

// EndpointStatus returns the current status of the given endpoint for the given Service
func (fd *failureDetector) EndpointStatus(namespace, service string, url *url.URL) (isHealthy bool, weight float32) {
	isHealthy = true
	weight = 1.0

	store := fd.readOnlyStore.Load()
	if store == nil {
		// nothing has been exported yet
		// consider the endpoint healthy
		return
	}

	serviceStore := store.(map[string]WeightedEndpointStatusStore)
	epSample := &EndpointSample{Namespace: namespace, Service:service, URL:url}

	serviceKey := fd.endpointSampleKeyFn(epSample)
	endpointsStore := serviceStore[serviceKey]
	if endpointsStore == nil {
		// we haven't collected any data for this Service
		// consider the endpoint healthy
		return
	}

	endpointKey, _ := convertToKeySample(epSample)
	endpoint := endpointsStore.Get(endpointKey)
	if endpoint == nil {
		// we haven't collected any data for this endpoint
		// consider the endpoint healthy
		return
	}

	weight = endpoint.weight
	isHealthy = len(endpoint.status) == 0

    return
}

func convertToKeySample(epSample *EndpointSample) (string, *Sample) {
	return EndpointSampleKeyFunction(epSample), &Sample{
		err: epSample.Err,
	}
}

// propagateChangesToReadOnlyStore makes a copy of fd.store and puts it into fd.readOnlyStore
func (fd *failureDetector) propagateChangesToReadOnlyStore() {
	serviceStoreCopy := map[string]WeightedEndpointStatusStore{}
	for serviceKey, epStore := range fd.store {
		newEpStore := fd.createStoreFn(24 * 365 * time.Hour)
		for _, weightedEndpointStatus := range epStore.List() {
			weightedEndpointStatusCopy := newWeightedEndpoint(0, weightedEndpointStatus.url)
			weightedEndpointStatusCopy.weight = weightedEndpointStatus.weight
			weightedEndpointStatusCopy.status = weightedEndpointStatus.status
			newEpStore.Add(endpointKeyFunction(weightedEndpointStatusCopy), weightedEndpointStatusCopy)

		}
		serviceStoreCopy[serviceKey] = newEpStore
	}

	fd.readOnlyStore.Store(serviceStoreCopy)
}

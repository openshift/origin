package failure_detector

import (
	"fmt"
	"net/url"
	"time"
)

type KeyFunc func(obj interface{}) string

// NewStoreFunc a func for creating WeightedEndpointStatus store per Service
type NewStoreFunc func(ttl time.Duration) WeightedEndpointStatusStore

// EvaluateFunc a function to an external policy evaluator that sets the status and weight of the given endpoint based on the collected samples.
type EvaluateFunc func(endpoint *WeightedEndpointStatus) bool

// Store an in-memory store for storing and retrieving arbitrary data
//
// For now it is used by newEndpointStore function and converted to a strongly typed store (WeightedEndpointStatus)
// It will be removed in the future once we move the store implementation to this package
type Store interface {
	// Add adds the given object to the store under the given key
	Add(key string, obj interface{})

	// Get retrieves an object from the store at the given key
	Get(key string) interface{}

	// List returns all objects in the store
	List() []interface{}
}

// WeightedEndpointStatusStore an in-memory store for WeightedEndpointStatus.
// It automatically removed entries that exceed the configured TTL
// as that allows for removing unused/removed endpoints
type WeightedEndpointStatusStore interface {
	// Add adds the given object to the store under the given key
	Add(key string, obj *WeightedEndpointStatus)

	// Get retrieves WeightedEndpointStatus from the store at the given key
	Get(key string) *WeightedEndpointStatus

	// List returns all WeightedEndpointStatus in the store
	List() []*WeightedEndpointStatus
}

// endpointStore implements WeightedEndpointStatusStore interface
type endpointStore struct {
	Store
}

// newEndpointStore creates a new strongly typed store that implements WeightedEndpointStatusStore interface
func newEndpointStore(delegate Store) *endpointStore {
	return &endpointStore{Store: delegate}
}

// Add adds the given WeightedEndpointStatus to the store under the given key
func (e *endpointStore) Add(key string, ep *WeightedEndpointStatus) {
	e.Store.Add(key, ep)
}

// Get retrieves a WeightedEndpointStatus from the store under the given key
func (e *endpointStore) Get(key string) *WeightedEndpointStatus {
	rawEndpoint := e.Store.Get(key)
	if rawEndpoint == nil {
		return nil
	}
	return rawEndpoint.(*WeightedEndpointStatus)
}

// List returns all WeightedEndpointStatus in the store
func (e *endpointStore) List() []*WeightedEndpointStatus {
	rawEndpoints := e.Store.List()
	endpoints := make([]*WeightedEndpointStatus, len(rawEndpoints))
	for idx, rawEndpoint := range rawEndpoints {
		endpoints[idx] = rawEndpoint.(*WeightedEndpointStatus)
	}
	return endpoints
}

// BatchQueue represents a generic work queue that process items in the order in which they were added,
// it also supports batching - items are grouped by a key and could be retrieved as a package
//
// For now it is used by newEndPointSampleBatchQueue function and converted to a strongly typed queue (endPointSampleBatchQueue)
// It will be removed in the future once we move the queue implementation to this package
type BatchQueue interface {
	// Get retrieves the next batch of collected items/work along with the unique key
	// A caller must execute the corresponding Done() method once it has finished its work
	Get() (key string, items []interface{})

	// Add adds the given item under the given key to the queue
	Add(key string, item interface{})

	// Done indicates that the caller finished working on items represented by a unique key
	// if it has been added again while it was being processed, it will be re-added to the queue for re-processing
	Done(key string)
}

// BatchQueue represents work queue that process EndpointSamples in the order in which they were added,
// it also supports batching - items are grouped by a key and could be retrieved as a package
type endPointSampleBatchQueue interface {
	// Get retrieves the next batch of collected EndpointSamples along with the unique key
	// A caller must execute the corresponding Done() method once it has finished its work
	Get() (key string, items []*EndpointSample)

	// Add adds the given EndpointSample under the given key to the queue
	Add(key string, item *EndpointSample)

	// Done indicates that the caller finished working on the batch of EndpointSamples represented by a unique key
	// if it has been added again while it was being processed, it will be re-added to the queue for re-processing
	Done(key string)
}

// endpointSampleBatchQueue implements endPointSampleBatchQueue
type endpointSampleBatchQueue struct {
	BatchQueue
}

// Get retrieves the next batch of collected EndpointSamples along with the unique key
// A caller must execute the corresponding Done() method once it has finished its work
func (q *endpointSampleBatchQueue) Get() (key string, items []*EndpointSample) {
	key, rawEndpointSample := q.BatchQueue.Get()
	endpointSamples := make([]*EndpointSample, len(rawEndpointSample))
	for i, r := range rawEndpointSample {
		endpointSamples[i] = r.(*EndpointSample)
	}
	return key, endpointSamples
}

// Add adds the given EndpointSample under the given key to the queue
func (q *endpointSampleBatchQueue) Add(key string, item *EndpointSample) {
	q.BatchQueue.Add(key, item)
}

// Done indicates that the caller finished working on the batch of EndpointSamples represented by a unique key
// if it has been added again while it was being processed, it will be re-added to the queue for re-processing
func (q *endpointSampleBatchQueue) Done(key string) {
	q.BatchQueue.Done(key)
}

// newEndPointSampleBatchQueue creates a strongly typed batch queue from the delegate
// the returned queue implements endPointSampleBatchQueue interface
func newEndPointSampleBatchQueue(delegate BatchQueue) endPointSampleBatchQueue {
	return &endpointSampleBatchQueue{BatchQueue: delegate}
}

// EndpointSample represents a sample collected for an endpoint derived from a proxied request.
// it holds:
//  - Namespace, Service and URL to uniquely identify the request
//  - an optional Err returned from the proxy
type EndpointSample struct {
	Namespace string
	Service   string
	URL       *url.URL
	Err       error
}

// WeightedEndpointStatus represents the current status of the given endpoint based on the collected samples.
// The status will be examined and filled by the external policy.
type WeightedEndpointStatus struct {
	data     []*Sample
	position int
	size     int

	url    *url.URL
	status string
	weight float32
}

// Sample represents a single sample collected for an endpoint
type Sample struct {
	err error
	// TODO: store latency
}

// newWeightedEndpoint creates WeightedEndpointStatus for the given URL
// it will store exactly "the size" of Samples
func newWeightedEndpoint(size int, url *url.URL) *WeightedEndpointStatus {
	ep := &WeightedEndpointStatus{}
	ep.data = make([]*Sample, size, size)
	ep.url = url
	ep.weight = 1
	ep.size = size
	return ep
}

// Add adds the given sample to the internal store
// it will overwrite the old values when it exceeds the configured capacity
func (ep *WeightedEndpointStatus) Add(sample *Sample) {
	size := cap(ep.data)
	ep.position = ep.position % size
	ep.data[ep.position] = sample
	ep.position = ep.position + 1
}

// Get retrieves the collected samples so far
func (ep *WeightedEndpointStatus) Get() []*Sample {
	size := cap(ep.data)
	ret := []*Sample{}

	for i := ep.position % size; i < size; i++ {
		if ep.data[i] == nil {
			break
		}
		ret = append(ret, ep.data[i])
	}
	for i := 0; i < ep.position%size; i++ {
		ret = append(ret, ep.data[i])
	}

	return ret
}

// EndpointSampleToServiceKeyFunction a function used by the batch queue and the internal store (failureDetector.store) for deriving a key from EndpointSample.
// The key identifies a Service an endpoints belongs to
func EndpointSampleToServiceKeyFunction(obj interface{}) string {
	item := obj.(*EndpointSample)
	return fmt.Sprintf("%s/%s", item.Namespace, item.Service)
}

// EndpointSampleKeyFunction a function used for deriving a key from an EndpointSample that uniquely identifies it
func EndpointSampleKeyFunction(obj interface{}) string {
	item := obj.(*EndpointSample)
	if item.URL == nil {
		return ""
	}
	return item.URL.Host
}

// endpointKeyFunction a function used for deriving a key from a WeightedEndpointStatus that uniquely identifies it
func endpointKeyFunction(obj interface{}) string {
	item := obj.(*WeightedEndpointStatus)
	if item.url == nil {
		return ""
	}
	return item.url.Host
}

const (
	// EndpointStatusReasonTooManyErrors means the detector experienced too many samples that indicated an error
	EndpointStatusReasonTooManyErrors = "TooManyErrors"
)

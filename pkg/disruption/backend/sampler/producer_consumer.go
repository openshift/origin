package sampler

import (
	"context"
	"fmt"

	"github.com/openshift/origin/pkg/disruption/backend"
	"github.com/openshift/origin/pkg/disruption/sampler"
)

// NewSampleProducerConsumer returns a ProducerConsumer, the Producer is
// capable of generating a new HTTP request and send it to to the given
// backend server, and the consumer is capable of receiving the result
// of the request sent and then feed the result to the specified
// SampleCollector for further analysis.
//
//	client: a wired http.Client that can send an HTTP request
//	 to a given server
//	requestor: a Requestor that can generate a desired http.Request
//	 object to be sent to the server.
//	checker: the ResponseChecker can check the result and determine
//	 whether this request should be treated as a failure.
//	collector: user specified SampleCollector that will collect each
//	 sample result for further analysis.
func NewSampleProducerConsumer(client backend.Client, requestor Requestor, checker ResponseChecker,
	collector SampleCollector) sampler.ProducerConsumer {
	return &producerConsumer{
		client:    client,
		requestor: requestor,
		checker:   checker,
		collector: collector,
	}
}

type producerConsumer struct {
	client    backend.Client
	requestor Requestor
	checker   ResponseChecker
	collector SampleCollector
}

func (pc *producerConsumer) Produce(stop context.Context, sampleID uint64) (interface{}, error) {
	rr := backend.RequestResponse{
		RequestContextAssociatedData: backend.RequestContextAssociatedData{},
	}

	// we leave it to the round tripper to set the appropriate request deadline.
	// we intentionally don't use the stop context as the base context since
	// we want a request in progress to be able to complete even if the stop
	// context is Canceled.
	ctx := backend.WithRequestContextAssociatedData(context.Background(), &rr.RequestContextAssociatedData)
	req, err := pc.requestor.NewHTTPRequest(ctx, sampleID)
	if err != nil {
		return rr, fmt.Errorf("backend-sampler: failed to create a new request - err: %w", err)
	}
	rr.Request = req

	resp, err := pc.client.Do(req)
	if err != nil {
		return rr, pc.checker.CheckError(err)
	}
	rr.Response = resp
	err = pc.checker.CheckResponse(rr)
	return rr, err
}

func (pc producerConsumer) Consume(s *sampler.Sample, custom interface{}) {
	// should never happen, we panic if for some programmer error
	rr := custom.(backend.RequestResponse)
	pc.collector.Collect(backend.SampleResult{
		Sample:          s,
		RequestResponse: rr,
	})
}

func (pc producerConsumer) Close() {
	// no more sample available, send an empty value
	pc.collector.Collect(backend.SampleResult{})
}

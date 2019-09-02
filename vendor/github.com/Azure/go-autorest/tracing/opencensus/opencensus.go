package opencensus

// Copyright 2018 Microsoft Corporation
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"contrib.go.opencensus.io/exporter/ocagent"
	"github.com/Azure/go-autorest/tracing"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/plugin/ochttp/propagation/tracecontext"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
)

func init() {
	enableFromEnv()
}

// split out for testing purposes
func enableFromEnv() {
	if _, ok := os.LookupEnv("AZURE_SDK_TRACING_ENABLED"); ok {
		agentEndpoint, ok := os.LookupEnv("OCAGENT_TRACE_EXPORTER_ENDPOINT")
		if ok {
			EnableWithAIForwarding(agentEndpoint)
		} else {
			Enable()
		}
	}
}

var defaultTracer = newocTracer()

type ocTracer struct {
	// Sampler is the tracing sampler. If tracing is disabled it will never sample. Otherwise
	// it will be using the parent sampler or the default.
	sampler trace.Sampler

	// Views for metric instrumentation.
	views map[string]*view.View

	// the trace exporter
	traceExporter trace.Exporter
}

func newocTracer() *ocTracer {
	return &ocTracer{
		sampler: trace.NeverSample(),
		views:   map[string]*view.View{},
	}
}

// NewTransport returns a new instance of a tracing-aware RoundTripper.
func (oct ocTracer) NewTransport(base *http.Transport) http.RoundTripper {
	return &ochttp.Transport{
		Base:        base,
		Propagation: &tracecontext.HTTPFormat{},
		GetStartOptions: func(*http.Request) trace.StartOptions {
			return trace.StartOptions{
				Sampler: oct.sampler,
			}
		},
	}
}

// StartSpan starts a trace span
func (oct ocTracer) StartSpan(ctx context.Context, name string) context.Context {
	ctx, _ = trace.StartSpan(ctx, name, trace.WithSampler(oct.sampler))
	return ctx
}

// EndSpan ends a previously started span stored in the context
func (oct ocTracer) EndSpan(ctx context.Context, httpStatusCode int, err error) {
	span := trace.FromContext(ctx)

	if span == nil {
		return
	}

	if err != nil {
		span.SetStatus(trace.Status{Message: err.Error(), Code: toTraceStatusCode(httpStatusCode)})
	}
	span.End()
}

// Enable will start instrumentation for metrics and traces.
func Enable() error {
	defaultTracer.sampler = nil

	// register the views for HTTP metrics
	clientViews := []*view.View{
		ochttp.ClientCompletedCount,
		ochttp.ClientRoundtripLatencyDistribution,
		ochttp.ClientReceivedBytesDistribution,
		ochttp.ClientSentBytesDistribution,
	}
	for _, cv := range clientViews {
		vn := fmt.Sprintf("Azure/go-autorest/tracing/opencensus-%s", cv.Name)
		defaultTracer.views[vn] = cv.WithName(vn)
		err := view.Register(defaultTracer.views[vn])
		if err != nil {
			return err
		}
	}
	tracing.Register(defaultTracer)
	return nil
}

// Disable will disable instrumentation for metrics and traces.
func Disable() {
	// unregister any previously registered metrics
	for _, v := range defaultTracer.views {
		view.Unregister(v)
	}
	defaultTracer.sampler = trace.NeverSample()
	if defaultTracer.traceExporter != nil {
		trace.UnregisterExporter(defaultTracer.traceExporter)
	}
	tracing.Register(nil)
}

// EnableWithAIForwarding will start instrumentation and will connect to app insights forwarder
// exporter making the metrics and traces available in app insights.
func EnableWithAIForwarding(agentEndpoint string) error {
	err := Enable()
	if err != nil {
		return err
	}

	defaultTracer.traceExporter, err = ocagent.NewExporter(ocagent.WithInsecure(), ocagent.WithAddress(agentEndpoint))
	if err != nil {
		return err
	}
	trace.RegisterExporter(defaultTracer.traceExporter)
	return nil
}

// toTraceStatusCode converts HTTP Codes to OpenCensus codes as defined
// at https://github.com/census-instrumentation/opencensus-specs/blob/master/trace/HTTP.md#status
func toTraceStatusCode(httpStatusCode int) int32 {
	switch {
	case http.StatusOK <= httpStatusCode && httpStatusCode < http.StatusBadRequest:
		return trace.StatusCodeOK
	case httpStatusCode == http.StatusBadRequest:
		return trace.StatusCodeInvalidArgument
	case httpStatusCode == http.StatusUnauthorized: // 401 is actually unauthenticated.
		return trace.StatusCodeUnauthenticated
	case httpStatusCode == http.StatusForbidden:
		return trace.StatusCodePermissionDenied
	case httpStatusCode == http.StatusNotFound:
		return trace.StatusCodeNotFound
	case httpStatusCode == http.StatusTooManyRequests:
		return trace.StatusCodeResourceExhausted
	case httpStatusCode == 499:
		return trace.StatusCodeCancelled
	case httpStatusCode == http.StatusNotImplemented:
		return trace.StatusCodeUnimplemented
	case httpStatusCode == http.StatusServiceUnavailable:
		return trace.StatusCodeUnavailable
	case httpStatusCode == http.StatusGatewayTimeout:
		return trace.StatusCodeDeadlineExceeded
	default:
		return trace.StatusCodeUnknown
	}
}

/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package filters

import (
	"fmt"
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/sets"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
)

// Constant for the retry-after interval on rate limiting.
// TODO: maybe make this dynamic? or user-adjustable?
const retryAfter = "1"

var nonMutatingRequestVerbs = sets.NewString("get", "list", "watch")

func handleError(w http.ResponseWriter, r *http.Request, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(w, "Internal Server Error: %#v", r.RequestURI)
	glog.Errorf(err.Error())
}

// WithMaxInFlightLimit limits the number of in-flight requests to buffer size of the passed in channel.
func WithMaxInFlightLimit(
	handler http.Handler,
	nonMutatingLimit int,
	mutatingLimit int,
	requestContextMapper genericapirequest.RequestContextMapper,
	longRunningRequestCheck LongRunningRequestCheck,
) http.Handler {
	if nonMutatingLimit == 0 && mutatingLimit == 0 {
		return handler
	}
	var nonMutatingChan chan bool
	var mutatingChan chan bool
	if nonMutatingLimit != 0 {
		nonMutatingChan = make(chan bool, nonMutatingLimit)
	}
	if mutatingLimit != 0 {
		mutatingChan = make(chan bool, mutatingLimit)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, ok := requestContextMapper.Get(r)
		if !ok {
			handleError(w, r, fmt.Errorf("no context found for request, handler chain must be wrong"))
			return
		}
		requestInfo, ok := apirequest.RequestInfoFrom(ctx)
		if !ok {
			handleError(w, r, fmt.Errorf("no RequestInfo found in context, handler chain must be wrong"))
			return
		}

		// Skip tracking long running events.
		if longRunningRequestCheck != nil && longRunningRequestCheck(r, requestInfo) {
			handler.ServeHTTP(w, r)
			return
		}

		var c chan bool
		if !nonMutatingRequestVerbs.Has(requestInfo.Verb) {
			c = mutatingChan
		} else {
			c = nonMutatingChan
		}

		if c == nil {
			handler.ServeHTTP(w, r)
		} else {
			select {
			case c <- true:
				defer func() { <-c }()
				handler.ServeHTTP(w, r)
			default:
				tooManyRequests(r, w, requestInfo)
			}
		}
	})
}

func tooManyRequests(req *http.Request, w http.ResponseWriter, requestInfo *apirequest.RequestInfo) {
	// Return a 429 status indicating "Too Many Requests"
	w.Header().Set("Retry-After", retryAfter)
	http.Error(w, "Too many requests, please try again later.", errors.StatusTooManyRequests)

	if requestInfo.IsResourceRequest {
		requestCounter.WithLabelValues(requestInfo.Verb, requestInfo.Resource, requestInfo.Subresource, cleanUserAgent(utilnet.GetHTTPClient(req)), "").Inc()
	} else {
		requestCounter.WithLabelValues(requestInfo.Verb, "", "", cleanUserAgent(utilnet.GetHTTPClient(req)), requestInfo.Path).Inc()
	}
}

var (
	requestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "maxinflight_request_count",
			Help: "Counter of rate-limited apiserver requests broken out by source. DEPRECATED: will be removed in 3.7.",
		},
		[]string{"verb", "resource", "subresource", "client", "path"},
	)
)

func init() {
	prometheus.MustRegister(requestCounter)
}

func cleanUserAgent(ua string) string {
	// We collapse all "web browser"-type user agents into one "browser" to reduce metric cardinality.
	if strings.HasPrefix(ua, "Mozilla/") {
		return "Browser"
	}
	return ua
}

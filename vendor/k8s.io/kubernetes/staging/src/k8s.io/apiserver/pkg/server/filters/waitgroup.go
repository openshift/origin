/*
Copyright 2017 The Kubernetes Authors.

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
	"errors"
	"net/http"

	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilwaitgroup "k8s.io/apimachinery/pkg/util/waitgroup"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/kubernetes/scheme"
)

// WithWaitGroup adds all non long-running requests to wait group, which is used for graceful shutdown.
func WithWaitGroup(handler http.Handler, longRunning apirequest.LongRunningRequestCheck, wg *utilwaitgroup.SafeWaitGroup) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		requestInfo, ok := apirequest.RequestInfoFrom(ctx)
		if !ok {
			// if this happens, the handler chain isn't setup correctly because there is no request info
			responsewriters.InternalError(w, req, errors.New("no RequestInfo found in the context"))
			return
		}

		if !longRunning(req, requestInfo) {
			if err := wg.Add(1); err != nil {
				// When apiserver is shutting down, signal clients to retry
				w.Header().Add("Retry-After", "1")
				w.Header().Add("Content-Type", runtime.ContentTypeJSON)
				statusErr := apierrors.NewServiceUnavailable("apiserver is shutting down").Status()
				http.Error(w, runtime.EncodeOrDie(scheme.Codecs.LegacyCodec(v1.SchemeGroupVersion), &statusErr), int(statusErr.Code))
				return
			}
			defer wg.Done()
		}

		handler.ServeHTTP(w, req)
	})
}

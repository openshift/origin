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
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	"k8s.io/klog"
)

// WithArtificialDelayAdder adds artificial delay to the .
func WithArtificialDelayAdder(
	handler http.Handler,
	userName string,
	longRunningRequestCheck apirequest.LongRunningRequestCheck,
) http.Handler {

	klog.Info("WithArtificialDelayAdder: adding artificial delay adder filter")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
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

		user, ok := apirequest.UserFrom(ctx)
		if !ok {
			handleError(w, r, fmt.Errorf("no User found in context"))
			return
		}

		if user.GetName() != userName {
			handler.ServeHTTP(w, r)
			return
		}

		// add the delay.
		waitTime := wait.Jitter(5 * time.Second, 1.0)
		<-time.After(waitTime)
		handler.ServeHTTP(w, r)
	})
}

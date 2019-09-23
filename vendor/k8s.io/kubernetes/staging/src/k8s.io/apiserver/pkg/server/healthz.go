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

package server

import (
	"fmt"
	"net/http"

	"k8s.io/apiserver/pkg/server/healthz"
)

// AddHealthzCheck allows you to add a HealthzCheck.
func (s *GenericAPIServer) AddHealthzChecks(checks ...healthz.HealthzChecker) error {
	s.healthzLock.Lock()
	defer s.healthzLock.Unlock()

	if s.healthChecksLocked {
		return fmt.Errorf("unable to add because the healthz endpoint has already been created")
	}

	s.healthzChecks = append(s.healthzChecks, checks...)
	return nil
}

// installHealthz creates the healthz endpoint for this server
func (s *GenericAPIServer) installHealthz() {
	s.healthzLock.Lock()
	defer s.healthzLock.Unlock()
	s.healthChecksLocked = true

	healthz.InstallHandler(s.Handler.NonGoRestfulMux, s.healthzChecks...)
}

// installReadyz creates the readyz endpoint for this server, using the defined healthz check plus a termination check
// that fails when the process is terminating.
func (s *GenericAPIServer) installReadyz(stopCh <-chan struct{}) {
	s.healthzLock.Lock()
	defer s.healthzLock.Unlock()
	s.healthChecksLocked = true

	healthz.InstallPathHandler(s.Handler.NonGoRestfulMux, "/readyz", append([]healthz.HealthzChecker{terminationCheck{stopCh}}, s.healthzChecks...)...)
}

// terminationCheck fails if the embedded channel is closed during termination of the process.
type terminationCheck struct {
	StopCh <-chan struct{}
}

func (terminationCheck) Name() string {
	return "terminating"
}

func (c terminationCheck) Check(req *http.Request) error {
	select {
	case <-c.StopCh:
		return fmt.Errorf("process is terminating")
	default:
	}
	return nil
}

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

package server

import (
	"k8s.io/apiserver/pkg/admission"
)

// SetAdmission gives access to the admission plugin.  We need this in 3.6 to allow us to twiddle admission
// until we are aggregating
// TODO drop this after we aggregate ourselvse
func (s *GenericAPIServer) SetAdmission(admissionControl admission.Interface) {
	s.admissionControl = admissionControl
}

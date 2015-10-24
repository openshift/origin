/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package testclient

import (
	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
)

// FakePodLogs implements PodTemplatesInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakePodLogs struct {
	Fake      *Fake
	Namespace string
}

func (c *FakePodLogs) Get(name string, opts *api.PodLogOptions) (*client.Request, error) {
	action := GenericActionImpl{}
	action.Verb = "get"
	action.Namespace = c.Namespace
	action.Resource = "pod"
	action.Subresource = "logs"
	action.Value = opts

	_, _ = c.Fake.Invokes(action, &api.Pod{})
	return &client.Request{}, nil
}

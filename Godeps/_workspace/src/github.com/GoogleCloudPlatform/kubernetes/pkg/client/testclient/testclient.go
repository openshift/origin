/*
Copyright 2014 Google Inc. All rights reserved.

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
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/version"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
)

type FakeAction struct {
	Action string
	Value  interface{}
}

type ReactionFunc func(FakeAction) (runtime.Object, error)

// Fake implements Interface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the method you want to test easier.
type Fake struct {
	Actions []FakeAction
	Watch   watch.Interface
	Err     error

	// ReactFn is an optional function that will be invoked with the provided action
	// and return a response.
	ReactFn ReactionFunc
}

func (c *Fake) Invokes(action FakeAction, obj runtime.Object) (runtime.Object, error) {
	c.Actions = append(c.Actions, action)
	if c.ReactFn != nil {
		return c.ReactFn(action)
	}
	return obj, c.Err
}

func (c *Fake) LimitRanges(namespace string) client.LimitRangeInterface {
	return &FakeLimitRanges{Fake: c, Namespace: namespace}
}

func (c *Fake) ResourceQuotas(namespace string) client.ResourceQuotaInterface {
	return &FakeResourceQuotas{Fake: c, Namespace: namespace}
}

func (c *Fake) ReplicationControllers(namespace string) client.ReplicationControllerInterface {
	return &FakeReplicationControllers{Fake: c, Namespace: namespace}
}

func (c *Fake) Nodes() client.NodeInterface {
	return &FakeNodes{Fake: c}
}

func (c *Fake) Events(namespace string) client.EventInterface {
	return &FakeEvents{Fake: c}
}

func (c *Fake) Endpoints(namespace string) client.EndpointsInterface {
	return &FakeEndpoints{Fake: c, Namespace: namespace}
}

func (c *Fake) Pods(namespace string) client.PodInterface {
	return &FakePods{Fake: c, Namespace: namespace}
}

func (c *Fake) Services(namespace string) client.ServiceInterface {
	return &FakeServices{Fake: c, Namespace: namespace}
}

func (c *Fake) Secrets(namespace string) client.SecretsInterface {
	return &FakeSecrets{Fake: c, Namespace: namespace}
}

func (c *Fake) Namespaces() client.NamespaceInterface {
	return &FakeNamespaces{Fake: c}
}

func (c *Fake) ServerVersion() (*version.Info, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-version", Value: nil})
	versionInfo := version.Get()
	return &versionInfo, nil
}

func (c *Fake) ServerAPIVersions() (*api.APIVersions, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-apiversions", Value: nil})
	return &api.APIVersions{Versions: []string{"v1beta1", "v1beta2"}}, nil
}

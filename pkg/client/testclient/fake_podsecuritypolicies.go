/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/security/policy/api"
)

// FakePodSecurityPolicies implements SecurityContextConstraintInterface. Meant to be
// embedded into a struct to get a default implementation. This makes faking out just
// the method you want to test easier.
type FakePodSecurityPolicies struct {
	Fake      *Fake
	Namespace string
}

func (c *FakePodSecurityPolicies) List(label labels.Selector, field fields.Selector) (*api.PodSecurityPolicyList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewListAction("podsecuritypolicies", c.Namespace, label, field), &api.PodSecurityPolicyList{})
	return obj.(*api.PodSecurityPolicyList), err
}

func (c *FakePodSecurityPolicies) Get(name string) (*api.PodSecurityPolicy, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewGetAction("podsecuritypolicies", c.Namespace, name), &api.PodSecurityPolicy{})
	return obj.(*api.PodSecurityPolicy), err
}

func (c *FakePodSecurityPolicies) Create(scc *api.PodSecurityPolicy) (*api.PodSecurityPolicy, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewCreateAction("podsecuritypolicies", c.Namespace, scc), &api.PodSecurityPolicy{})
	return obj.(*api.PodSecurityPolicy), err
}

func (c *FakePodSecurityPolicies) Update(scc *api.PodSecurityPolicy) (*api.PodSecurityPolicy, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewUpdateAction("podsecuritypolicies", c.Namespace, scc), &api.PodSecurityPolicy{})
	return obj.(*api.PodSecurityPolicy), err
}

func (c *FakePodSecurityPolicies) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewDeleteAction("podsecuritypolicies", c.Namespace, name), &api.PodSecurityPolicy{})
	return err
}

func (c *FakePodSecurityPolicies) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return c.Fake.InvokesWatch(ktestclient.NewWatchAction("podsecuritypolicies", c.Namespace, label, field, resourceVersion))
}

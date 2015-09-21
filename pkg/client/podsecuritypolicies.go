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

package client

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/security/policy/api"
)

type PodSecurityPoliciesInterface interface {
	PodSecurityPolicies() PodSecurityPolicyInterface
}

type PodSecurityPolicyInterface interface {
	Get(name string) (result *api.PodSecurityPolicy, err error)
	Create(psp *api.PodSecurityPolicy) (*api.PodSecurityPolicy, error)
	List(label labels.Selector, field fields.Selector) (*api.PodSecurityPolicyList, error)
	Delete(name string) error
	Update(*api.PodSecurityPolicy) (*api.PodSecurityPolicy, error)
	Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error)
}

// podSecurityPolicies implements SecurityContextConstraintInterface
type podSecurityPolicy struct {
	client *Client
}

// newpodSecurityPolicies returns a podSecurityPolicies object.
func newPodSecurityPolicy(c *Client) *podSecurityPolicy {
	return &podSecurityPolicy{c}
}

func (s *podSecurityPolicy) Create(psp *api.PodSecurityPolicy) (*api.PodSecurityPolicy, error) {
	result := &api.PodSecurityPolicy{}
	err := s.client.Post().
		Resource("podSecurityPolicies").
		Body(psp).
		Do().
		Into(result)

	return result, err
}

// List returns a list of PodSecurityPolicy matching the selectors.
func (s *podSecurityPolicy) List(label labels.Selector, field fields.Selector) (*api.PodSecurityPolicyList, error) {
	result := &api.PodSecurityPolicyList{}

	err := s.client.Get().
		Resource("podSecurityPolicies").
		LabelsSelectorParam(label).
		FieldsSelectorParam(field).
		Do().
		Into(result)

	return result, err
}

// Get returns the given podSecurityPolicies, or an error.
func (s *podSecurityPolicy) Get(name string) (*api.PodSecurityPolicy, error) {
	result := &api.PodSecurityPolicy{}
	err := s.client.Get().
		Resource("podSecurityPolicies").
		Name(name).
		Do().
		Into(result)

	return result, err
}

// Watch starts watching for podSecurityPolicies matching the given selectors.
func (s *podSecurityPolicy) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return s.client.Get().
		Prefix("watch").
		Resource("podSecurityPolicies").
		Param("resourceVersion", resourceVersion).
		LabelsSelectorParam(label).
		FieldsSelectorParam(field).
		Watch()
}

func (s *podSecurityPolicy) Delete(name string) error {
	return s.client.Delete().
		Resource("podSecurityPolicies").
		Name(name).
		Do().
		Error()
}

func (s *podSecurityPolicy) Update(psp *api.PodSecurityPolicy) (result *api.PodSecurityPolicy, err error) {
	result = &api.PodSecurityPolicy{}
	err = s.client.Put().
		Resource("podSecurityPolicies").
		Name(psp.Name).
		Body(psp).
		Do().
		Into(result)

	return
}

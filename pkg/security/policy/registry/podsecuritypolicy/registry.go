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

package podsecuritypolicy

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"

	policyapi "github.com/openshift/origin/pkg/security/policy/api"
)

// Registry is an interface implemented by things that know how to store SecurityContextConstraints objects.
type Registry interface {
	// ListPodSecurityPolicies obtains a list of PodSecurityPolicies having labels which match selector.
	ListPodSecurityPolicies(ctx api.Context, selector labels.Selector) (*policyapi.PodSecurityPolicyList, error)
	// Watch for new/changed/deleted PodSecurityPolicies
	WatchPodSecurityPolicies(ctx api.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error)
	// Get a specific PodSecurityPolicy
	GetPodSecurityPolicy(ctx api.Context, name string) (*policyapi.PodSecurityPolicy, error)
	// Create a PodSecurityPolicy based on a specification.
	CreatePodSecurityPolicy(ctx api.Context, scc *policyapi.PodSecurityPolicy) error
	// Update an existing PodSecurityPolicy
	UpdatePodSecurityPolicy(ctx api.Context, scc *policyapi.PodSecurityPolicy) error
	// Delete an existing PodSecurityPolicy
	DeletePodSecurityPolicy(ctx api.Context, name string) error
}

// storage puts strong typing around storage calls
type storage struct {
	rest.StandardStorage
}

// NewRegistry returns a new Registry interface for the given Storage. Any mismatched
// types will panic.
func NewRegistry(s rest.StandardStorage) Registry {
	return &storage{s}
}

func (s *storage) ListPodSecurityPolicies(ctx api.Context, label labels.Selector) (*policyapi.PodSecurityPolicyList, error) {
	obj, err := s.List(ctx, label, fields.Everything())
	if err != nil {
		return nil, err
	}
	return obj.(*policyapi.PodSecurityPolicyList), nil
}

func (s *storage) WatchPodSecurityPolicies(ctx api.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return s.Watch(ctx, label, field, resourceVersion)
}

func (s *storage) GetPodSecurityPolicy(ctx api.Context, name string) (*policyapi.PodSecurityPolicy, error) {
	obj, err := s.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return obj.(*policyapi.PodSecurityPolicy), nil
}

func (s *storage) CreatePodSecurityPolicy(ctx api.Context, scc *policyapi.PodSecurityPolicy) error {
	_, err := s.Create(ctx, scc)
	return err
}

func (s *storage) UpdatePodSecurityPolicy(ctx api.Context, scc *policyapi.PodSecurityPolicy) error {
	_, _, err := s.Update(ctx, scc)
	return err
}

func (s *storage) DeletePodSecurityPolicy(ctx api.Context, name string) error {
	_, err := s.Delete(ctx, name, nil)
	return err
}

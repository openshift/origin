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
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
)

// FakeSecurityContextConstraints implements SecurityContextConstraintInterface. Meant to be
// embedded into a struct to get a default implementation. This makes faking out just
// the method you want to test easier.
type FakeSecurityContextConstraints struct {
	Fake *Fake
}

func (c *FakeSecurityContextConstraints) List(labels labels.Selector, field fields.Selector) (*api.SecurityContextConstraintsList, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "list-securitycontextconstraints"}, &api.SecurityContextConstraintsList{})
	return obj.(*api.SecurityContextConstraintsList), err
}

func (c *FakeSecurityContextConstraints) Get(name string) (*api.SecurityContextConstraints, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "get-securitycontextconstraints", Value: name}, &api.SecurityContextConstraints{})
	return obj.(*api.SecurityContextConstraints), err
}

func (c *FakeSecurityContextConstraints) Create(scc *api.SecurityContextConstraints) (*api.SecurityContextConstraints, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "create-securitycontextconstraints", Value: scc}, &api.SecurityContextConstraints{})
	return obj.(*api.SecurityContextConstraints), err
}

func (c *FakeSecurityContextConstraints) Update(scc *api.SecurityContextConstraints) (*api.SecurityContextConstraints, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "update-securitycontextconstraints", Value: scc}, &api.SecurityContextConstraints{})
	return obj.(*api.SecurityContextConstraints), err
}

func (c *FakeSecurityContextConstraints) Delete(name string) error {
	_, err := c.Fake.Invokes(FakeAction{Action: "delete-securitycontextconstraints", Value: name}, &api.SecurityContextConstraints{})
	return err
}

func (c *FakeSecurityContextConstraints) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	c.Fake.Invokes(FakeAction{Action: "watch-securitycontextconstraints", Value: resourceVersion}, nil)
	return c.Fake.Watch, c.Fake.Err()
}

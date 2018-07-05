/*
Copyright 2018 Red Hat, Inc.

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

package idler

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	idling "github.com/openshift/service-idler/pkg/apis/idling/v1alpha2"
)

// CoWIdler is a copy-on-write abstraction over the idler object.
// It allows us to skip deepcopying unless we really need it.
type CoWIdler struct {
	original  *idling.Idler
	newStatus *idling.IdlerStatus
}

// NewCow (🐮) creates a new copy-on-write wrapper around the given idler.
func NewCoW(original *idling.Idler) *CoWIdler {
	return &CoWIdler{original: original}
}

// Status returns the most-recently-updated status
// of the idler.  It should be considered read-only.
func (c *CoWIdler) Status() *idling.IdlerStatus {
	if c.newStatus != nil {
		return c.newStatus
	}
	return &c.original.Status
}

// ObjectMeta returns the object metadata of the idler.
// It should be considered read-only.
func (c *CoWIdler) ObjectMeta() *metav1.ObjectMeta {
	return &c.original.ObjectMeta
}

// Spec returns the spec of the idler.
// It should be considered read-only.
func (c *CoWIdler) Spec() *idling.IdlerSpec {
	return &c.original.Spec
}

// WritableStatus returns a writable copy of the status.
// It will copy the current status.
func (c *CoWIdler) WritableStatus() *idling.IdlerStatus {
	return c.WritableStatusIf(true)
}

// WritableStatusIf returns a writable copy of the status
// if the given condition is true, and otherwise returns
// a read-only copy of the status.
func (c *CoWIdler) WritableStatusIf(cond bool) *idling.IdlerStatus {
	if !cond {
		return c.Status()
	}

	if c.newStatus == nil {
		c.newStatus = c.original.Status.DeepCopy()
	}
	return c.newStatus
}

// Full returns the most-recently-updated idler object.
// It will copy the idler if necessary, and reset the CoW-ness.
func (c *CoWIdler) Full() *idling.Idler {
	if c.newStatus == nil {
		return c.original
	}

	copiedIdler := c.original.DeepCopy()
	copiedIdler.Status = *c.newStatus

	c.newStatus = nil
	c.original = copiedIdler

	return c.original
}

// Updated returns whether or not a copy has been made.
func (c *CoWIdler) Updated() bool {
	return c.newStatus != nil
}

// UpdateIfNeeded calls the given function if the CoW idler has been updated,
// and not otherwise dealt-with via Full.
func (c *CoWIdler) UpdateIfNeeded(updateFunc func(u *idling.Idler) error) error {
	if !c.Updated() {
		return nil
	}
	fullObj := c.Full()
	return updateFunc(fullObj)
}

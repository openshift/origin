/*
Copyright 2021 The Kubernetes Authors.

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

package komega

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Komega is a collection of utilites for writing tests involving a mocked
// Kubernetes API.
type Komega interface {
	// Get returns a function that fetches a resource and returns the occurring error.
	// It can be used with gomega.Eventually() like this
	//   deployment := appsv1.Deployment{ ... }
	//   gomega.Eventually(k.Get(&deployment)).To(gomega.Succeed())
	// By calling the returned function directly it can also be used with gomega.Expect(k.Get(...)()).To(...)
	Get(client.Object) func() error

	// List returns a function that lists resources and returns the occurring error.
	// It can be used with gomega.Eventually() like this
	//   deployments := v1.DeploymentList{ ... }
	//   gomega.Eventually(k.List(&deployments)).To(gomega.Succeed())
	// By calling the returned function directly it can also be used as gomega.Expect(k.List(...)()).To(...)
	List(client.ObjectList, ...client.ListOption) func() error

	// Update returns a function that fetches a resource, applies the provided update function and then updates the resource.
	// It can be used with gomega.Eventually() like this:
	//   deployment := appsv1.Deployment{ ... }
	//   gomega.Eventually(k.Update(&deployment, func() {
	//     deployment.Spec.Replicas = 3
	//   })).To(gomega.Succeed())
	// By calling the returned function directly it can also be used as gomega.Expect(k.Update(...)()).To(...)
	Update(client.Object, func(), ...client.UpdateOption) func() error

	// UpdateStatus returns a function that fetches a resource, applies the provided update function and then updates the resource's status.
	// It can be used with gomega.Eventually() like this:
	//   deployment := appsv1.Deployment{ ... }
	//   gomega.Eventually(k.Update(&deployment, func() {
	//     deployment.Status.AvailableReplicas = 1
	//   })).To(gomega.Succeed())
	// By calling the returned function directly it can also be used as gomega.Expect(k.UpdateStatus(...)()).To(...)
	UpdateStatus(client.Object, func(), ...client.SubResourceUpdateOption) func() error

	// Object returns a function that fetches a resource and returns the object.
	// It can be used with gomega.Eventually() like this:
	//   deployment := appsv1.Deployment{ ... }
	//   gomega.Eventually(k.Object(&deployment)).To(HaveField("Spec.Replicas", gomega.Equal(ptr.To(int32(3)))))
	// By calling the returned function directly it can also be used as gomega.Expect(k.Object(...)()).To(...)
	Object(client.Object) func() (client.Object, error)

	// ObjectList returns a function that fetches a resource and returns the object.
	// It can be used with gomega.Eventually() like this:
	//   deployments := appsv1.DeploymentList{ ... }
	//   gomega.Eventually(k.ObjectList(&deployments)).To(HaveField("Items", HaveLen(1)))
	// By calling the returned function directly it can also be used as gomega.Expect(k.ObjectList(...)()).To(...)
	ObjectList(client.ObjectList, ...client.ListOption) func() (client.ObjectList, error)

	// WithContext returns a copy that uses the given context.
	WithContext(context.Context) Komega
}

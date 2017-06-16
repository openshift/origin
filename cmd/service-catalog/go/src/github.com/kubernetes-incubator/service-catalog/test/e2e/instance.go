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

package e2e

import (
	"time"

	v1alpha1 "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	"github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset"
	"github.com/kubernetes-incubator/service-catalog/test/e2e/framework"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	// how long to wait for an instance to be deleted.
	instanceDeleteTimeout = 30 * time.Second
)

func newTestInstance(name, serviceClassName, planName string) *v1alpha1.Instance {
	return &v1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.InstanceSpec{
			PlanName:         planName,
			ServiceClassName: serviceClassName,
		},
	}
}

// createInstance in the specified namespace
func createInstance(c clientset.Interface, namespace string, instance *v1alpha1.Instance) (*v1alpha1.Instance, error) {
	return c.ServicecatalogV1alpha1().Instances(namespace).Create(instance)
}

// deleteInstance with the specified namespace and name
func deleteInstance(c clientset.Interface, namespace, name string) error {
	return c.ServicecatalogV1alpha1().Instances(namespace).Delete(name, nil)
}

// waitForInstanceToBeDeleted waits for the instance to be removed.
func waitForInstanceToBeDeleted(c clientset.Interface, namespace, name string) error {
	return wait.Poll(framework.Poll, instanceDeleteTimeout, func() (bool, error) {
		_, err := c.ServicecatalogV1alpha1().Instances(namespace).Get(name, metav1.GetOptions{})
		if err == nil {
			framework.Logf("waiting for instance %s to be deleted", name)
			return false, nil
		}
		if errors.IsNotFound(err) {
			framework.Logf("verified instance %s is deleted", name)
			return true, nil
		}
		return false, err
	})
}

var _ = framework.ServiceCatalogDescribe("Instance", func() {
	f := framework.NewDefaultFramework("instance")

	It("should verify an Instance can be deleted if referenced service class does not exist.", func() {
		By("Creating an Instance")
		instance := newTestInstance("test-instance", "no-service-class", "no-plan")
		instance, err := createInstance(f.ServiceCatalogClientSet, f.Namespace.Name, instance)
		Expect(err).NotTo(HaveOccurred())
		By("Deleting the Instance")
		err = deleteInstance(f.ServiceCatalogClientSet, f.Namespace.Name, instance.Name)
		Expect(err).NotTo(HaveOccurred())
		err = waitForInstanceToBeDeleted(f.ServiceCatalogClientSet, f.Namespace.Name, instance.Name)
		Expect(err).NotTo(HaveOccurred())
	})
})

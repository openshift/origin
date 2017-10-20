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

package util

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	v1beta1servicecatalog "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset/typed/servicecatalog/v1beta1"
)

// WaitForBrokerCondition waits for the status of the named broker to contain
// a condition whose type and status matches the supplied one.
func WaitForBrokerCondition(client v1beta1servicecatalog.ServicecatalogV1beta1Interface, name string, condition v1beta1.ServiceBrokerCondition) error {
	return wait.PollImmediate(500*time.Millisecond, wait.ForeverTestTimeout,
		func() (bool, error) {
			glog.V(5).Infof("Waiting for broker %v condition %#v", name, condition)
			broker, err := client.ClusterServiceBrokers().Get(name, metav1.GetOptions{})
			if nil != err {
				return false, fmt.Errorf("error getting Broker %v: %v", name, err)
			}

			if len(broker.Status.Conditions) == 0 {
				return false, nil
			}

			for _, cond := range broker.Status.Conditions {
				if condition.Type == cond.Type && condition.Status == cond.Status {
					return true, nil
				}
			}

			return false, nil
		},
	)
}

// WaitForBrokerToNotExist waits for the Broker with the given name to no
// longer exist.
func WaitForBrokerToNotExist(client v1beta1servicecatalog.ServicecatalogV1beta1Interface, name string) error {
	return wait.PollImmediate(500*time.Millisecond, wait.ForeverTestTimeout,
		func() (bool, error) {
			glog.V(5).Infof("Waiting for broker %v to not exist", name)
			_, err := client.ClusterServiceBrokers().Get(name, metav1.GetOptions{})
			if nil == err {
				return false, nil
			}

			if errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		},
	)
}

// WaitForClusterServiceClassToExist waits for the ClusterServiceClass with the given name
// to exist.
func WaitForClusterServiceClassToExist(client v1beta1servicecatalog.ServicecatalogV1beta1Interface, name string) error {
	return wait.PollImmediate(500*time.Millisecond, wait.ForeverTestTimeout,
		func() (bool, error) {
			glog.V(5).Infof("Waiting for serviceClass %v to exist", name)
			_, err := client.ClusterServiceClasses().Get(name, metav1.GetOptions{})
			if nil == err {
				return true, nil
			}

			return false, nil
		},
	)
}

// WaitForClusterServiceClassToNotExist waits for the ClusterServiceClass with the given
// name to no longer exist.
func WaitForClusterServiceClassToNotExist(client v1beta1servicecatalog.ServicecatalogV1beta1Interface, name string) error {
	return wait.PollImmediate(500*time.Millisecond, wait.ForeverTestTimeout,
		func() (bool, error) {
			glog.V(5).Infof("Waiting for serviceClass %v to not exist", name)
			_, err := client.ClusterServiceClasses().Get(name, metav1.GetOptions{})
			if nil == err {
				return false, nil
			}

			if errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		},
	)
}

// WaitForInstanceCondition waits for the status of the named instance to
// contain a condition whose type and status matches the supplied one.
func WaitForInstanceCondition(client v1beta1servicecatalog.ServicecatalogV1beta1Interface, namespace, name string, condition v1beta1.ServiceInstanceCondition) error {
	return wait.PollImmediate(500*time.Millisecond, wait.ForeverTestTimeout,
		func() (bool, error) {
			glog.V(5).Infof("Waiting for instance %v/%v condition %#v", namespace, name, condition)
			instance, err := client.ServiceInstances(namespace).Get(name, metav1.GetOptions{})
			if nil != err {
				return false, fmt.Errorf("error getting Instance %v/%v: %v", namespace, name, err)
			}

			if len(instance.Status.Conditions) == 0 {
				return false, nil
			}

			for _, cond := range instance.Status.Conditions {
				if condition.Type == cond.Type && condition.Status == cond.Status {
					return true, nil
				}
			}

			return false, nil
		},
	)
}

// WaitForInstanceToNotExist waits for the Instance with the given name to no
// longer exist.
func WaitForInstanceToNotExist(client v1beta1servicecatalog.ServicecatalogV1beta1Interface, namespace, name string) error {
	return wait.PollImmediate(500*time.Millisecond, wait.ForeverTestTimeout,
		func() (bool, error) {
			glog.V(5).Infof("Waiting for instance %v/%v to not exist", namespace, name)

			_, err := client.ServiceInstances(namespace).Get(name, metav1.GetOptions{})
			if nil == err {
				return false, nil
			}

			if errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		},
	)
}

// WaitForInstanceReconciledGeneration waits for the status of the named instance to
// have the specified reconciled generation.
func WaitForInstanceReconciledGeneration(client v1beta1servicecatalog.ServicecatalogV1beta1Interface, namespace, name string, reconciledGeneration int64) error {
	return wait.PollImmediate(500*time.Millisecond, wait.ForeverTestTimeout,
		func() (bool, error) {
			glog.V(5).Infof("Waiting for instance %v/%v to have reconciled generation of %v", namespace, name, reconciledGeneration)
			instance, err := client.ServiceInstances(namespace).Get(name, metav1.GetOptions{})
			if nil != err {
				return false, fmt.Errorf("error getting Instance %v/%v: %v", namespace, name, err)
			}

			if instance.Status.ReconciledGeneration == reconciledGeneration {
				return true, nil
			}

			return false, nil
		},
	)
}

// WaitForBindingCondition waits for the status of the named binding to
// contain a condition whose type and status matches the supplied one.
func WaitForBindingCondition(client v1beta1servicecatalog.ServicecatalogV1beta1Interface, namespace, name string, condition v1beta1.ServiceBindingCondition) error {
	return wait.PollImmediate(500*time.Millisecond, wait.ForeverTestTimeout,
		func() (bool, error) {
			glog.V(5).Infof("Waiting for binding %v/%v condition %#v", namespace, name, condition)

			binding, err := client.ServiceBindings(namespace).Get(name, metav1.GetOptions{})
			if nil != err {
				return false, fmt.Errorf("error getting Binding %v/%v: %v", namespace, name, err)
			}

			if len(binding.Status.Conditions) == 0 {
				return false, nil
			}

			for _, cond := range binding.Status.Conditions {
				if condition.Type == cond.Type && condition.Status == cond.Status {
					return true, nil
				}
			}

			return false, nil
		},
	)
}

// WaitForBindingToNotExist waits for the Binding with the given name to no
// longer exist.
func WaitForBindingToNotExist(client v1beta1servicecatalog.ServicecatalogV1beta1Interface, namespace, name string) error {
	return wait.PollImmediate(500*time.Millisecond, wait.ForeverTestTimeout,
		func() (bool, error) {
			glog.V(5).Infof("Waiting for binding %v/%v to not exist", namespace, name)

			_, err := client.ServiceBindings(namespace).Get(name, metav1.GetOptions{})
			if nil == err {
				return false, nil
			}

			if errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		},
	)
}

// WaitForBindingReconciledGeneration waits for the status of the named binding to
// have the specified reconciled generation.
func WaitForBindingReconciledGeneration(client v1beta1servicecatalog.ServicecatalogV1beta1Interface, namespace, name string, reconciledGeneration int64) error {
	return wait.PollImmediate(500*time.Millisecond, wait.ForeverTestTimeout,
		func() (bool, error) {
			glog.V(5).Infof("Waiting for binding %v/%v to have reconciled generation of %v", namespace, name, reconciledGeneration)
			binding, err := client.ServiceBindings(namespace).Get(name, metav1.GetOptions{})
			if nil != err {
				return false, fmt.Errorf("error getting ServiceBinding %v/%v: %v", namespace, name, err)
			}

			if binding.Status.ReconciledGeneration == reconciledGeneration {
				return true, nil
			}

			return false, nil
		},
	)
}

// AssertServiceInstanceCondition asserts that the instance's status contains
// the given condition type, status, and reason.
func AssertServiceInstanceCondition(t *testing.T, instance *v1beta1.ServiceInstance, conditionType v1beta1.ServiceInstanceConditionType, status v1beta1.ConditionStatus, reason ...string) {
	foundCondition := false
	for _, condition := range instance.Status.Conditions {
		if condition.Type == conditionType {
			foundCondition = true
			if condition.Status != status {
				t.Fatalf("%v condition had unexpected status; expected %v, got %v", conditionType, status, condition.Status)
			}
			if len(reason) == 1 && condition.Reason != reason[0] {
				t.Fatalf("unexpected reason; expected %v, got %v", reason[0], condition.Reason)
			}
		}
	}

	if !foundCondition {
		t.Fatalf("%v condition not found", conditionType)
	}
}

// AssertServiceBindingCondition asserts that the binding's status contains
// the given condition type, status, and reason.
func AssertServiceBindingCondition(t *testing.T, binding *v1beta1.ServiceBinding, conditionType v1beta1.ServiceBindingConditionType, status v1beta1.ConditionStatus, reason ...string) {
	foundCondition := false
	for _, condition := range binding.Status.Conditions {
		if condition.Type == conditionType {
			foundCondition = true
			if condition.Status != status {
				t.Fatalf("%v condition had unexpected status; expected %v, got %v", conditionType, status, condition.Status)
			}
			if len(reason) == 1 && condition.Reason != reason[0] {
				t.Fatalf("unexpected reason; expected %v, got %v", reason[0], condition.Reason)
			}
		}
	}

	if !foundCondition {
		t.Fatalf("%v condition not found", conditionType)
	}
}

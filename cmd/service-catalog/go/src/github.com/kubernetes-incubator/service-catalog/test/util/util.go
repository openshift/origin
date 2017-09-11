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
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	v1alpha1servicecatalog "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset/typed/servicecatalog/v1alpha1"
)

// WaitForBrokerCondition waits for the status of the named broker to contain
// a condition whose type and status matches the supplied one.
func WaitForBrokerCondition(client v1alpha1servicecatalog.ServicecatalogV1alpha1Interface, name string, condition v1alpha1.ServiceBrokerCondition) error {
	return wait.PollImmediate(500*time.Millisecond, wait.ForeverTestTimeout,
		func() (bool, error) {
			glog.V(5).Infof("Waiting for broker %v condition %#v", name, condition)
			broker, err := client.ServiceBrokers().Get(name, metav1.GetOptions{})
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
func WaitForBrokerToNotExist(client v1alpha1servicecatalog.ServicecatalogV1alpha1Interface, name string) error {
	return wait.PollImmediate(500*time.Millisecond, wait.ForeverTestTimeout,
		func() (bool, error) {
			glog.V(5).Infof("Waiting for broker %v to not exist", name)
			_, err := client.ServiceBrokers().Get(name, metav1.GetOptions{})
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

// WaitForServiceClassToExist waits for the ServiceClass with the given name
// to exist.
func WaitForServiceClassToExist(client v1alpha1servicecatalog.ServicecatalogV1alpha1Interface, name string) error {
	return wait.PollImmediate(500*time.Millisecond, wait.ForeverTestTimeout,
		func() (bool, error) {
			glog.V(5).Infof("Waiting for serviceClass %v to exist", name)
			_, err := client.ServiceClasses().Get(name, metav1.GetOptions{})
			if nil == err {
				return true, nil
			}

			return false, nil
		},
	)
}

// WaitForServiceClassToNotExist waits for the ServiceClass with the given
// name to no longer exist.
func WaitForServiceClassToNotExist(client v1alpha1servicecatalog.ServicecatalogV1alpha1Interface, name string) error {
	return wait.PollImmediate(500*time.Millisecond, wait.ForeverTestTimeout,
		func() (bool, error) {
			glog.V(5).Infof("Waiting for serviceClass %v to not exist", name)
			_, err := client.ServiceClasses().Get(name, metav1.GetOptions{})
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
func WaitForInstanceCondition(client v1alpha1servicecatalog.ServicecatalogV1alpha1Interface, namespace, name string, condition v1alpha1.ServiceInstanceCondition) error {
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
func WaitForInstanceToNotExist(client v1alpha1servicecatalog.ServicecatalogV1alpha1Interface, namespace, name string) error {
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

// WaitForBindingCondition waits for the status of the named binding to
// contain a condition whose type and status matches the supplied one.
func WaitForBindingCondition(client v1alpha1servicecatalog.ServicecatalogV1alpha1Interface, namespace, name string, condition v1alpha1.ServiceInstanceCredentialCondition) error {
	return wait.PollImmediate(500*time.Millisecond, wait.ForeverTestTimeout,
		func() (bool, error) {
			glog.V(5).Infof("Waiting for binding %v/%v condition %#v", namespace, name, condition)

			binding, err := client.ServiceInstanceCredentials(namespace).Get(name, metav1.GetOptions{})
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
func WaitForBindingToNotExist(client v1alpha1servicecatalog.ServicecatalogV1alpha1Interface, namespace, name string) error {
	return wait.PollImmediate(500*time.Millisecond, wait.ForeverTestTimeout,
		func() (bool, error) {
			glog.V(5).Infof("Waiting for binding %v/%v to not exist", namespace, name)

			_, err := client.ServiceInstanceCredentials(namespace).Get(name, metav1.GetOptions{})
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

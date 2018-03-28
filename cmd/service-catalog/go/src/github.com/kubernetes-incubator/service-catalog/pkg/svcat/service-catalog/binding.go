/*
Copyright 2018 The Kubernetes Authors.

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

package servicecatalog

import (
	"fmt"
	"strings"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RetrieveBindings lists all bindings in a namespace.
func (sdk *SDK) RetrieveBindings(ns string) (*v1beta1.ServiceBindingList, error) {
	bindings, err := sdk.ServiceCatalog().ServiceBindings(ns).List(v1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to list bindings in %s (%s)", ns, err)
	}

	return bindings, nil
}

// RetrieveBinding gets a binding by its name.
func (sdk *SDK) RetrieveBinding(ns, name string) (*v1beta1.ServiceBinding, error) {
	binding, err := sdk.ServiceCatalog().ServiceBindings(ns).Get(name, v1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to get binding '%s.%s' (%+v)", ns, name, err)
	}
	return binding, nil
}

// RetrieveBindingsByInstance gets all child bindings for an instance.
func (sdk *SDK) RetrieveBindingsByInstance(instance *v1beta1.ServiceInstance,
) ([]v1beta1.ServiceBinding, error) {
	// Not using a filtered list operation because it's not supported yet.
	results, err := sdk.ServiceCatalog().ServiceBindings(instance.Namespace).List(v1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to search bindings (%s)", err)
	}

	var bindings []v1beta1.ServiceBinding
	for _, binding := range results.Items {
		if binding.Spec.ServiceInstanceRef.Name == instance.Name {
			bindings = append(bindings, binding)
		}
	}

	return bindings, nil
}

// Bind an instance to a secret.
func (sdk *SDK) Bind(namespace, bindingName, instanceName, secretName string,
	params map[string]string, secrets map[string]string) (*v1beta1.ServiceBinding, error) {

	// Manually defaulting the name of the binding
	// I'm not doing the same for the secret since the API handles defaulting that value.
	if bindingName == "" {
		bindingName = instanceName
	}

	request := &v1beta1.ServiceBinding{
		ObjectMeta: v1.ObjectMeta{
			Name:      bindingName,
			Namespace: namespace,
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{
				Name: instanceName,
			},
			SecretName:     secretName,
			Parameters:     BuildParameters(params),
			ParametersFrom: BuildParametersFrom(secrets),
		},
	}

	result, err := sdk.ServiceCatalog().ServiceBindings(namespace).Create(request)
	if err != nil {
		return nil, fmt.Errorf("bind request failed (%s)", err)
	}

	return result, nil
}

// Unbind deletes all bindings associated to an instance.
func (sdk *SDK) Unbind(ns, instanceName string) ([]v1beta1.ServiceBinding, error) {
	instance, err := sdk.RetrieveInstance(ns, instanceName)
	if err != nil {
		return nil, err
	}
	bindings, err := sdk.RetrieveBindingsByInstance(instance)
	if err != nil {
		return nil, err
	}
	var g sync.WaitGroup
	errs := make(chan error, len(bindings))
	deletedBindings := make(chan v1beta1.ServiceBinding, len(bindings))
	for _, binding := range bindings {
		g.Add(1)
		go func(binding v1beta1.ServiceBinding) {
			defer g.Done()
			err := sdk.DeleteBinding(binding.Namespace, binding.Name)
			if err == nil {
				deletedBindings <- binding
			}
			errs <- err
		}(binding)
	}

	g.Wait()
	close(errs)
	close(deletedBindings)

	// Collect any errors that occurred into a single formatted error
	bindErr := &multierror.Error{
		ErrorFormat: func(errors []error) string {
			return joinErrors("could not remove some bindings:", errors, "\n  ")
		},
	}
	for err := range errs {
		bindErr = multierror.Append(bindErr, err)
	}

	//Range over the deleted bindings to build a slice to return
	deleted := []v1beta1.ServiceBinding{}
	for b := range deletedBindings {
		deleted = append(deleted, b)
	}
	return deleted, bindErr.ErrorOrNil()
}

// DeleteBinding by name.
func (sdk *SDK) DeleteBinding(ns, bindingName string) error {
	err := sdk.ServiceCatalog().ServiceBindings(ns).Delete(bindingName, &v1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("remove binding %s/%s failed (%s)", ns, bindingName, err)
	}
	return nil
}

func joinErrors(groupMsg string, errors []error, sep string, a ...interface{}) string {
	if len(errors) == 0 {
		return ""
	}

	msgs := make([]string, 0, len(errors)+1)
	msgs = append(msgs, fmt.Sprintf(groupMsg, a...))
	for _, err := range errors {
		msgs = append(msgs, err.Error())
	}

	return strings.Join(msgs, sep)
}

// BindingParentHierarchy retrieves all ancestor resources of a binding.
func (sdk *SDK) BindingParentHierarchy(binding *v1beta1.ServiceBinding,
) (*v1beta1.ServiceInstance, *v1beta1.ClusterServiceClass, *v1beta1.ClusterServicePlan, *v1beta1.ClusterServiceBroker, error) {
	instance, err := sdk.RetrieveInstanceByBinding(binding)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	class, plan, err := sdk.InstanceToServiceClassAndPlan(instance)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	broker, err := sdk.RetrieveBrokerByClass(class)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return instance, class, plan, broker, nil
}

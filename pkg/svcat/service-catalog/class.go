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

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
)

const (
	// FieldExternalClassName is the jsonpath to a class's external name.
	FieldExternalClassName = "spec.externalName"
)

// RetrieveClasses lists all classes defined in the cluster.
func (sdk *SDK) RetrieveClasses() ([]v1beta1.ClusterServiceClass, error) {
	classes, err := sdk.ServiceCatalog().ClusterServiceClasses().List(v1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to list classes (%s)", err)
	}

	return classes.Items, nil
}

// RetrieveClassByName gets a class by its external name.
func (sdk *SDK) RetrieveClassByName(name string) (*v1beta1.ClusterServiceClass, error) {
	opts := v1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(FieldExternalClassName, name).String(),
	}
	searchResults, err := sdk.ServiceCatalog().ClusterServiceClasses().List(opts)
	if err != nil {
		return nil, fmt.Errorf("unable to search classes by name (%s)", err)
	}
	if len(searchResults.Items) == 0 {
		return nil, fmt.Errorf("class '%s' not found", name)
	}
	if len(searchResults.Items) > 1 {
		return nil, fmt.Errorf("more than one matching class found for '%s'", name)
	}
	return &searchResults.Items[0], nil
}

// RetrieveClassByID gets a class by its UUID.
func (sdk *SDK) RetrieveClassByID(uuid string) (*v1beta1.ClusterServiceClass, error) {
	class, err := sdk.ServiceCatalog().ClusterServiceClasses().Get(uuid, v1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to get class (%s)", err)
	}
	return class, nil
}

// RetrieveClassByPlan gets the class associated to a plan.
func (sdk *SDK) RetrieveClassByPlan(plan *v1beta1.ClusterServicePlan,
) (*v1beta1.ClusterServiceClass, error) {
	// Retrieve the class as well because plans don't have the external class name
	class, err := sdk.ServiceCatalog().ClusterServiceClasses().Get(plan.Spec.ClusterServiceClassRef.Name, v1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to get class (%s)", err)
	}

	return class, nil
}

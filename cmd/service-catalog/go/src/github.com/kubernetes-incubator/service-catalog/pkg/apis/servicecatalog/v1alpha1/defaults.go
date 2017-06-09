/*
Copyright 2016 The Kubernetes Authors.

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

package v1alpha1

import (
	"github.com/satori/go.uuid"
	"k8s.io/apimachinery/pkg/runtime"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	RegisterDefaults(scheme)
	return scheme.AddDefaultingFuncs(
		SetDefaults_InstanceSpec,
		SetDefaults_BindingSpec,
	)
}

func SetDefaults_InstanceSpec(spec *InstanceSpec) {
	if spec.ExternalID == "" {
		spec.ExternalID = uuid.NewV4().String()
	}
}

func SetDefaults_BindingSpec(spec *BindingSpec) {
	if spec.ExternalID == "" {
		spec.ExternalID = uuid.NewV4().String()
	}
}

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

package apiserver

import (
	servicecatalogv1beta1 "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	genericapiserver "k8s.io/apiserver/pkg/server"
	serverstorage "k8s.io/apiserver/pkg/server/storage"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	settingsv1alpha1 "github.com/kubernetes-incubator/service-catalog/pkg/apis/settings/v1alpha1"
	"github.com/kubernetes-incubator/service-catalog/pkg/features"
)

// ServiceCatalogAPIServer contains the base GenericAPIServer along with other
// configured runtime configuration
type ServiceCatalogAPIServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

// PrepareRun prepares s to run. The returned value represents the runnable server
func (s ServiceCatalogAPIServer) PrepareRun() RunnableServer {
	return s.GenericAPIServer.PrepareRun()
}

// DefaultAPIResourceConfigSource returns a default API Resource config source
func DefaultAPIResourceConfigSource() *serverstorage.ResourceConfig {
	ret := serverstorage.NewResourceConfig()
	versions := []schema.GroupVersion{servicecatalogv1beta1.SchemeGroupVersion}

	if utilfeature.DefaultFeatureGate.Enabled(features.PodPreset) {
		versions = append(versions, settingsv1alpha1.SchemeGroupVersion)
	}
	ret.EnableVersions(versions...)

	return ret
}

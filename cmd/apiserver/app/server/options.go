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

package server

import (
	"os"

	"github.com/kubernetes-incubator/service-catalog/pkg/registry/servicecatalog/server"
	"github.com/spf13/pflag"
	genericserveroptions "k8s.io/apiserver/pkg/server/options"
)

// ServiceCatalogServerOptions contains the aggregation of configuration structs for
// the service-catalog server. It contains everything needed to configure a basic API server.
// It is public so that integration tests can access it.
type ServiceCatalogServerOptions struct {
	StorageTypeString string
	// the runtime configuration of our server
	GenericServerRunOptions *genericserveroptions.ServerRunOptions
	// the https configuration. certs, etc
	SecureServingOptions *genericserveroptions.SecureServingOptions
	// authn for the API
	AuthenticationOptions *genericserveroptions.DelegatingAuthenticationOptions
	// authz for the API
	AuthorizationOptions *genericserveroptions.DelegatingAuthorizationOptions
	// InsecureOptions are options for serving insecurely.
	InsecureServingOptions *genericserveroptions.ServingOptions
	// audit options for api server
	AuditOptions *genericserveroptions.AuditLogOptions
	// EtcdOptions are options for serving with etcd as the backing store
	EtcdOptions *EtcdOptions
	// TPROptions are options for serving with TPR as the backing store
	TPROptions *TPROptions
	// DisableAuth disables delegating authentication and authorization for testing scenarios
	DisableAuth bool
	StopCh      <-chan struct{}
	// StandaloneMode if true asserts that we will not depend on a kube-apiserver
	StandaloneMode bool
}

func (s *ServiceCatalogServerOptions) addFlags(flags *pflag.FlagSet) {
	flags.StringVar(
		&s.StorageTypeString,
		"storage-type",
		"etcd",
		"The type of backing storage this API server should use",
	)

	flags.BoolVar(
		&s.DisableAuth,
		"disable-auth",
		false,
		"Disable authentication and authorization for testing purposes",
	)

	s.GenericServerRunOptions.AddUniversalFlags(flags)
	s.SecureServingOptions.AddFlags(flags)
	s.AuthenticationOptions.AddFlags(flags)
	s.AuthorizationOptions.AddFlags(flags)
	s.InsecureServingOptions.AddFlags(flags)
	s.EtcdOptions.addFlags(flags)
	s.TPROptions.addFlags(flags)
	s.AuditOptions.AddFlags(flags)
}

// StorageType returns the storage type configured on s, or a non-nil error if s holds an
// invalid storage type
func (s *ServiceCatalogServerOptions) StorageType() (server.StorageType, error) {
	return server.StorageTypeFromString(s.StorageTypeString)
}

// standaloneMode returns true if the env var SERVICE_CATALOG_STANALONE=true
// If enabled, we will assume no integration with Kubernetes API server is performed.
// It is intended for testing purposes only.
func standaloneMode() bool {
	val := os.Getenv("SERVICE_CATALOG_STANDALONE")
	return val == "true"
}

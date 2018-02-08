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

// The controller is responsible for running control loops that reconcile
// the state of service catalog API resources with service brokers, service
// classes, service instances, and service instance credentials.

package options

import (
	"time"

	"github.com/spf13/pflag"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/componentconfig"
	k8scomponentconfig "github.com/kubernetes-incubator/service-catalog/pkg/kubernetes/pkg/apis/componentconfig"
	"github.com/kubernetes-incubator/service-catalog/pkg/kubernetes/pkg/client/leaderelectionconfig"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
)

// ControllerManagerServer is the main context object for the controller
// manager.
type ControllerManagerServer struct {
	componentconfig.ControllerManagerConfiguration
}

const (
	defaultResyncInterval                         = 5 * time.Minute
	defaultServiceBrokerRelistInterval            = 24 * time.Hour
	defaultContentType                            = "application/json"
	defaultBindAddress                            = "0.0.0.0"
	defaultPort                                   = 10000
	defaultK8sKubeconfigPath                      = "./kubeconfig"
	defaultServiceCatalogKubeconfigPath           = "./service-catalog-kubeconfig"
	defaultOSBAPIContextProfile                   = true
	defaultConcurrentSyncs                        = 5
	defaultLeaderElectionNamespace                = "kube-system"
	defaultReconciliationRetryDuration            = 7 * 24 * time.Hour
	defaultOperationPollingMaximumBackoffDuration = 20 * time.Minute
)

var defaultOSBAPIPreferredVersion = osb.LatestAPIVersion().HeaderValue()

// NewControllerManagerServer creates a new ControllerManagerServer with a
// default config.
func NewControllerManagerServer() *ControllerManagerServer {
	s := ControllerManagerServer{
		ControllerManagerConfiguration: componentconfig.ControllerManagerConfiguration{
			Address:                                defaultBindAddress,
			Port:                                   defaultPort,
			ContentType:                            defaultContentType,
			K8sKubeconfigPath:                      defaultK8sKubeconfigPath,
			ServiceCatalogKubeconfigPath:           defaultServiceCatalogKubeconfigPath,
			ResyncInterval:                         defaultResyncInterval,
			ServiceBrokerRelistInterval:            defaultServiceBrokerRelistInterval,
			OSBAPIContextProfile:                   defaultOSBAPIContextProfile,
			OSBAPIPreferredVersion:                 defaultOSBAPIPreferredVersion,
			ConcurrentSyncs:                        defaultConcurrentSyncs,
			LeaderElection:                         leaderelectionconfig.DefaultLeaderElectionConfiguration(),
			LeaderElectionNamespace:                defaultLeaderElectionNamespace,
			EnableProfiling:                        true,
			EnableContentionProfiling:              false,
			ReconciliationRetryDuration:            defaultReconciliationRetryDuration,
			OperationPollingMaximumBackoffDuration: defaultOperationPollingMaximumBackoffDuration,
		},
	}
	s.LeaderElection.LeaderElect = true
	return &s
}

// AddFlags adds flags for a ControllerManagerServer to the specified FlagSet.
func (s *ControllerManagerServer) AddFlags(fs *pflag.FlagSet) {
	fs.Var(k8scomponentconfig.IPVar{Val: &s.Address}, "address", "The IP address to serve on (set to 0.0.0.0 for all interfaces)")
	fs.Int32Var(&s.Port, "port", s.Port, "The port that the controller-manager's http service runs on")
	fs.StringVar(&s.ContentType, "api-content-type", s.ContentType, "Content type of requests sent to API servers")
	fs.StringVar(&s.K8sAPIServerURL, "k8s-api-server-url", "", "The URL for the k8s API server")
	fs.StringVar(&s.K8sKubeconfigPath, "k8s-kubeconfig", "", "Path to k8s core kubeconfig")
	fs.StringVar(&s.ServiceCatalogAPIServerURL, "service-catalog-api-server-url", "", "The URL for the service-catalog API server")
	fs.StringVar(&s.ServiceCatalogKubeconfigPath, "service-catalog-kubeconfig", "", "Path to service-catalog kubeconfig")
	fs.BoolVar(&s.ServiceCatalogInsecureSkipVerify, "service-catalog-insecure-skip-verify", s.ServiceCatalogInsecureSkipVerify, "Skip verification of the TLS certificate for the service-catalog API server")
	fs.DurationVar(&s.ResyncInterval, "resync-interval", s.ResyncInterval, "The interval on which the controller will resync its informers")
	fs.DurationVar(&s.ServiceBrokerRelistInterval, "broker-relist-interval", s.ServiceBrokerRelistInterval, "The interval on which a broker's catalog is relisted after the broker becomes ready")
	fs.BoolVar(&s.OSBAPIContextProfile, "enable-osb-api-context-profile", s.OSBAPIContextProfile, "This does nothing.")
	fs.MarkHidden("enable-osb-api-context-profile")
	fs.StringVar(&s.OSBAPIPreferredVersion, "osb-api-preferred-version", s.OSBAPIPreferredVersion, "The string to send as the version header.")
	fs.BoolVar(&s.EnableProfiling, "profiling", s.EnableProfiling, "Enable profiling via web interface host:port/debug/pprof/")
	fs.BoolVar(&s.EnableContentionProfiling, "contention-profiling", s.EnableContentionProfiling, "Enable lock contention profiling, if profiling is enabled")
	leaderelectionconfig.BindFlags(&s.LeaderElection, fs)
	fs.StringVar(&s.LeaderElectionNamespace, "leader-election-namespace", s.LeaderElectionNamespace, "Namespace to use for leader election lock")
	fs.DurationVar(&s.ReconciliationRetryDuration, "reconciliation-retry-duration", s.ReconciliationRetryDuration, "The maximum amount of time to retry reconciliations on a resource before failing")
	fs.DurationVar(&s.OperationPollingMaximumBackoffDuration, "operation-polling-maximum-backoff-duration", s.OperationPollingMaximumBackoffDuration, "The maximum amount of time to back-off while polling an OSB API operation")

	utilfeature.DefaultFeatureGate.AddFlag(fs)
}

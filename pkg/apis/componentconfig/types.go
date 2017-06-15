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

// The controller is responsible for running control loops that reconcile
// the state of service catalog API resources with service brokers, service
// classes, service instances, and service bindings.

package componentconfig

import (
	"time"

	"k8s.io/kubernetes/pkg/apis/componentconfig"
)

// ControllerManagerConfiguration encapsulates configuration for the
// controller manager.
type ControllerManagerConfiguration struct {
	// Address is the IP address to serve on (set to 0.0.0.0 for all interfaces).
	Address string
	// Port is the port that the controller's http service runs on.
	Port int32

	// ContentType is the content type for requests sent to API servers.
	ContentType string

	// kubeAPIQPS is the QPS to use while talking with kubernetes apiserver.
	KubeAPIQPS float32
	// kubeAPIBurst is the burst to use while talking with kubernetes apiserver.
	KubeAPIBurst int32

	// K8sAPIServerURL is the URL for the k8s API server.
	K8sAPIServerURL string
	// K8sKubeconfigPath is the path to the kubeconfig file with authorization
	// information.
	K8sKubeconfigPath string

	// ServiceCatalogAPIServerURL is the URL for the service-catalog API
	// server.
	ServiceCatalogAPIServerURL string
	// ServiceCatalogKubeconfigPath is the path to the kubeconfig file with
	// information about the service catalog API server.
	ServiceCatalogKubeconfigPath string

	// ResyncInterval is the interval on which the controller should re-sync
	// all informers.
	ResyncInterval time.Duration

	// BrokerRelistInterval is the interval on which Broker's catalogs are re-
	// listed.
	BrokerRelistInterval time.Duration

	// Whether or not to send the proposed optional
	// OpenServiceBroker API Context Profile field
	OSBAPIContextProfile bool

	// ConcurrentSyncs is the number of resources, per resource type,
	// that are allowed to sync concurrently. Larger number = more responsive
	// SC operations, but more CPU (and network) load.
	ConcurrentSyncs int

	// leaderElection defines the configuration of leader election client.
	LeaderElection componentconfig.LeaderElectionConfiguration

	// LeaderElectionNamespace is the namespace to use for the leader election
	// lock.
	LeaderElectionNamespace string

	// enableProfiling enables profiling via web interface host:port/debug/pprof/
	EnableProfiling bool

	// enableContentionProfiling enables lock contention profiling, if enableProfiling is true.
	EnableContentionProfiling bool
}

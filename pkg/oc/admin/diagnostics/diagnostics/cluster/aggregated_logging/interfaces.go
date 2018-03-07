package aggregated_logging

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapisext "k8s.io/kubernetes/pkg/apis/extensions"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	authapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	routesapi "github.com/openshift/origin/pkg/route/apis/route"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
)

//diagnosticReporter provides diagnostic messages
type diagnosticReporter interface {
	Info(id string, message string)
	Debug(id string, message string)
	Error(id string, err error, message string)
	Warn(id string, err error, message string)
}

type routesAdapter interface {
	routes(project string, options metav1.ListOptions) (*routesapi.RouteList, error)
}

type sccAdapter interface {
	getScc(name string) (*securityapi.SecurityContextConstraints, error)
}

type clusterRoleBindingsAdapter interface {
	listClusterRoleBindings() (*authapi.ClusterRoleBindingList, error)
}

//deploymentConfigAdapter is an abstraction to retrieve resource for validating dcs
//for aggregated logging diagnostics
type deploymentConfigAdapter interface {
	deploymentconfigs(project string, options metav1.ListOptions) (*appsapi.DeploymentConfigList, error)
	podsAdapter
}

//daemonsetAdapter is an abstraction to retrieve resources for validating daemonsets
//for aggregated logging diagnostics
type daemonsetAdapter interface {
	daemonsets(project string, options metav1.ListOptions) (*kapisext.DaemonSetList, error)
	nodes(options metav1.ListOptions) (*kapi.NodeList, error)
	podsAdapter
}

type podsAdapter interface {
	pods(project string, options metav1.ListOptions) (*kapi.PodList, error)
}

//saAdapter abstractions to retrieve service accounts
type saAdapter interface {
	serviceAccounts(project string, options metav1.ListOptions) (*kapi.ServiceAccountList, error)
}

//servicesAdapter abstracts retrieving services
type servicesAdapter interface {
	services(project string, options metav1.ListOptions) (*kapi.ServiceList, error)
	endpointsForService(project string, serviceName string) (*kapi.Endpoints, error)
}

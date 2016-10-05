package aggregated_logging

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kapisext "k8s.io/kubernetes/pkg/apis/extensions"

	authapi "github.com/openshift/origin/pkg/authorization/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	routesapi "github.com/openshift/origin/pkg/route/api"
)

//diagnosticReporter provides diagnostic messages
type diagnosticReporter interface {
	Info(id string, message string)
	Debug(id string, message string)
	Error(id string, err error, message string)
	Warn(id string, err error, message string)
}

type routesAdapter interface {
	routes(project string, options kapi.ListOptions) (*routesapi.RouteList, error)
}

type sccAdapter interface {
	getScc(name string) (*kapi.SecurityContextConstraints, error)
}

type clusterRoleBindingsAdapter interface {
	getClusterRoleBinding(name string) (*authapi.ClusterRoleBinding, error)
}

//deploymentConfigAdapter is an abstraction to retrieve resource for validating dcs
//for aggregated logging diagnostics
type deploymentConfigAdapter interface {
	deploymentconfigs(project string, options kapi.ListOptions) (*deployapi.DeploymentConfigList, error)
	podsAdapter
}

//daemonsetAdapter is an abstraction to retrieve resources for validating daemonsets
//for aggregated logging diagnostics
type daemonsetAdapter interface {
	daemonsets(project string, options kapi.ListOptions) (*kapisext.DaemonSetList, error)
	nodes(options kapi.ListOptions) (*kapi.NodeList, error)
	podsAdapter
}

type podsAdapter interface {
	pods(project string, options kapi.ListOptions) (*kapi.PodList, error)
}

//saAdapter abstractions to retrieve service accounts
type saAdapter interface {
	serviceAccounts(project string, options kapi.ListOptions) (*kapi.ServiceAccountList, error)
}

//servicesAdapter abstracts retrieving services
type servicesAdapter interface {
	services(project string, options kapi.ListOptions) (*kapi.ServiceList, error)
	endpointsForService(project string, serviceName string) (*kapi.Endpoints, error)
}

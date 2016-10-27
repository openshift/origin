package aggregated_logging

import (
	"errors"
	"fmt"
	"net/url"

	kapi "k8s.io/kubernetes/pkg/api"
	kapisext "k8s.io/kubernetes/pkg/apis/extensions"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/labels"

	authapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	hostdiag "github.com/openshift/origin/pkg/diagnostics/host"
	"github.com/openshift/origin/pkg/diagnostics/types"
	routesapi "github.com/openshift/origin/pkg/route/api"
)

// AggregatedLogging is a Diagnostic to check the configurations
// and general integration of the OpenShift stack
// for aggregating container logs
// https://github.com/openshift/origin-aggregated-logging
type AggregatedLogging struct {
	masterConfig     *configapi.MasterConfig
	MasterConfigFile string
	OsClient         *client.Client
	KubeClient       *kclient.Client
	result           types.DiagnosticResult
}

const (
	AggregatedLoggingName = "AggregatedLogging"

	loggingInfraKey = "logging-infra"
	componentKey    = "component"
	providerKey     = "provider"
	openshiftValue  = "openshift"

	fluentdServiceAccountName = "aggregated-logging-fluentd"
)

var loggingSelector = labels.Set{loggingInfraKey: "support"}

//NewAggregatedLogging returns the AggregatedLogging Diagnostic
func NewAggregatedLogging(masterConfigFile string, kclient *kclient.Client, osclient *client.Client) *AggregatedLogging {
	return &AggregatedLogging{nil, masterConfigFile, osclient, kclient, types.NewDiagnosticResult(AggregatedLoggingName)}
}

func (d *AggregatedLogging) getScc(name string) (*kapi.SecurityContextConstraints, error) {
	return d.KubeClient.SecurityContextConstraints().Get(name)
}

func (d *AggregatedLogging) getClusterRoleBinding(name string) (*authapi.ClusterRoleBinding, error) {
	return d.OsClient.ClusterRoleBindings().Get(name)
}

func (d *AggregatedLogging) routes(project string, options kapi.ListOptions) (*routesapi.RouteList, error) {
	return d.OsClient.Routes(project).List(options)
}

func (d *AggregatedLogging) serviceAccounts(project string, options kapi.ListOptions) (*kapi.ServiceAccountList, error) {
	return d.KubeClient.ServiceAccounts(project).List(options)
}

func (d *AggregatedLogging) services(project string, options kapi.ListOptions) (*kapi.ServiceList, error) {
	return d.KubeClient.Services(project).List(options)
}

func (d *AggregatedLogging) endpointsForService(project string, service string) (*kapi.Endpoints, error) {
	return d.KubeClient.Endpoints(project).Get(service)
}

func (d *AggregatedLogging) daemonsets(project string, options kapi.ListOptions) (*kapisext.DaemonSetList, error) {
	return d.KubeClient.DaemonSets(project).List(kapi.ListOptions{LabelSelector: loggingInfraFluentdSelector.AsSelector()})
}

func (d *AggregatedLogging) nodes(options kapi.ListOptions) (*kapi.NodeList, error) {
	return d.KubeClient.Nodes().List(kapi.ListOptions{})
}

func (d *AggregatedLogging) pods(project string, options kapi.ListOptions) (*kapi.PodList, error) {
	return d.KubeClient.Pods(project).List(options)
}
func (d *AggregatedLogging) deploymentconfigs(project string, options kapi.ListOptions) (*deployapi.DeploymentConfigList, error) {
	return d.OsClient.DeploymentConfigs(project).List(options)
}

func (d *AggregatedLogging) Info(id string, message string) {
	d.result.Info(id, message)
}

func (d *AggregatedLogging) Error(id string, err error, message string) {
	d.result.Error(id, err, message)
}

func (d *AggregatedLogging) Debug(id string, message string) {
	d.result.Debug(id, message)
}

func (d *AggregatedLogging) Warn(id string, err error, message string) {
	d.result.Warn(id, err, message)
}

func (d *AggregatedLogging) Name() string {
	return AggregatedLoggingName
}

func (d *AggregatedLogging) Description() string {
	return "Check aggregated logging integration for proper configuration"
}

func (d *AggregatedLogging) CanRun() (bool, error) {
	if len(d.MasterConfigFile) == 0 {
		return false, errors.New("No master config file was provided")
	}
	if d.OsClient == nil {
		return false, errors.New("Config must include a cluster-admin context to run this diagnostic")
	}
	if d.KubeClient == nil {
		return false, errors.New("Config must include a cluster-admin context to run this diagnostic")
	}
	var err error
	d.masterConfig, err = hostdiag.GetMasterConfig(d.result, d.MasterConfigFile)
	if err != nil {
		return false, errors.New("Unreadable master config; skipping this diagnostic.")
	}
	return true, nil
}

func (d *AggregatedLogging) Check() types.DiagnosticResult {
	project := retrieveLoggingProject(d.result, d.masterConfig, d.OsClient)
	if len(project) != 0 {
		checkServiceAccounts(d, d, project)
		checkClusterRoleBindings(d, d, project)
		checkSccs(d, d, project)
		checkDeploymentConfigs(d, d, project)
		checkDaemonSets(d, d, project)
		checkServices(d, d, project)
		checkRoutes(d, d, project)
		checkKibana(d.result, d.OsClient, d.KubeClient, project)
	}
	return d.result
}

const projectNodeSelectorWarning = `
The project '%[1]s' was found with either a missing or non-empty node selector annotation.  
This could keep Fluentd from running on certain nodes and collecting logs from the entire cluster.  
You can correct it by editing the project:

  oc edit namespace %[1]s

and updating the annotation:

  'openshift.io/node-selector' : ""

`

func retrieveLoggingProject(r types.DiagnosticResult, masterCfg *configapi.MasterConfig, osClient *client.Client) string {
	r.Debug("AGL0010", fmt.Sprintf("masterConfig.AssetConfig.LoggingPublicURL: '%s'", masterCfg.AssetConfig.LoggingPublicURL))
	projectName := ""
	if len(masterCfg.AssetConfig.LoggingPublicURL) == 0 {
		r.Debug("AGL0017", "masterConfig.AssetConfig.LoggingPublicURL is empty")
		return projectName
	}

	loggingUrl, err := url.Parse(masterCfg.AssetConfig.LoggingPublicURL)
	if err != nil {
		r.Error("AGL0011", err, fmt.Sprintf("Unable to parse the loggingPublicURL from the masterConfig '%s'", masterCfg.AssetConfig.LoggingPublicURL))
		return projectName
	}

	routeList, err := osClient.Routes(kapi.NamespaceAll).List(kapi.ListOptions{LabelSelector: loggingSelector.AsSelector()})
	if err != nil {
		r.Error("AGL0012", err, fmt.Sprintf("There was an error while trying to find the route associated with '%s' which is probably transient: %s", loggingUrl, err))
		return projectName
	}

	for _, route := range routeList.Items {
		r.Debug("AGL0013", fmt.Sprintf("Comparing URL to route.Spec.Host: %s", route.Spec.Host))
		if loggingUrl.Host == route.Spec.Host {
			if len(projectName) == 0 {
				projectName = route.ObjectMeta.Namespace
				r.Info("AGL0015", fmt.Sprintf("Found route '%s' matching logging URL '%s' in project: '%s'", route.ObjectMeta.Name, loggingUrl.Host, projectName))
			} else {
				r.Warn("AGL0019", nil, fmt.Sprintf("Found additional route '%s' matching logging URL '%s' in project: '%s'.  This could mean you have multiple logging deployments.", route.ObjectMeta.Name, loggingUrl.Host, projectName))
			}
		}
	}
	if len(projectName) == 0 {
		message := fmt.Sprintf("Unable to find a route matching the loggingPublicURL defined in the master config '%s'. Check that the URL is correct and aggregated logging is deployed.", loggingUrl)
		r.Error("AGL0014", errors.New(message), message)
		return ""
	}
	project, err := osClient.Projects().Get(projectName)
	if err != nil {
		r.Error("AGL0018", err, fmt.Sprintf("There was an error retrieving project '%s' which is most likely a transient error: %s", projectName, err))
		return ""
	}
	nodeSelector, ok := project.ObjectMeta.Annotations["openshift.io/node-selector"]
	if !ok || len(nodeSelector) != 0 {
		r.Warn("AGL0030", nil, fmt.Sprintf(projectNodeSelectorWarning, projectName))
	}
	return projectName
}

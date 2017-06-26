package aggregated_logging

import (
	"errors"
	"fmt"
	"net/url"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kapi "k8s.io/kubernetes/pkg/api"
	kapisext "k8s.io/kubernetes/pkg/apis/extensions"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	authapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
	hostdiag "github.com/openshift/origin/pkg/diagnostics/host"
	"github.com/openshift/origin/pkg/diagnostics/types"
	routesapi "github.com/openshift/origin/pkg/route/apis/route"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	"github.com/openshift/origin/pkg/security/legacyclient"
)

// AggregatedLogging is a Diagnostic to check the configurations
// and general integration of the OpenShift stack
// for aggregating container logs
// https://github.com/openshift/origin-aggregated-logging
type AggregatedLogging struct {
	masterConfig     *configapi.MasterConfig
	MasterConfigFile string
	OsClient         *client.Client
	KubeClient       kclientset.Interface
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
func NewAggregatedLogging(masterConfigFile string, kclient kclientset.Interface, osclient *client.Client) *AggregatedLogging {
	return &AggregatedLogging{nil, masterConfigFile, osclient, kclient, types.NewDiagnosticResult(AggregatedLoggingName)}
}

func (d *AggregatedLogging) getScc(name string) (*securityapi.SecurityContextConstraints, error) {
	return legacyclient.NewFromClient(d.KubeClient.Core().RESTClient()).Get(name, metav1.GetOptions{})
}

func (d *AggregatedLogging) getClusterRoleBinding(name string) (*authapi.ClusterRoleBinding, error) {
	return d.OsClient.ClusterRoleBindings().Get(name, metav1.GetOptions{})
}

func (d *AggregatedLogging) routes(project string, options metav1.ListOptions) (*routesapi.RouteList, error) {
	return d.OsClient.Routes(project).List(options)
}

func (d *AggregatedLogging) serviceAccounts(project string, options metav1.ListOptions) (*kapi.ServiceAccountList, error) {
	return d.KubeClient.Core().ServiceAccounts(project).List(options)
}

func (d *AggregatedLogging) services(project string, options metav1.ListOptions) (*kapi.ServiceList, error) {
	return d.KubeClient.Core().Services(project).List(options)
}

func (d *AggregatedLogging) endpointsForService(project string, service string) (*kapi.Endpoints, error) {
	return d.KubeClient.Core().Endpoints(project).Get(service, metav1.GetOptions{})
}

func (d *AggregatedLogging) daemonsets(project string, options metav1.ListOptions) (*kapisext.DaemonSetList, error) {
	return d.KubeClient.Extensions().DaemonSets(project).List(metav1.ListOptions{LabelSelector: loggingInfraFluentdSelector.AsSelector().String()})
}

func (d *AggregatedLogging) nodes(options metav1.ListOptions) (*kapi.NodeList, error) {
	return d.KubeClient.Core().Nodes().List(metav1.ListOptions{})
}

func (d *AggregatedLogging) pods(project string, options metav1.ListOptions) (*kapi.PodList, error) {
	return d.KubeClient.Core().Pods(project).List(options)
}
func (d *AggregatedLogging) deploymentconfigs(project string, options metav1.ListOptions) (*deployapi.DeploymentConfigList, error) {
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
		return false, errors.New("Master configuration is unreadable")
	}
	if d.masterConfig.AssetConfig.LoggingPublicURL == "" {
		return false, errors.New("No LoggingPublicURL is defined in the master configuration")
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

  $ oc edit namespace %[1]s

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

	routeList, err := osClient.Routes(metav1.NamespaceAll).List(metav1.ListOptions{LabelSelector: loggingSelector.AsSelector().String()})
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
	project, err := osClient.Projects().Get(projectName, metav1.GetOptions{})
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

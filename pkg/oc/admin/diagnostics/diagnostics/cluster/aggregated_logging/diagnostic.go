package aggregated_logging

import (
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapisext "k8s.io/kubernetes/pkg/apis/extensions"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appstypedclient "github.com/openshift/origin/pkg/apps/generated/internalclientset/typed/apps/internalversion"
	authapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	oauthorizationtypedclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset/typed/authorization/internalversion"
	oauthtypedclient "github.com/openshift/origin/pkg/oauth/generated/internalclientset/typed/oauth/internalversion"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/log"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/types"
	projecttypedclient "github.com/openshift/origin/pkg/project/generated/internalclientset/typed/project/internalversion"
	routesapi "github.com/openshift/origin/pkg/route/apis/route"
	routetypedclient "github.com/openshift/origin/pkg/route/generated/internalclientset/typed/route/internalversion"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	securitytypedclient "github.com/openshift/origin/pkg/security/generated/internalclientset/typed/security/internalversion"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"strings"
)

// AggregatedLogging is a Diagnostic to check the configurations
// and general integration of the OpenShift stack
// for aggregating container logs
// https://github.com/openshift/origin-aggregated-logging
type AggregatedLogging struct {
	Project           string
	OAuthClientClient oauthtypedclient.OAuthClientsGetter
	ProjectClient     projecttypedclient.ProjectsGetter
	RouteClient       routetypedclient.RoutesGetter
	CRBClient         oauthorizationtypedclient.ClusterRoleBindingsGetter
	DCClient          appstypedclient.DeploymentConfigsGetter
	SCCClient         securitytypedclient.SecurityContextConstraintsGetter
	KubeClient        kclientset.Interface
	result            types.DiagnosticResult
}

const (
	AggregatedLoggingName = "AggregatedLogging"

	loggingInfraKey = "logging-infra"
	componentKey    = "component"
	providerKey     = "provider"
	openshiftValue  = "openshift"

	fluentdServiceAccountName = "aggregated-logging-fluentd"

	flagLoggingProject = "logging-project"
)

var loggingSelector = labels.Set{loggingInfraKey: "support"}
var defaultLoggingProjects = []string{"openshift-logging", "logging"}

//NewAggregatedLogging returns the AggregatedLogging Diagnostic
func NewAggregatedLogging(
	project string,
	kclient kclientset.Interface,
	oauthClientClient oauthtypedclient.OAuthClientsGetter,
	projectClient projecttypedclient.ProjectsGetter,
	routeClient routetypedclient.RoutesGetter,
	crbClient oauthorizationtypedclient.ClusterRoleBindingsGetter,
	dcClient appstypedclient.DeploymentConfigsGetter,
	sccClient securitytypedclient.SecurityContextConstraintsGetter,
) *AggregatedLogging {
	return &AggregatedLogging{
		Project:           project,
		OAuthClientClient: oauthClientClient,
		ProjectClient:     projectClient,
		RouteClient:       routeClient,
		CRBClient:         crbClient,
		DCClient:          dcClient,
		SCCClient:         sccClient,
		KubeClient:        kclient,
		result:            types.NewDiagnosticResult(AggregatedLoggingName),
	}
}

func (d *AggregatedLogging) getScc(name string) (*securityapi.SecurityContextConstraints, error) {
	return d.SCCClient.SecurityContextConstraints().Get(name, metav1.GetOptions{})
}

func (d *AggregatedLogging) listClusterRoleBindings() (*authapi.ClusterRoleBindingList, error) {
	return d.CRBClient.ClusterRoleBindings().List(metav1.ListOptions{})
}

func (d *AggregatedLogging) routes(project string, options metav1.ListOptions) (*routesapi.RouteList, error) {
	return d.RouteClient.Routes(project).List(options)
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
func (d *AggregatedLogging) deploymentconfigs(project string, options metav1.ListOptions) (*appsapi.DeploymentConfigList, error) {
	return d.DCClient.DeploymentConfigs(project).List(options)
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

func (d *AggregatedLogging) Requirements() (client bool, host bool) {
	return true, false
}

func (d *AggregatedLogging) Complete(logger *log.Logger) error {
	if len(d.Project) > 0 {
		return nil
	}

	// Check if any of the default logging projects are present in the cluster
	for _, project := range defaultLoggingProjects {
		d.Debug("AGL0031", fmt.Sprintf("Trying default logging project %q", project))
		_, err := d.ProjectClient.Projects().Get(project, metav1.GetOptions{})
		if err != nil {
			if kerrors.IsNotFound(err) {
				d.Debug("AGL0032", fmt.Sprintf("Project %q not found", project))
				continue
			}
			d.Error("AGL0034", err, fmt.Sprintf("Fetching project %q returned with error", project))
			return nil
		}

		d.Debug("AGL0033", fmt.Sprintf("Found default logging project %q", project))
		d.Project = project
		return nil
	}
	//tried to complete here but no known logging project exists, will be checked in CanRun()
	return nil
}

func (d *AggregatedLogging) CanRun() (bool, error) {
	if len(d.Project) == 0 {
		return false, errors.New("Logging project does not exist")
	}
	if d.OAuthClientClient == nil || d.ProjectClient == nil || d.RouteClient == nil || d.CRBClient == nil || d.DCClient == nil {
		return false, errors.New("Config must include a cluster-admin context to run this diagnostic")
	}
	if d.KubeClient == nil {
		return false, errors.New("Config must include a cluster-admin context to run this diagnostic")
	}
	return true, nil
}

func (d *AggregatedLogging) Check() types.DiagnosticResult {
	d.Debug("AGL0015", fmt.Sprintf("Trying diagnostics for project '%s'", d.Project))
	p, err := d.ProjectClient.Projects().Get(d.Project, metav1.GetOptions{})
	if err != nil {
		d.Error("AGL0018", err, fmt.Sprintf("There was an error retrieving project '%s' which is most likely a transient error: %s", d.Project, err))
		return d.result
	}
	nodeSelector, ok := p.ObjectMeta.Annotations["openshift.io/node-selector"]
	if !ok || len(nodeSelector) != 0 {
		d.Warn("AGL0030", nil, fmt.Sprintf(projectNodeSelectorWarning, d.Project))
	}
	checkServiceAccounts(d, d, d.Project)
	checkClusterRoleBindings(d, d, d.Project)
	checkSccs(d, d, d.Project)
	checkDeploymentConfigs(d, d, d.Project)
	checkDaemonSets(d, d, d.Project)
	checkServices(d, d, d.Project)
	checkRoutes(d, d, d.Project)
	checkKibana(d, d.RouteClient, d.OAuthClientClient, d.KubeClient, d.Project)
	return d.result
}

func (d *AggregatedLogging) AvailableParameters() []types.Parameter {
	return []types.Parameter{
		{flagLoggingProject, fmt.Sprintf("Project that has deployed aggregated logging. Default projects: %s", strings.Join(defaultLoggingProjects, " or ")), &d.Project, ""},
	}
}

const projectNodeSelectorWarning = `
The project '%[1]s' was found with either a missing or non-empty node selector annotation.
This could keep Fluentd from running on certain nodes and collecting logs from the entire cluster.
You can correct it by editing the project:

  $ oc edit namespace %[1]s

and updating the annotation:

  'openshift.io/node-selector' : ""

`

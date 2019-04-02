package aggregated_logging

import (
	"errors"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kapibatch "k8s.io/kubernetes/pkg/apis/batch"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapisext "k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/apis/rbac"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	cronjobclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/batch/internalversion"
	rbacclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/rbac/internalversion"

	appsv1 "github.com/openshift/api/apps/v1"
	appstypedclient "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	oauthtypedclient "github.com/openshift/origin/pkg/oauth/generated/internalclientset/typed/oauth/internalversion"
	"github.com/openshift/origin/pkg/oc/cli/admin/diagnostics/diagnostics/log"
	"github.com/openshift/origin/pkg/oc/cli/admin/diagnostics/diagnostics/types"
	projecttypedclient "github.com/openshift/origin/pkg/project/generated/internalclientset/typed/project/internalversion"
	routesapi "github.com/openshift/origin/pkg/route/apis/route"
	routetypedclient "github.com/openshift/origin/pkg/route/generated/internalclientset/typed/route/internalversion"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	securitytypedclient "github.com/openshift/origin/pkg/security/generated/internalclientset/typed/security/internalversion"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
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
	CRBClient         rbacclient.ClusterRoleBindingsGetter
	DCClient          appstypedclient.DeploymentConfigsGetter
	SCCClient         securitytypedclient.SecurityContextConstraintsGetter
	KubeClient        kclientset.Interface
	CronJobClient     cronjobclient.CronJobsGetter
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
	crbClient rbacclient.ClusterRoleBindingsGetter,
	dcClient appstypedclient.DeploymentConfigsGetter,
	sccClient securitytypedclient.SecurityContextConstraintsGetter,
	cronjobClient cronjobclient.CronJobsGetter,
) *AggregatedLogging {
	return &AggregatedLogging{
		Project:           project,
		OAuthClientClient: oauthClientClient,
		ProjectClient:     projectClient,
		RouteClient:       routeClient,
		CRBClient:         crbClient,
		DCClient:          dcClient,
		SCCClient:         sccClient,
		CronJobClient:     cronjobClient,
		KubeClient:        kclient,
		result:            types.NewDiagnosticResult(AggregatedLoggingName),
	}
}

func (d *AggregatedLogging) getScc(name string) (*securityapi.SecurityContextConstraints, error) {
	return d.SCCClient.SecurityContextConstraints().Get(name, metav1.GetOptions{})
}

func (d *AggregatedLogging) listClusterRoleBindings() (*rbac.ClusterRoleBindingList, error) {
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
func (d *AggregatedLogging) deploymentconfigs(project string, options metav1.ListOptions) (*appsv1.DeploymentConfigList, error) {
	return d.DCClient.DeploymentConfigs(project).List(options)
}

func (d *AggregatedLogging) cronjobs(project string, options metav1.ListOptions) (*kapibatch.CronJobList, error) {
	return d.CronJobClient.CronJobs(project).List(options)
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
	checkCronJobs(d, d, d.Project)
	checkDaemonSets(d, d, d.Project)
	checkServices(d, d, d.Project)
	checkRoutes(d, d, d.Project)
	checkKibana(d, d.RouteClient, d.OAuthClientClient, d.KubeClient, d.Project)
	return d.result
}

func (d *AggregatedLogging) AvailableParameters() []types.Parameter {
	return []types.Parameter{
		{
			Name:        flagLoggingProject,
			Description: fmt.Sprintf("Project that has deployed aggregated logging. Default projects: %s", strings.Join(defaultLoggingProjects, " or ")),
			Target:      &d.Project,
			Default:     "",
		},
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

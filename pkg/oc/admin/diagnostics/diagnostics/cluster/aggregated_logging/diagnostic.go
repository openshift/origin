package aggregated_logging

import (
	"bytes"
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
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/types"
	projecttypedclient "github.com/openshift/origin/pkg/project/generated/internalclientset/typed/project/internalversion"
	routesapi "github.com/openshift/origin/pkg/route/apis/route"
	routetypedclient "github.com/openshift/origin/pkg/route/generated/internalclientset/typed/route/internalversion"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	securitytypedclient "github.com/openshift/origin/pkg/security/generated/internalclientset/typed/security/internalversion"
)

// AggregatedLogging is a Diagnostic to check the configurations
// and general integration of the OpenShift stack
// for aggregating container logs
// https://github.com/openshift/origin-aggregated-logging
type AggregatedLogging struct {
	OAuthClientClient oauthtypedclient.OAuthClientsGetter
	ProjectClient     projecttypedclient.ProjectsGetter
	RouteClient       routetypedclient.RoutesGetter
	CRBClient         oauthorizationtypedclient.ClusterRoleBindingsGetter
	DCClient          appstypedclient.DeploymentConfigsGetter
	SCCClient         securitytypedclient.SecurityContextConstraintsGetter
	KubeClient        kclientset.Interface
	sumResult         types.DiagnosticResult
	projResult        map[string]types.DiagnosticResult
	currentProject    string
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
var loggingProjects = []string{"logging", "openshift-logging"}

//NewAggregatedLogging returns the AggregatedLogging Diagnostic
func NewAggregatedLogging(
	kclient kclientset.Interface,
	oauthClientClient oauthtypedclient.OAuthClientsGetter,
	projectClient projecttypedclient.ProjectsGetter,
	routeClient routetypedclient.RoutesGetter,
	crbClient oauthorizationtypedclient.ClusterRoleBindingsGetter,
	dcClient appstypedclient.DeploymentConfigsGetter,
	sccClient securitytypedclient.SecurityContextConstraintsGetter,
) *AggregatedLogging {
	projResult := make(map[string]types.DiagnosticResult)
	for _, p := range loggingProjects {
		projResult[p] = types.NewDiagnosticResult(AggregatedLoggingName)
	}
	return &AggregatedLogging{
		OAuthClientClient: oauthClientClient,
		ProjectClient:     projectClient,
		RouteClient:       routeClient,
		CRBClient:         crbClient,
		DCClient:          dcClient,
		SCCClient:         sccClient,
		KubeClient:        kclient,
		sumResult:         types.NewDiagnosticResult(AggregatedLoggingName),
		projResult:        projResult,
		currentProject:    loggingProjects[0],
	}
}

func (d *AggregatedLogging) getScc(name string) (*securityapi.SecurityContextConstraints, error) {
	return d.SCCClient.SecurityContextConstraints().Get(name, metav1.GetOptions{})
}

func (d *AggregatedLogging) getClusterRoleBinding(name string) (*authapi.ClusterRoleBinding, error) {
	return d.CRBClient.ClusterRoleBindings().Get(name, metav1.GetOptions{})
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
func (d *AggregatedLogging) checkProjectDiagnostics(project string) {
	p, err := d.ProjectClient.Projects().Get(project, metav1.GetOptions{})
	if err != nil {
		d.Error("AGL0012", err, fmt.Sprintf("There was an error retrieving project '%s' which is most likely a transient error: %s", project, err))
		return
	}
	nodeSelector, ok := p.ObjectMeta.Annotations["openshift.io/node-selector"]
	if !ok || len(nodeSelector) != 0 {
		d.Warn("AGL0014", nil, fmt.Sprintf(projectNodeSelectorWarning, project))
	}
	checkServiceAccounts(d, d, project)
	checkClusterRoleBindings(d, d, project)
	checkSccs(d, d, project)
	checkDeploymentConfigs(d, d, project)
	checkDaemonSets(d, d, project)
	checkServices(d, d, project)
	checkRoutes(d, d, project)
	checkKibana(d, d.RouteClient, d.OAuthClientClient, d.KubeClient, project)
}

func (d *AggregatedLogging) Info(id string, message string) {
	d.sumResult.Info(id, message)
	d.projResult[d.currentProject].Info(id, message)
}

func (d *AggregatedLogging) Error(id string, err error, message string) {
	d.sumResult.Error(id, err, message)
	d.projResult[d.currentProject].Error(id, err, message)
}

func (d *AggregatedLogging) Debug(id string, message string) {
	d.sumResult.Debug(id, message)
	d.projResult[d.currentProject].Debug(id, message)
}

func (d *AggregatedLogging) Warn(id string, err error, message string) {
	d.sumResult.Warn(id, err, message)
	d.projResult[d.currentProject].Warn(id, err, message)
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

func (d *AggregatedLogging) CanRun() (bool, error) {
	if d.OAuthClientClient == nil || d.ProjectClient == nil || d.RouteClient == nil || d.CRBClient == nil || d.DCClient == nil {
		return false, errors.New("Config must include a cluster-admin context to run this diagnostic")
	}
	if d.KubeClient == nil {
		return false, errors.New("Config must include a cluster-admin context to run this diagnostic")
	}
	return true, nil
}

func (d *AggregatedLogging) Check() types.DiagnosticResult {
	for _, p := range loggingProjects {
		d.currentProject = p
		d.Debug("AGL0010", fmt.Sprintf("Trying diagnostics for project '%s'", p))
		d.checkProjectDiagnostics(p)
		if d.projResult[p].Failure() {
			d.Debug("AGL0020", fmt.Sprintf("Diagnostics for project '%s' have errors", p))
		} else {
			d.Debug("AGL0022", fmt.Sprintf("Diagnostics for project '%s' look ok", p))
			return d.projResult[p]
		}
	}

	var buff bytes.Buffer
	for p, r := range d.projResult {
		s := "errors"
		if len(r.Errors()) == 1 {
			s = "error"
		}
		buff.WriteString(fmt.Sprintf(", %s: %d %s", p, len(r.Errors()), s))
	}
	msg := fmt.Sprintf("Unable to find the AggregatedLogging project without errors%s", buff.String())
	d.Error("AGL0030", fmt.Errorf(msg), msg)
	return d.sumResult
}

const projectNodeSelectorWarning = `
The project '%[1]s' was found with either a missing or non-empty node selector annotation.
This could keep Fluentd from running on certain nodes and collecting logs from the entire cluster.
You can correct it by editing the project:

  $ oc edit namespace %[1]s

and updating the annotation:

  'openshift.io/node-selector' : ""

`

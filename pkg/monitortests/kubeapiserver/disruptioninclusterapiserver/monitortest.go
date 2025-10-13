package disruptioninclusterapiserver

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/openshift/origin/pkg/monitortestlibrary/disruptionlibrary"
	"github.com/openshift/origin/pkg/test/extensions"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	corev1 "k8s.io/api/core/v1"

	exutil "github.com/openshift/origin/test/extended/util"

	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
)

var (
	//go:embed manifests/namespace.yaml
	namespaceYaml []byte
	//go:embed manifests/crb-privileged.yaml
	rbacPrivilegedYaml []byte
	//go:embed manifests/clusterrole-monitor.yaml
	rbacMonitorClusterRoleYaml []byte
	//go:embed manifests/role-monitor.yaml
	rbacMonitorRoleYaml []byte
	//go:embed manifests/rb-monitor.yaml
	rbacMonitorRBYaml []byte
	//go:embed manifests/crb-monitor.yaml
	rbacListOauthClientCRBYaml []byte
	//go:embed manifests/serviceaccount.yaml
	serviceAccountYaml []byte
	//go:embed manifests/dep-internal-lb.yaml
	internalLBDeploymentYaml []byte
	//go:embed manifests/pdb-internal-lb.yaml
	internalLBPDBYaml []byte
	//go:embed manifests/dep-service-network.yaml
	serviceNetworkDeploymentYaml []byte
	//go:embed manifests/pdb-service-network.yaml
	serviceNetworkPDBYaml []byte
	//go:embed manifests/dep-localhost.yaml
	localhostDeploymentYaml    []byte
	rbacPrivilegedCRBName      string
	rbacMonitorClusterRoleName string
	rbacMonitorCRBName         string
)

type InvariantInClusterDisruption struct {
	namespaceName               string
	openshiftTestsImagePullSpec string
	payloadImagePullSpec        string
	notSupportedReason          string
	replicas                    int32
	controlPlaneNodes           int32

	isHypershift          bool
	isAROHCPCluster       bool
	isBareMetalHypershift bool
	adminRESTConfig *rest.Config
	kubeClient      kubernetes.Interface
}

func NewInvariantInClusterDisruption(info monitortestframework.MonitorTestInitializationInfo) monitortestframework.MonitorTest {
	return &InvariantInClusterDisruption{
		payloadImagePullSpec: info.UpgradeTargetPayloadImagePullSpec,
	}
}

// parseAdminRESTConfigHost parses the adminRESTConfig.Host URL and returns hostname and port
func (i *InvariantInClusterDisruption) parseAdminRESTConfigHost() (hostname, port string, err error) {
	parsedURL, err := url.Parse(i.adminRESTConfig.Host)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse adminRESTConfig.Host %q: %v", i.adminRESTConfig.Host, err)
	}

	hostname = parsedURL.Hostname()
	if hostname == "" {
		return "", "", fmt.Errorf("no hostname found in adminRESTConfig.Host %q", i.adminRESTConfig.Host)
	}

	port = parsedURL.Port()
	if port == "" {
		port = "6443" // default port
	}

	return hostname, port, nil
}

// setKubernetesServiceEnvVars sets the KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT environment variables
// based on the cluster type (ARO HCP, bare metal HyperShift, or standard)
func (i *InvariantInClusterDisruption) setKubernetesServiceEnvVars(envVars []corev1.EnvVar, apiIntHost, apiIntPort string) []corev1.EnvVar {
	// Parse adminRESTConfig.Host once for bare metal HyperShift
	var bareMetalHost, bareMetalPort string
	var bareMetalErr error
	if i.isHypershift && i.isBareMetalHypershift {
		bareMetalHost, bareMetalPort, bareMetalErr = i.parseAdminRESTConfigHost()
		if bareMetalErr != nil {
			logrus.WithError(bareMetalErr).Errorf("Failed to parse adminRESTConfig.Host for bare metal HyperShift")
		}
	}

	for j, env := range envVars {
		switch env.Name {
		case "KUBERNETES_SERVICE_HOST":
			if i.isHypershift && i.isBareMetalHypershift {
				if bareMetalErr != nil {
					envVars[j].Value = apiIntHost
				} else {
					envVars[j].Value = bareMetalHost
				}
			} else {
				envVars[j].Value = apiIntHost
			}
		case "KUBERNETES_SERVICE_PORT":
			if i.isHypershift && i.isAROHCPCluster {
				envVars[j].Value = "7443"
			} else if i.isHypershift && i.isBareMetalHypershift {
				if bareMetalErr != nil {
					envVars[j].Value = apiIntPort
				} else {
					envVars[j].Value = bareMetalPort
				}
			} else {
				envVars[j].Value = apiIntPort
			}
		}
	}
	return envVars
}

func (i *InvariantInClusterDisruption) createDeploymentAndWaitToRollout(ctx context.Context, deploymentObj *appsv1.Deployment) error {
	deploymentID := uuid.New().String()
	deploymentObj = disruptionlibrary.UpdateDeploymentENVs(deploymentObj, deploymentID, "")

	client := i.kubeClient.AppsV1().Deployments(deploymentObj.Namespace)
	_, err := client.Create(ctx, deploymentObj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error creating deployment %s: %v", deploymentObj.Namespace, err)
	}

	timeLimitedCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if _, watchErr := watchtools.UntilWithSync(timeLimitedCtx,
		cache.NewListWatchFromClient(
			i.kubeClient.AppsV1().RESTClient(), "deployments", deploymentObj.Namespace, fields.OneTermEqualSelector("metadata.name", deploymentObj.Name)),
		&appsv1.Deployment{},
		nil,
		func(event watch.Event) (bool, error) {
			deployment := event.Object.(*appsv1.Deployment)
			return deployment.Status.AvailableReplicas == *deployment.Spec.Replicas, nil
		},
	); watchErr != nil {
		return fmt.Errorf("deployment %s didn't roll out: %v", deploymentObj.Name, watchErr)
	}
	return nil
}

func (i *InvariantInClusterDisruption) createInternalLBDeployment(ctx context.Context, apiIntHost string) error {
	deploymentObj := resourceread.ReadDeploymentV1OrDie(internalLBDeploymentYaml)
	deploymentObj.SetNamespace(i.namespaceName)
	// set amount of deployment replicas to make sure it runs on all nodes
	deploymentObj.Spec.Replicas = &i.replicas
	// we need to use the openshift-tests image of the destination during an upgrade.
	deploymentObj.Spec.Template.Spec.Containers[0].Image = i.openshiftTestsImagePullSpec

	// Set the correct host and port for internal API server based on cluster type
	deploymentObj.Spec.Template.Spec.Containers[0].Env = i.setKubernetesServiceEnvVars(
		deploymentObj.Spec.Template.Spec.Containers[0].Env, apiIntHost, apiIntPort)
	err := i.createDeploymentAndWaitToRollout(ctx, deploymentObj)
	if err != nil {
		return err
	}

	pdbObj := resourceread.ReadPodDisruptionBudgetV1OrDie(internalLBPDBYaml)
	pdbObj.SetNamespace(i.namespaceName)
	client := i.kubeClient.PolicyV1().PodDisruptionBudgets(i.namespaceName)
	_, err = client.Create(ctx, pdbObj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error creating PDB %s: %v", deploymentObj.Namespace, err)
	}
	return nil
}

func (i *InvariantInClusterDisruption) createServiceNetworkDeployment(ctx context.Context) error {
	deploymentObj := resourceread.ReadDeploymentV1OrDie(serviceNetworkDeploymentYaml)
	deploymentObj.SetNamespace(i.namespaceName)
	// set amount of deployment replicas to make sure it runs on all nodes
	deploymentObj.Spec.Replicas = &i.replicas
	// we need to use the openshift-tests image of the destination during an upgrade.
	deploymentObj.Spec.Template.Spec.Containers[0].Image = i.openshiftTestsImagePullSpec

	err := i.createDeploymentAndWaitToRollout(ctx, deploymentObj)
	if err != nil {
		return err
	}

	pdbObj := resourceread.ReadPodDisruptionBudgetV1OrDie(serviceNetworkPDBYaml)
	pdbObj.SetNamespace(i.namespaceName)
	client := i.kubeClient.PolicyV1().PodDisruptionBudgets(i.namespaceName)
	_, err = client.Create(ctx, pdbObj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error creating PDB %s: %v", deploymentObj.Namespace, err)
	}
	return nil
}

func (i *InvariantInClusterDisruption) createLocalhostDeployment(ctx context.Context) error {
	// Don't start localhost deployment on hypershift
	if i.controlPlaneNodes == 0 {
		return nil
	}

	deploymentObj := resourceread.ReadDeploymentV1OrDie(localhostDeploymentYaml)
	deploymentObj.SetNamespace(i.namespaceName)
	// set amount of deployment replicas to make sure it runs on control plane nodes
	deploymentObj.Spec.Replicas = &i.controlPlaneNodes
	// we need to use the openshift-tests image of the destination during an upgrade.
	deploymentObj.Spec.Template.Spec.Containers[0].Image = i.openshiftTestsImagePullSpec

	return i.createDeploymentAndWaitToRollout(ctx, deploymentObj)
}

func (i *InvariantInClusterDisruption) createRBACPrivileged(ctx context.Context) error {
	rbacPrivilegedObj := resourceread.ReadClusterRoleBindingV1OrDie(rbacPrivilegedYaml)
	rbacPrivilegedObj.Subjects[0].Namespace = i.namespaceName

	client := i.kubeClient.RbacV1().ClusterRoleBindings()
	obj, err := client.Create(ctx, rbacPrivilegedObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating privileged SCC CRB: %v", err)
	}
	rbacPrivilegedCRBName = obj.Name
	return nil
}

func (i *InvariantInClusterDisruption) createMonitorClusterRole(ctx context.Context) error {
	rbacMonitorRoleObj := resourceread.ReadClusterRoleV1OrDie(rbacMonitorClusterRoleYaml)

	client := i.kubeClient.RbacV1().ClusterRoles()
	_, err := client.Create(ctx, rbacMonitorRoleObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating oauthclients list role: %v", err)
	}
	rbacMonitorClusterRoleName = rbacMonitorRoleObj.Name
	return nil
}

func (i *InvariantInClusterDisruption) createMonitorRole(ctx context.Context) error {
	rbacMonitorRoleObj := resourceread.ReadRoleV1OrDie(rbacMonitorRoleYaml)
	rbacMonitorRoleObj.Namespace = i.namespaceName

	client := i.kubeClient.RbacV1().Roles(i.namespaceName)
	_, err := client.Create(ctx, rbacMonitorRoleObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating oauthclients list role: %v", err)
	}
	return nil
}

func (i *InvariantInClusterDisruption) createMonitorCRB(ctx context.Context) error {
	rbacMonitorCRBObj := resourceread.ReadClusterRoleBindingV1OrDie(rbacListOauthClientCRBYaml)
	rbacMonitorCRBObj.Subjects[0].Namespace = i.namespaceName

	client := i.kubeClient.RbacV1().ClusterRoleBindings()
	obj, err := client.Create(ctx, rbacMonitorCRBObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating oauthclients list CRB: %v", err)
	}
	rbacMonitorCRBName = obj.Name
	return nil
}

func (i *InvariantInClusterDisruption) createMonitorRB(ctx context.Context) error {
	rbacMonitorCRBObj := resourceread.ReadRoleBindingV1OrDie(rbacMonitorRBYaml)
	rbacMonitorCRBObj.Subjects[0].Namespace = i.namespaceName

	client := i.kubeClient.RbacV1().RoleBindings(i.namespaceName)
	obj, err := client.Create(ctx, rbacMonitorCRBObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating monitor RB: %v", err)
	}
	rbacMonitorCRBName = obj.Name
	return nil
}

func (i *InvariantInClusterDisruption) createServiceAccount(ctx context.Context) error {
	serviceAccountObj := resourceread.ReadServiceAccountV1OrDie(serviceAccountYaml)
	serviceAccountObj.SetNamespace(i.namespaceName)

	client := i.kubeClient.CoreV1().ServiceAccounts(i.namespaceName)
	_, err := client.Create(ctx, serviceAccountObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating service account: %v", err)
	}
	return nil
}

func (i *InvariantInClusterDisruption) createNamespace(ctx context.Context) (string, error) {
	log := logrus.WithField("monitorTest", "apiserver-incluster-availability").WithField("namespace", i.namespaceName).WithField("func", "createNamespace")

	namespaceObj := resourceread.ReadNamespaceV1OrDie(namespaceYaml)

	client := i.kubeClient.CoreV1().Namespaces()
	actualNamespace, err := client.Create(ctx, namespaceObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return "", fmt.Errorf("error creating namespace: %v", err)
	}
	log.Infof("created namespace %s", actualNamespace.Name)
	return actualNamespace.Name, nil
}

func (i *InvariantInClusterDisruption) removeExistingMonitorNamespaces(ctx context.Context) error {
	log := logrus.WithField("monitorTest", "apiserver-incluster-availability").WithField("namespace", i.namespaceName).WithField("func", "namespaceAlreadyCreated")
	namespaces, err := i.kubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{"apiserver.openshift.io/incluster-disruption": "true"}.AsSelector().String(),
	})
	if err != nil {
		log.Infof("error: %v", err)
		return err
	}
	for _, ns := range namespaces.Items {
		if err := i.deleteNamespace(ctx, ns.Name); err != nil {
			return err
		}
	}
	return nil
}

func (i *InvariantInClusterDisruption) deleteNamespace(ctx context.Context, name string) error {
	log := logrus.WithField("monitorTest", "apiserver-incluster-availability").WithField("namespace", name).WithField("func", "deleteNamespace")
	log.Infof("removing monitoring namespace")
	nsClient := i.kubeClient.CoreV1().Namespaces()
	err := nsClient.Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error removing namespace %s: %v", name, err)
	}
	if !apierrors.IsNotFound(err) {
		log.Infof("Namespace %s removed", name)
	}
	return nil
}

func (i *InvariantInClusterDisruption) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, _ monitorapi.RecorderWriter) error {
	var err error
	log := logrus.WithField("monitorTest", "apiserver-incluster-availability").WithField("namespace", i.namespaceName).WithField("func", "StartCollection")
	log.Infof("payload image pull spec is %v", i.payloadImagePullSpec)
	if len(i.payloadImagePullSpec) == 0 {
		configClient, err := configclient.NewForConfig(adminRESTConfig)
		if err != nil {
			return err
		}
		clusterVersion, err := configClient.ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			i.notSupportedReason = "clusterversion/version not found and no image pull spec specified."
			return nil
		}
		if err != nil {
			return err
		}
		i.payloadImagePullSpec = clusterVersion.Status.History[0].Image
	}

	// Check for ARO HCP and skip if detected
	oc := exutil.NewCLI("apiserver-incluster-availability").AsAdmin()
	var isAROHCPcluster bool
	isHypershift, _ := exutil.IsHypershift(ctx, oc.AdminConfigClient())
	if isHypershift {
		_, hcpNamespace, err := exutil.GetHypershiftManagementClusterConfigAndNamespace()
		if err != nil {
			logrus.WithError(err).Error("failed to get hypershift management cluster config and namespace")
		}

		// For Hypershift, only skip if it's specifically ARO HCP
		// Use management cluster client to check the control-plane-operator deployment
		managementOC := exutil.NewHypershiftManagementCLI(hcpNamespace)
		if isAROHCPcluster, err = exutil.IsAroHCP(ctx, hcpNamespace, managementOC.AdminKubeClient()); err != nil {
			logrus.WithError(err).Warning("Failed to check if ARO HCP, assuming it's not")
			i.isAROHCPCluster = false // Assume not ARO HCP on error
		} else if isAROHCPcluster {
			i.isAROHCPCluster = true
		} else {
			i.isAROHCPCluster = false
		}

		// Check if this is a bare metal HyperShift cluster
		i.isBareMetalHypershift, err = exutil.IsBareMetalHyperShiftCluster(ctx, managementOC)
		if err != nil {
			logrus.WithError(err).Warning("Failed to check if bare metal HyperShift, assuming it's not")
			i.isBareMetalHypershift = false // Assume not bare metal HyperShift on error
		}
	}

	if len(i.payloadImagePullSpec) == 0 {
		i.payloadImagePullSpec, err = extensions.DetermineReleasePayloadImage()
		if err != nil {
			return err
		}

		if len(i.payloadImagePullSpec) == 0 {
			log.Info("unable to determine payloadImagePullSpec")
			i.notSupportedReason = "no image pull spec specified."
			return nil
		}
	}

	// runImageExtract extracts src from specified image to dst
	cmd := exec.Command("oc", "adm", "release", "info", i.payloadImagePullSpec, "--image-for=tests")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.Stdout = out
	cmd.Stderr = errOut
	if err := cmd.Run(); err != nil {
		i.notSupportedReason = fmt.Sprintf("unable to determine openshift-tests image: %v: %v", err, errOut.String())
		return nil
	}
	i.openshiftTestsImagePullSpec = strings.TrimSpace(out.String())
	log.Infof("openshift-tests image pull spec is %v", i.openshiftTestsImagePullSpec)

	i.adminRESTConfig = adminRESTConfig
	i.kubeClient, err = kubernetes.NewForConfig(i.adminRESTConfig)
	if err != nil {
		return fmt.Errorf("error constructing kube client: %v", err)
	}

	if ok, _ := exutil.IsMicroShiftCluster(i.kubeClient); ok {
		i.notSupportedReason = "microshift clusters don't have load balancers"
		log.Infof("IsMicroShiftCluster: %s", i.notSupportedReason)
		return nil
	}

	// Replace namespace from earlier test
	if err := i.removeExistingMonitorNamespaces(ctx); err != nil {
		log.Infof("removeExistingMonitorNamespaces returned error %v", err)
		return err
	}

	log.Infof("starting monitoring deployments")
	configClient, err := configclient.NewForConfig(i.adminRESTConfig)
	if err != nil {
		return fmt.Errorf("error constructing openshift config client: %v", err)
	}
	infra, err := configClient.ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting openshift infrastructure: %v", err)
	}

	var apiIntHost string
	var apiIntPort string
	if i.isHypershift {
		apiIntHost, apiIntPort, err = i.parseAdminRESTConfigHost()
		if err != nil {
			return fmt.Errorf("failed to parse adminRESTConfig.Host: %v", err)
		}
	} else {
		internalAPI, err := url.Parse(infra.Status.APIServerInternalURL)
		if err != nil {
			return fmt.Errorf("error parsing api int url: %v", err)
		}
		apiIntHost = internalAPI.Hostname()
		if internalAPI.Port() != "" {
			apiIntPort = internalAPI.Port()
		} else {
			apiIntPort = "6443" // default port
		}
	}

	allNodes, err := i.kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error getting nodes: %v", err)
	}
	i.replicas = 2
	if len(allNodes.Items) == 1 {
		i.replicas = 1
	}
	controlPlaneNodes, err := i.kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{"node-role.kubernetes.io/master": ""}.AsSelector().String(),
	})
	if err != nil {
		return fmt.Errorf("error getting control plane nodes: %v", err)
	}
	i.controlPlaneNodes = int32(len(controlPlaneNodes.Items))

	namespace, err := i.createNamespace(ctx)
	if err != nil {
		return fmt.Errorf("error creating namespace: %v", err)
	}
	i.namespaceName = namespace

	err = i.createServiceAccount(ctx)
	if err != nil {
		return fmt.Errorf("error creating service accounts: %v", err)
	}
	err = i.createRBACPrivileged(ctx)
	if err != nil {
		return fmt.Errorf("error creating privileged SCC rolebinding: %v", err)
	}
	err = i.createMonitorClusterRole(ctx)
	if err != nil {
		return fmt.Errorf("error creating monitor cluster role: %v", err)
	}
	err = i.createMonitorCRB(ctx)
	if err != nil {
		return fmt.Errorf("error creating monitor rolebinding: %v", err)
	}
	err = i.createMonitorRole(ctx)
	if err != nil {
		return fmt.Errorf("error creating monitor role: %v", err)
	}
	err = i.createMonitorRB(ctx)
	if err != nil {
		return fmt.Errorf("error creating monitor rolebinding: %v", err)
	}
	err = i.createServiceNetworkDeployment(ctx)
	if err != nil {
		return fmt.Errorf("error creating service network deployment: %v", err)
	}
	err = i.createLocalhostDeployment(ctx)
	if err != nil {
		return fmt.Errorf("error creating localhost: %v", err)
	}
	err = i.createInternalLBDeployment(ctx, apiIntHost)
	if err != nil {
		return fmt.Errorf("error creating internal LB: %v", err)
	}
	log.Infof("monitoring deployments started")
	return nil
}

func (i *InvariantInClusterDisruption) CollectData(ctx context.Context, storageDir string, beginning time.Time, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	log := logrus.WithField("monitorTest", "apiserver-incluster-availability").WithField("namespace", i.namespaceName).WithField("func", "CollectData")

	if len(i.notSupportedReason) > 0 {
		return nil, nil, nil
	}

	log.Infof("creating flag configmap")

	// create the stop collecting configmap and wait for 30s to thing to have stopped.  the 30s is just a guess
	if _, err := i.kubeClient.CoreV1().ConfigMaps(i.namespaceName).Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "stop-collecting"},
	}, metav1.CreateOptions{}); err != nil {
		return nil, nil, err
	}

	// TODO create back-pressure on the configmap
	select {
	case <-time.After(30 * time.Second):
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}

	log.Infof("collecting data from the deployments")

	pollerLabel, err := labels.NewRequirement("apiserver.openshift.io/disruption-actor", selection.Equals, []string{"poller"})
	if err != nil {
		return nil, nil, err
	}

	intervals, junits, errs := disruptionlibrary.CollectIntervalsForPods(ctx, i.kubeClient, "Jira: \"kube-apiserver\"", i.namespaceName, labels.NewSelector().Add(*pollerLabel))
	log.Infof("intervals collected")
	return intervals, junits, utilerrors.NewAggregate(errs)
}

func (i *InvariantInClusterDisruption) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, _ monitorapi.ResourcesMap, beginning time.Time, end time.Time) (constructedIntervals monitorapi.Intervals, err error) {
	return nil, nil
}

func (i *InvariantInClusterDisruption) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (i *InvariantInClusterDisruption) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (i *InvariantInClusterDisruption) Cleanup(ctx context.Context) error {
	log := logrus.WithField("monitorTest", "apiserver-incluster-availability").WithField("namespace", i.namespaceName).WithField("func", "Cleanup")
	if len(i.notSupportedReason) > 0 {
		return nil
	}

	log.Infof("removing monitoring namespace")
	nsClient := i.kubeClient.CoreV1().Namespaces()
	err := nsClient.Delete(ctx, i.namespaceName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error removing namespace %s: %v", i.namespaceName, err)
	}
	if !apierrors.IsNotFound(err) {
		log.Infof("Namespace %s removed", i.namespaceName)
	}

	log.Infof("removing monitoring cluster roles and bindings")
	crbClient := i.kubeClient.RbacV1().ClusterRoleBindings()
	err = crbClient.Delete(ctx, rbacPrivilegedCRBName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error removing cluster reader CRB: %v", err)
	}
	if !apierrors.IsNotFound(err) {
		log.Infof("CRB %s removed", rbacPrivilegedCRBName)
	}

	err = crbClient.Delete(ctx, rbacMonitorCRBName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error removing monitor CRB: %v", err)
	}
	if !apierrors.IsNotFound(err) {
		log.Infof("CRB %s removed", rbacMonitorCRBName)
	}

	rolesClient := i.kubeClient.RbacV1().ClusterRoles()
	err = rolesClient.Delete(ctx, rbacMonitorClusterRoleName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error removing monitor role: %v", err)
	}
	if !apierrors.IsNotFound(err) {
		log.Infof("Role %s removed", rbacMonitorClusterRoleName)
	}
	log.Infof("collect data completed")
	return nil
}

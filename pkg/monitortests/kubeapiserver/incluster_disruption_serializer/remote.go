package incluster_disruption_serializer

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/openshift/origin/pkg/monitortestlibrary/disruptionlibrary"
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
	//go:embed manifests/role-monitor.yaml
	rbacMonitorRoleYaml []byte
	//go:embed manifests/crb-monitor.yaml
	rbacListOauthClientCRBYaml []byte
	//go:embed manifests/serviceaccount.yaml
	serviceAccountYaml []byte
	//go:embed manifests/dep-internal-lb.yaml
	internalLBDeploymentYaml []byte
	//go:embed manifests/dep-service-network.yaml
	serviceNetworkDeploymentYaml []byte
	//go:embed manifests/dep-localhost.yaml
	localhostDeploymentYaml []byte
	rbacPrivilegedCRBName   string
	rbacMonitorRoleName     string
	rbacMonitorCRBName      string
)

type InvariantInClusterDisruption struct {
	namespaceName               string
	openshiftTestsImagePullSpec string
	payloadImagePullSpec        string
	notSupportedReason          string
	allNodes                    int32
	controlPlaneNodes           int32

	adminRESTConfig *rest.Config
	kubeClient      kubernetes.Interface
}

func NewInvariantInClusterDisruption(info monitortestframework.MonitorTestInitializationInfo) monitortestframework.MonitorTest {
	return &InvariantInClusterDisruption{
		payloadImagePullSpec: info.UpgradeTargetPayloadImagePullSpec,
	}
}

func (i *InvariantInClusterDisruption) createDeploymentAndWaitToRollout(ctx context.Context, deploymentObj *appsv1.Deployment) error {

	client := i.kubeClient.AppsV1().Deployments(deploymentObj.Namespace)
	_, err := client.Create(ctx, deploymentObj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error creating deployment %s: %v", deploymentObj.Namespace, err)
	}
	fmt.Printf("in-cluster monitor: deployment %s:\n%#v\n", deploymentObj.Name, deploymentObj.ObjectMeta)

	timeLimitedCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if _, watchErr := watchtools.UntilWithSync(timeLimitedCtx,
		cache.NewListWatchFromClient(
			i.kubeClient.AppsV1().RESTClient(), "deployments", deploymentObj.Namespace, fields.OneTermEqualSelector("metadata.name", deploymentObj.Name)),
		&appsv1.Deployment{},
		nil,
		func(event watch.Event) (bool, error) {
			deployment := event.Object.(*appsv1.Deployment)
			return deployment.Status.ReadyReplicas == deployment.Status.Replicas, nil
		},
	); watchErr != nil {
		return fmt.Errorf("deployment %s didn't roll out: %v", deploymentObj.Name, watchErr)
	}
	return nil
}

func (i *InvariantInClusterDisruption) createInternalLBDeployment(ctx context.Context, apiIntHost string) error {
	deploymentObj := resourceread.ReadDeploymentV1OrDie(internalLBDeploymentYaml)
	deploymentObj.SetNamespace(i.namespaceName)
	deploymentObj.Spec.Template.Spec.Containers[0].Env[0].Value = apiIntHost
	// set amount of deployment replicas to make sure it runs on all nodes
	deploymentObj.Spec.Replicas = &i.allNodes
	// we need to use the openshift-tests image of the destination during an upgrade.
	deploymentObj.Spec.Template.Spec.Containers[0].Image = i.openshiftTestsImagePullSpec

	return i.createDeploymentAndWaitToRollout(ctx, deploymentObj)
}

func (i *InvariantInClusterDisruption) createServiceNetworkDeployment(ctx context.Context) error {
	deploymentObj := resourceread.ReadDeploymentV1OrDie(serviceNetworkDeploymentYaml)
	deploymentObj.SetNamespace(i.namespaceName)
	// set amount of deployment replicas to make sure it runs on all nodes
	deploymentObj.Spec.Replicas = &i.allNodes
	// we need to use the openshift-tests image of the destination during an upgrade.
	deploymentObj.Spec.Template.Spec.Containers[0].Image = i.openshiftTestsImagePullSpec

	return i.createDeploymentAndWaitToRollout(ctx, deploymentObj)
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
	fmt.Printf("in-cluster monitor: privileged CRB:\n%#v\n", rbacPrivilegedObj)

	client := i.kubeClient.RbacV1().ClusterRoleBindings()
	obj, err := client.Create(ctx, rbacPrivilegedObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating privileged SCC CRB: %v", err)
	}
	rbacPrivilegedCRBName = obj.Name
	return nil
}

func (i *InvariantInClusterDisruption) createMonitorRole(ctx context.Context) error {
	rbacMonitorRoleObj := resourceread.ReadClusterRoleV1OrDie(rbacMonitorRoleYaml)

	client := i.kubeClient.RbacV1().ClusterRoles()
	_, err := client.Create(ctx, rbacMonitorRoleObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating oauthclients list role: %v", err)
	}
	rbacMonitorRoleName = rbacMonitorRoleObj.Name
	return nil
}

func (i *InvariantInClusterDisruption) createMonitorCRB(ctx context.Context) error {
	rbacMonitorCRBObj := resourceread.ReadClusterRoleBindingV1OrDie(rbacListOauthClientCRBYaml)
	rbacMonitorCRBObj.Subjects[0].Namespace = i.namespaceName
	fmt.Printf("in-cluster monitor: monitor role CRB:\n%#v\n", rbacMonitorCRBObj)

	client := i.kubeClient.RbacV1().ClusterRoleBindings()
	obj, err := client.Create(ctx, rbacMonitorCRBObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating oauthclients list CRB: %v", err)
	}
	rbacMonitorCRBName = obj.Name
	return nil
}

func (i *InvariantInClusterDisruption) createServiceAccount(ctx context.Context) error {
	serviceAccountObj := resourceread.ReadServiceAccountV1OrDie(serviceAccountYaml)
	serviceAccountObj.SetNamespace(i.namespaceName)
	fmt.Printf("in-cluster monitor: serviceaccount created in %s namespace\n", i.namespaceName)

	client := i.kubeClient.CoreV1().ServiceAccounts(i.namespaceName)
	_, err := client.Create(ctx, serviceAccountObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating service account: %v", err)
	}
	return nil
}

func (i *InvariantInClusterDisruption) createNamespace(ctx context.Context) (string, error) {
	namespaceObj := resourceread.ReadNamespaceV1OrDie(namespaceYaml)

	client := i.kubeClient.CoreV1().Namespaces()
	actualNamespace, err := client.Create(ctx, namespaceObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return "", fmt.Errorf("error creating namespace: %v", err)
	}
	fmt.Printf("in-cluster monitor: created namespace %s\n", actualNamespace.Name)
	return actualNamespace.Name, nil
}

func (i *InvariantInClusterDisruption) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, _ monitorapi.RecorderWriter) error {
	var err error
	fmt.Printf("in-cluster monitor: payload image pull spec is %v\n", i.payloadImagePullSpec)
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
	fmt.Printf("in-cluster monitor: openshift-tests image pull spec is %v\n", i.openshiftTestsImagePullSpec)

	i.adminRESTConfig = adminRESTConfig
	i.kubeClient, err = kubernetes.NewForConfig(i.adminRESTConfig)
	if err != nil {
		return fmt.Errorf("error constructing kube client: %v", err)
	}

	if ok, _ := exutil.IsMicroShiftCluster(i.kubeClient); ok {
		i.notSupportedReason = "microshift clusters don't have load balancers"
		return nil
	}

	fmt.Fprintf(os.Stderr, "Starting in-cluster monitoring deployments\n")
	configClient, err := configclient.NewForConfig(i.adminRESTConfig)
	if err != nil {
		return fmt.Errorf("error constructing openshift config client: %v", err)
	}
	infra, err := configClient.ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting openshift infrastructure: %v", err)
	}

	internalAPI, err := url.Parse(infra.Status.APIServerInternalURL)
	if err != nil {
		return fmt.Errorf("error parsing api int url: %v", err)
	}
	apiIntHost := internalAPI.Hostname()

	allNodes, err := i.kubeClient.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error getting nodes: %v", err)
	}
	i.allNodes = int32(len(allNodes.Items))

	controlPlaneNodes, err := i.kubeClient.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
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
	err = i.createMonitorRole(ctx)
	if err != nil {
		return fmt.Errorf("error creating monitor role: %v", err)
	}
	err = i.createMonitorCRB(ctx)
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
	return nil
}

func (i *InvariantInClusterDisruption) CollectData(ctx context.Context, storageDir string, beginning time.Time, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if len(i.notSupportedReason) > 0 {
		return nil, nil, nil
	}

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

	fmt.Fprintf(os.Stderr, "Collecting data from in-cluster monitoring deployments\n")

	pollerLabel, err := labels.NewRequirement("network.openshift.io/disruption-actor", selection.Equals, []string{"poller"})
	if err != nil {
		return nil, nil, err
	}

	intervals, junits, errs := disruptionlibrary.CollectIntervalsForPods(ctx, i.kubeClient, i.namespaceName, labels.NewSelector().Add(*pollerLabel))
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
	if len(i.notSupportedReason) > 0 {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Removing in-cluster monitoring namespace\n")
	nsClient := i.kubeClient.CoreV1().Namespaces()
	err := nsClient.Delete(ctx, i.namespaceName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error removing namespace %s: %v", i.namespaceName, err)
	}
	if !apierrors.IsNotFound(err) {
		fmt.Fprintf(os.Stderr, "Namespace %s removed\n", i.namespaceName)
	}

	fmt.Fprintf(os.Stderr, "Removing in-cluster monitoring cluster roles and bindings\n")
	crbClient := i.kubeClient.RbacV1().ClusterRoleBindings()
	err = crbClient.Delete(ctx, rbacPrivilegedCRBName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error removing cluster reader CRB: %v", err)
	}
	if !apierrors.IsNotFound(err) {
		fmt.Fprintf(os.Stderr, "CRB %s removed\n", rbacPrivilegedCRBName)
	}

	err = crbClient.Delete(ctx, rbacMonitorCRBName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error removing monitor CRB: %v", err)
	}
	if !apierrors.IsNotFound(err) {
		fmt.Fprintf(os.Stderr, "CRB %s removed\n", rbacMonitorCRBName)
	}

	rolesClient := i.kubeClient.RbacV1().ClusterRoles()
	err = rolesClient.Delete(ctx, rbacMonitorRoleName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error removing monitor role: %v", err)
	}
	if !apierrors.IsNotFound(err) {
		fmt.Fprintf(os.Stderr, "Role %s removed\n", rbacMonitorRoleName)
	}
	return nil
}

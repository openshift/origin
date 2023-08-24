package incluster_disruption_serializer

import (
	"context"
	_ "embed"
	"fmt"
	"net/url"
	"os"
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

const (
	disruptionDataFolder = "disruption-data"
	disruptionTypeEnvVar = "DISRUPTION_TYPE_LABEL"
	inClusterEventsFile  = "junit/AdditionalEvents__in_cluster_disruption.json"
)

var (
	namespace string
	//go:embed manifests/namespace.yaml
	namespaceYaml []byte
	//go:embed manifests/crb-hostaccess.yaml
	rbacPrivilegedYaml []byte
	//go:embed manifests/role-monitor.yaml
	rbacMonitorRoleYaml []byte
	//go:embed manifests/crb-monitor.yaml
	rbacListOauthClientCRBYaml []byte
	//go:embed manifests/serviceaccount.yaml
	serviceAccountYaml []byte
	//go:embed manifests/ds-internal-lb.yaml
	dsInternalLBYaml []byte
	//go:embed manifests/ds-service-network.yaml
	dsServiceNetworkYaml []byte
	//go:embed manifests/ds-localhost.yaml
	dsLocalhostYaml []byte
	//go:embed manifests/ds-cleanup.yaml
	dsCleanupYaml         []byte
	rbacPrivilegedCRBName string
	rbacMonitorRoleName   string
	rbacMonitorCRBName    string
)

func createInternalLBDS(ctx context.Context, clientset kubernetes.Interface, apiIntHost, openshiftTestsImagePullSpec string) error {
	dsObj := resourceread.ReadDaemonSetV1OrDie(dsInternalLBYaml)
	dsObj.Namespace = namespace
	dsObj.Spec.Template.Spec.Containers[0].Env[0].Value = apiIntHost
	// we need to use the openshift-tests image of the destination during an upgrade.
	dsObj.Spec.Template.Spec.Containers[0].Image = openshiftTestsImagePullSpec

	client := clientset.AppsV1().DaemonSets(namespace)
	var err error
	_, err = client.Create(ctx, dsObj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error creating daemonset: %v", err)
	}

	timeLimitedCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if _, watchErr := watchtools.UntilWithSync(timeLimitedCtx,
		cache.NewListWatchFromClient(
			clientset.AppsV1().RESTClient(), "daemonsets", namespace, fields.OneTermEqualSelector("metadata.name", dsObj.Name)),
		&appsv1.DaemonSet{},
		nil,
		func(event watch.Event) (bool, error) {
			ds := event.Object.(*appsv1.DaemonSet)
			return ds.Status.NumberReady > 0, nil
		},
	); watchErr != nil {
		return fmt.Errorf("daemonset %s didn't roll out: %v", dsObj.Name, watchErr)
	}
	return nil
}

func createCleanupDS(ctx context.Context, clientset kubernetes.Interface) error {
	dsObj := resourceread.ReadDaemonSetV1OrDie(dsCleanupYaml)
	dsObj.Namespace = namespace

	client := clientset.AppsV1().DaemonSets(namespace)
	var err error
	_, err = client.Create(ctx, dsObj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error creating daemonset: %v", err)
	}

	timeLimitedCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if _, watchErr := watchtools.UntilWithSync(timeLimitedCtx,
		cache.NewListWatchFromClient(
			clientset.AppsV1().RESTClient(), "daemonsets", namespace, fields.OneTermEqualSelector("metadata.name", dsObj.Name)),
		&appsv1.DaemonSet{},
		nil,
		func(event watch.Event) (bool, error) {
			ds := event.Object.(*appsv1.DaemonSet)
			return ds.Status.NumberReady > 0, nil
		},
	); watchErr != nil {
		return fmt.Errorf("daemonset %s didn't roll out: %v", dsObj.Name, watchErr)
	}
	return nil
}

func createServiceNetworkDS(ctx context.Context, clientset kubernetes.Interface, openshiftTestsImagePullSpec string) error {
	dsObj := resourceread.ReadDaemonSetV1OrDie(dsServiceNetworkYaml)
	dsObj.Namespace = namespace
	// we need to use the openshift-tests image of the destination during an upgrade.
	dsObj.Spec.Template.Spec.Containers[0].Image = openshiftTestsImagePullSpec

	client := clientset.AppsV1().DaemonSets(namespace)
	var err error
	_, err = client.Create(ctx, dsObj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error creating daemonset: %v", err)
	}

	timeLimitedCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if _, watchErr := watchtools.UntilWithSync(timeLimitedCtx,
		cache.NewListWatchFromClient(
			clientset.AppsV1().RESTClient(), "daemonsets", namespace, fields.OneTermEqualSelector("metadata.name", dsObj.Name)),
		&appsv1.DaemonSet{},
		nil,
		func(event watch.Event) (bool, error) {
			ds := event.Object.(*appsv1.DaemonSet)
			return ds.Status.NumberReady > 0, nil
		},
	); watchErr != nil {
		return fmt.Errorf("daemonset %s didn't roll out: %v", dsObj.Name, watchErr)
	}
	return nil
}

func createLocalhostDS(ctx context.Context, clientset kubernetes.Interface, openshiftTestsImagePullSpec string) error {
	dsObj := resourceread.ReadDaemonSetV1OrDie(dsLocalhostYaml)
	dsObj.Namespace = namespace
	// we need to use the openshift-tests image of the destination during an upgrade.
	dsObj.Spec.Template.Spec.Containers[0].Image = openshiftTestsImagePullSpec

	client := clientset.AppsV1().DaemonSets(namespace)
	var err error
	_, err = client.Create(ctx, dsObj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error creating daemonset: %v", err)
	}

	timeLimitedCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if _, watchErr := watchtools.UntilWithSync(timeLimitedCtx,
		cache.NewListWatchFromClient(
			clientset.AppsV1().RESTClient(), "daemonsets", namespace, fields.OneTermEqualSelector("metadata.name", dsObj.Name)),
		&appsv1.DaemonSet{},
		nil,
		func(event watch.Event) (bool, error) {
			ds := event.Object.(*appsv1.DaemonSet)
			return ds.Status.NumberReady > 0, nil
		},
	); watchErr != nil {
		return fmt.Errorf("daemonset %s didn't roll out: %v", dsObj.Name, watchErr)
	}
	return nil
}

func createRBACPrivileged(ctx context.Context, clientset kubernetes.Interface) error {
	rbacPrivilegedObj := resourceread.ReadClusterRoleBindingV1OrDie(rbacPrivilegedYaml)
	rbacPrivilegedObj.Subjects[0].Namespace = namespace

	client := clientset.RbacV1().ClusterRoleBindings()
	_, err := client.Create(ctx, rbacPrivilegedObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating privileged SCC CRB: %v", err)
	}
	rbacPrivilegedCRBName = rbacPrivilegedObj.Name
	return nil
}

func createMonitorRole(ctx context.Context, clientset kubernetes.Interface) error {
	rbacMonitorRoleObj := resourceread.ReadClusterRoleV1OrDie(rbacMonitorRoleYaml)
	rbacMonitorRoleName = rbacMonitorRoleObj.Name

	client := clientset.RbacV1().ClusterRoles()
	_, err := client.Create(ctx, rbacMonitorRoleObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating oauthclients list role: %v", err)
	}
	rbacMonitorRoleName = rbacMonitorRoleObj.Name
	return nil
}

func createMonitorCRB(ctx context.Context, clientset kubernetes.Interface) error {
	rbacMonitorCRBObj := resourceread.ReadClusterRoleBindingV1OrDie(rbacListOauthClientCRBYaml)
	rbacMonitorCRBObj.Subjects[0].Namespace = namespace
	rbacMonitorCRBName = rbacMonitorCRBObj.Name

	client := clientset.RbacV1().ClusterRoleBindings()
	_, err := client.Create(ctx, rbacMonitorCRBObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating oauthclients list CRB: %v", err)
	}
	rbacMonitorCRBName = rbacMonitorCRBObj.Name
	return nil
}

func createServiceAccount(ctx context.Context, clientset kubernetes.Interface) error {
	serviceAccountObj := resourceread.ReadServiceAccountV1OrDie(serviceAccountYaml)
	serviceAccountObj.Namespace = namespace
	client := clientset.CoreV1().ServiceAccounts(namespace)
	_, err := client.Create(ctx, serviceAccountObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating service account: %v", err)
	}
	return nil
}

func createNamespace(ctx context.Context, clientset kubernetes.Interface) error {
	namespaceObj := resourceread.ReadNamespaceV1OrDie(namespaceYaml)

	client := clientset.CoreV1().Namespaces()
	actualNamespace, err := client.Create(ctx, namespaceObj, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating namespace: %v", err)
	}
	namespace = actualNamespace.Name
	return nil
}

type InvariantInClusterDisruption struct {
	getImagePullSpec monitortestframework.OpenshiftTestImageGetterFunc

	namespaceName        string
	adminRESTConfig      *rest.Config
	kubeClient           kubernetes.Interface
	payloadImagePullSpec string

	notSupportedReason string
}

func NewInvariantInClusterDisruption(info monitortestframework.MonitorTestInitializationInfo) monitortestframework.MonitorTest {
	return &InvariantInClusterDisruption{
		getImagePullSpec: info.GetOpenshiftTestsImagePullSpec,
	}
}

func (i *InvariantInClusterDisruption) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, _ monitorapi.RecorderWriter) error {
	openshiftTestsImagePullSpec, notSupportedReason, err := i.getImagePullSpec(ctx, adminRESTConfig)
	if err != nil {
		return err
	}
	if len(notSupportedReason) > 0 {
		i.notSupportedReason = notSupportedReason
		return nil
	}

	i.kubeClient, err = kubernetes.NewForConfig(i.adminRESTConfig)
	if err != nil {
		return err
	}

	if ok, _ := exutil.IsMicroShiftCluster(i.kubeClient); ok {
		i.notSupportedReason = "microshift clusters don't have load balancers"
		return nil
	}

	fmt.Fprintf(os.Stderr, "Starting in-cluster monitoring daemonsets\n")
	i.adminRESTConfig = adminRESTConfig
	configClient, err := configclient.NewForConfig(i.adminRESTConfig)
	if err != nil {
		return err
	}
	infra, err := configClient.ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return err
	}

	internalAPI, err := url.Parse(infra.Status.APIServerInternalURL)
	if err != nil {
		return err
	}
	apiIntHost := internalAPI.Hostname()

	err = createNamespace(ctx, i.kubeClient)
	if err != nil {
		return err
	}
	// TODO real create
	i.namespaceName = namespace

	err = createServiceAccount(ctx, i.kubeClient)
	if err != nil {
		return err
	}
	err = createRBACPrivileged(ctx, i.kubeClient)
	if err != nil {
		return err
	}
	err = createMonitorRole(ctx, i.kubeClient)
	if err != nil {
		return err
	}
	err = createMonitorCRB(ctx, i.kubeClient)
	if err != nil {
		return err
	}
	err = createServiceNetworkDS(ctx, i.kubeClient, openshiftTestsImagePullSpec)
	if err != nil {
		return err
	}
	err = createLocalhostDS(ctx, i.kubeClient, openshiftTestsImagePullSpec)
	if err != nil {
		return err
	}
	err = createInternalLBDS(ctx, i.kubeClient, apiIntHost, openshiftTestsImagePullSpec)
	if err != nil {
		return err
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

	fmt.Fprintf(os.Stderr, "Collecting data from in-cluster monitoring daemonsets\n")

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
	kubeClient, err := kubernetes.NewForConfig(i.adminRESTConfig)
	if err != nil {
		return err
	}
	nsClient := kubeClient.CoreV1().Namespaces()
	err = nsClient.Delete(ctx, namespace, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error removing namespace %s: %v", namespace, err)
	}

	crbClient := kubeClient.RbacV1().ClusterRoleBindings()
	err = crbClient.Delete(ctx, rbacPrivilegedCRBName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error removing cluster reader CRB: %v", err)
	}

	err = crbClient.Delete(ctx, rbacMonitorCRBName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error removing monitor CRB: %v", err)
	}

	rolesClient := kubeClient.RbacV1().ClusterRoles()
	err = rolesClient.Delete(ctx, rbacMonitorRoleName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error removing monitor role: %v", err)
	}
	return nil
}

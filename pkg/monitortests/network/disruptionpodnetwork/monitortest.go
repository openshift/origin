package disruptionpodnetwork

import (
	"context"
	"embed"
	_ "embed"
	"fmt"
	"time"

	"github.com/google/uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	k8simage "k8s.io/kubernetes/test/utils/image"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/disruptionlibrary"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

var (
	//go:embed *.yaml
	yamls embed.FS

	namespace                                *corev1.Namespace
	pollerRoleBinding                        *rbacv1.RoleBinding
	podNetworkToPodNetworkPollerDeployment   *appsv1.Deployment
	podNetworkToHostNetworkPollerDeployment  *appsv1.Deployment
	hostNetworkToPodNetworkPollerDeployment  *appsv1.Deployment
	hostNetworkToHostNetworkPollerDeployment *appsv1.Deployment
	podNetworkServicePollerDep               *appsv1.Deployment
	hostNetworkServicePollerDep              *appsv1.Deployment
	podNetworkTargetDeployment               *appsv1.Deployment
	podNetworkTargetService                  *corev1.Service
	hostNetworkTargetDeployment              *appsv1.Deployment
	hostNetworkTargetService                 *corev1.Service
)

func yamlOrDie(name string) []byte {
	ret, err := yamls.ReadFile(name)
	if err != nil {
		panic(err)
	}

	return ret
}

func init() {
	namespace = resourceread.ReadNamespaceV1OrDie(yamlOrDie("namespace.yaml"))
	pollerRoleBinding = resourceread.ReadRoleBindingV1OrDie(yamlOrDie("poller-rolebinding.yaml"))
	podNetworkToPodNetworkPollerDeployment = resourceread.ReadDeploymentV1OrDie(yamlOrDie("pod-network-to-pod-network-poller-deployment.yaml"))
	podNetworkToHostNetworkPollerDeployment = resourceread.ReadDeploymentV1OrDie(yamlOrDie("pod-network-to-host-network-poller-deployment.yaml"))
	hostNetworkToPodNetworkPollerDeployment = resourceread.ReadDeploymentV1OrDie(yamlOrDie("host-network-to-pod-network-poller-deployment.yaml"))
	hostNetworkToHostNetworkPollerDeployment = resourceread.ReadDeploymentV1OrDie(yamlOrDie("host-network-to-host-network-poller-deployment.yaml"))
	podNetworkServicePollerDep = resourceread.ReadDeploymentV1OrDie(yamlOrDie("pod-network-to-service-poller-deployment.yaml"))
	hostNetworkServicePollerDep = resourceread.ReadDeploymentV1OrDie(yamlOrDie("host-network-to-service-poller-deployment.yaml"))
	podNetworkTargetDeployment = resourceread.ReadDeploymentV1OrDie(yamlOrDie("pod-network-target-deployment.yaml"))
	podNetworkTargetService = resourceread.ReadServiceV1OrDie(yamlOrDie("pod-network-target-service.yaml"))
	hostNetworkTargetDeployment = resourceread.ReadDeploymentV1OrDie(yamlOrDie("host-network-target-deployment.yaml"))
	hostNetworkTargetService = resourceread.ReadServiceV1OrDie(yamlOrDie("host-network-target-service.yaml"))
}

type podNetworkAvalibility struct {
	payloadImagePullSpec string
	notSupportedReason   error
	namespaceName        string
	targetService        *corev1.Service
	kubeClient           kubernetes.Interface
}

func NewPodNetworkAvalibilityInvariant(info monitortestframework.MonitorTestInitializationInfo) monitortestframework.MonitorTest {
	return &podNetworkAvalibility{
		payloadImagePullSpec: info.UpgradeTargetPayloadImagePullSpec,
	}
}

func (pna *podNetworkAvalibility) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	deploymentID := uuid.New().String()

	openshiftTestsImagePullSpec, err := GetOpenshiftTestsImagePullSpec(ctx, adminRESTConfig, pna.payloadImagePullSpec, nil)
	if err != nil {
		pna.notSupportedReason = &monitortestframework.NotSupportedError{Reason: fmt.Sprintf("unable to determine openshift-tests image: %v", err)}
		return pna.notSupportedReason
	}

	// Skip on ROSA TRT-1869
	oc := util.NewCLIWithoutNamespace("openshift-tests")
	isManagedServiceCluster, err := util.IsManagedServiceCluster(ctx, oc.AdminKubeClient())
	if isManagedServiceCluster {
		pna.notSupportedReason = &monitortestframework.NotSupportedError{Reason: fmt.Sprintf("pod network tests are unschedulable on ROSA TRT-1869")}
		return pna.notSupportedReason
	}

	pna.kubeClient, err = kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	actualNamespace, err := pna.kubeClient.CoreV1().Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	pna.namespaceName = actualNamespace.Name

	if _, err = pna.kubeClient.RbacV1().RoleBindings(pna.namespaceName).Create(context.Background(), pollerRoleBinding, metav1.CreateOptions{}); err != nil {
		return err
	}

	// our pods tolerate masters, so create one for each of them.
	nodes, err := pna.kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	numNodes := int32(len(nodes.Items))

	klog.Infof("Starting deployment: %s", podNetworkToPodNetworkPollerDeployment.Name)
	podNetworkToPodNetworkPollerDeployment.Spec.Replicas = &numNodes
	podNetworkToPodNetworkPollerDeployment.Spec.Template.Spec.Containers[0].Image = openshiftTestsImagePullSpec
	podNetworkToPodNetworkPollerDeployment = disruptionlibrary.UpdateDeploymentENVs(podNetworkToPodNetworkPollerDeployment, deploymentID, "")
	if _, err = pna.kubeClient.AppsV1().Deployments(pna.namespaceName).Create(context.Background(), podNetworkToPodNetworkPollerDeployment, metav1.CreateOptions{}); err != nil {
		return err
	}
	klog.Infof("Starting deployment: %s", podNetworkToHostNetworkPollerDeployment.Name)
	podNetworkToHostNetworkPollerDeployment.Spec.Replicas = &numNodes
	podNetworkToHostNetworkPollerDeployment.Spec.Template.Spec.Containers[0].Image = openshiftTestsImagePullSpec
	podNetworkToHostNetworkPollerDeployment = disruptionlibrary.UpdateDeploymentENVs(podNetworkToHostNetworkPollerDeployment, deploymentID, "")
	if _, err = pna.kubeClient.AppsV1().Deployments(pna.namespaceName).Create(context.Background(), podNetworkToHostNetworkPollerDeployment, metav1.CreateOptions{}); err != nil {
		return err
	}
	klog.Infof("Starting deployment: %s", hostNetworkToPodNetworkPollerDeployment.Name)
	hostNetworkToPodNetworkPollerDeployment.Spec.Replicas = &numNodes
	hostNetworkToPodNetworkPollerDeployment.Spec.Template.Spec.Containers[0].Image = openshiftTestsImagePullSpec
	hostNetworkToPodNetworkPollerDeployment = disruptionlibrary.UpdateDeploymentENVs(hostNetworkToPodNetworkPollerDeployment, deploymentID, "")
	if _, err = pna.kubeClient.AppsV1().Deployments(pna.namespaceName).Create(context.Background(), hostNetworkToPodNetworkPollerDeployment, metav1.CreateOptions{}); err != nil {
		return err
	}
	klog.Infof("Starting deployment: %s", hostNetworkToHostNetworkPollerDeployment.Name)
	hostNetworkToHostNetworkPollerDeployment.Spec.Replicas = &numNodes
	hostNetworkToHostNetworkPollerDeployment.Spec.Template.Spec.Containers[0].Image = openshiftTestsImagePullSpec
	hostNetworkToHostNetworkPollerDeployment = disruptionlibrary.UpdateDeploymentENVs(hostNetworkToHostNetworkPollerDeployment, deploymentID, "")
	if _, err = pna.kubeClient.AppsV1().Deployments(pna.namespaceName).Create(context.Background(), hostNetworkToHostNetworkPollerDeployment, metav1.CreateOptions{}); err != nil {
		return err
	}

	klog.Infof("Starting deployment: %s", podNetworkTargetDeployment.Name)
	// force the image to use the "normal" global mapping.
	originalAgnhost := k8simage.GetOriginalImageConfigs()[k8simage.Agnhost]
	podNetworkTargetDeployment.Spec.Replicas = &numNodes
	podNetworkTargetDeployment.Spec.Template.Spec.Containers[0].Image = image.LocationFor(originalAgnhost.GetE2EImage())
	if _, err := pna.kubeClient.AppsV1().Deployments(pna.namespaceName).Create(context.Background(), podNetworkTargetDeployment, metav1.CreateOptions{}); err != nil {
		return err
	}
	service, err := pna.kubeClient.CoreV1().Services(pna.namespaceName).Create(context.Background(), podNetworkTargetService, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	pna.targetService = service

	klog.Infof("Starting deployment: %s", hostNetworkTargetDeployment.Name)
	hostNetworkTargetDeployment.Spec.Replicas = &numNodes
	hostNetworkTargetDeployment.Spec.Template.Spec.Containers[0].Image = openshiftTestsImagePullSpec
	if _, err := pna.kubeClient.AppsV1().Deployments(pna.namespaceName).Create(context.Background(), hostNetworkTargetDeployment, metav1.CreateOptions{}); err != nil {
		return err
	}
	if _, err := pna.kubeClient.CoreV1().Services(pna.namespaceName).Create(context.Background(), hostNetworkTargetService, metav1.CreateOptions{}); err != nil {
		return err
	}

	// we need to have the service network pollers wait until we have at least one healthy endpoint before starting.
	err = wait.PollUntilContextTimeout(ctx, 1*time.Second, 300*time.Second, true, pna.serviceHasEndpoints)
	if err != nil {
		return err
	}

	for _, deployment := range []*appsv1.Deployment{podNetworkServicePollerDep, hostNetworkServicePollerDep} {
		klog.Infof("Starting deployment: %s", deployment.Name)
		deployment.Spec.Replicas = &numNodes
		deployment.Spec.Template.Spec.Containers[0].Image = openshiftTestsImagePullSpec
		deployment = disruptionlibrary.UpdateDeploymentENVs(deployment, deploymentID, service.Spec.ClusterIP)
		if _, err = pna.kubeClient.AppsV1().Deployments(pna.namespaceName).Create(context.Background(), deployment, metav1.CreateOptions{}); err != nil {
			return err
		}
	}

	// allow time to initialize
	// though we do that above as well...
	time.Sleep(30 * time.Second)

	return nil
}

func (pna *podNetworkAvalibility) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (pna *podNetworkAvalibility) serviceHasEndpoints(ctx context.Context) (bool, error) {
	targetServiceLabel, err := labels.NewRequirement("kubernetes.io/service-name", selection.Equals, []string{pna.targetService.Name})
	if err != nil {
		return false, err
	}
	listOptions := metav1.ListOptions{
		LabelSelector: labels.NewSelector().Add(*targetServiceLabel).String(),
	}
	endpointSlices, err := pna.kubeClient.DiscoveryV1().EndpointSlices(pna.targetService.Namespace).List(ctx, listOptions)
	if err != nil {
		klog.Errorf(err.Error())
		return false, nil
	}

	for _, endpointSlice := range endpointSlices.Items {
		for _, endpoint := range endpointSlice.Endpoints {
			if endpoint.Conditions.Serving != nil && *endpoint.Conditions.Serving {
				// we have at least one endpoint
				return true, nil
			}
		}
	}

	return false, nil
}

func (pna *podNetworkAvalibility) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if pna.notSupportedReason != nil {
		return nil, nil, pna.notSupportedReason
	}

	// create the stop collecting configmap and wait for 30s to thing to have stopped.  the 30s is just a guess
	if _, err := pna.kubeClient.CoreV1().ConfigMaps(pna.namespaceName).Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "stop-collecting"},
	}, metav1.CreateOptions{}); err != nil {
		return nil, nil, err
	}

	select {
	case <-time.After(30 * time.Second):
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}

	retIntervals := monitorapi.Intervals{}
	junits := []*junitapi.JUnitTestCase{}
	errs := []error{}
	for _, typeOfConnection := range []string{"pod-to-pod", "pod-to-host", "host-to-pod", "host-to-host", "pod-to-service", "host-to-service"} {
		localIntervals, localJunit, localErrs := pna.collectDetailsForPoller(ctx, typeOfConnection)
		retIntervals = append(retIntervals, localIntervals...)
		junits = append(junits, localJunit...)
		errs = append(errs, localErrs...)

	}

	return retIntervals, junits, utilerrors.NewAggregate(errs)
}

func (pna *podNetworkAvalibility) collectDetailsForPoller(ctx context.Context, typeOfConnection string) (monitorapi.Intervals, []*junitapi.JUnitTestCase, []error) {
	pollerLabel, err := labels.NewRequirement("network.openshift.io/disruption-actor", selection.Equals, []string{"poller"})
	if err != nil {
		return nil, nil, []error{err}
	}
	typeLabel, err := labels.NewRequirement("network.openshift.io/disruption-target", selection.Equals, []string{typeOfConnection})
	if err != nil {
		return nil, nil, []error{err}
	}
	labelSelector := labels.NewSelector().Add(*pollerLabel).Add(*typeLabel)
	return disruptionlibrary.CollectIntervalsForPods(ctx, pna.kubeClient, "sig-network", pna.namespaceName, labelSelector)
}

func (pna *podNetworkAvalibility) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (constructedIntervals monitorapi.Intervals, err error) {
	return nil, pna.notSupportedReason
}

func (pna *podNetworkAvalibility) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, pna.notSupportedReason
}

func (pna *podNetworkAvalibility) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return pna.notSupportedReason
}

func (pna *podNetworkAvalibility) namespaceDeleted(ctx context.Context) (bool, error) {
	_, err := pna.kubeClient.CoreV1().Namespaces().Get(ctx, pna.namespaceName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return true, nil
	}

	if err != nil {
		klog.Errorf("Error checking for deleted namespace: %s, %s", pna.namespaceName, err.Error())
		return false, err
	}

	return false, nil
}

func (pna *podNetworkAvalibility) Cleanup(ctx context.Context) error {
	if len(pna.namespaceName) > 0 && pna.kubeClient != nil {
		if err := pna.kubeClient.CoreV1().Namespaces().Delete(ctx, pna.namespaceName, metav1.DeleteOptions{}); err != nil {
			return err
		}

		startTime := time.Now()
		err := wait.PollUntilContextTimeout(ctx, 15*time.Second, 20*time.Minute, true, pna.namespaceDeleted)
		if err != nil {
			return err
		}

		klog.Infof("Deleting namespace: %s took %.2f seconds", pna.namespaceName, time.Now().Sub(startTime).Seconds())

	}
	return nil
}

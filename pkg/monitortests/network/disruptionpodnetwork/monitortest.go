package disruptionpodnetwork

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	_ "embed"
	"fmt"
	"os/exec"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	configclient "github.com/openshift/client-go/config/clientset/versioned"

	"github.com/openshift/origin/pkg/monitortestframework"

	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/openshift/origin/test/extended/util/image"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	k8simage "k8s.io/kubernetes/test/utils/image"
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
	notSupportedReason   string
	namespaceName        string
	kubeClient           kubernetes.Interface
}

func NewPodNetworkAvalibilityInvariant(info monitortestframework.MonitorTestInitializationInfo) monitortestframework.MonitorTest {
	return &podNetworkAvalibility{
		payloadImagePullSpec: info.UpgradeTargetPayloadImagePullSpec,
	}
}

func (pna *podNetworkAvalibility) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	if len(pna.payloadImagePullSpec) == 0 {
		configClient, err := configclient.NewForConfig(adminRESTConfig)
		if err != nil {
			return err
		}
		clusterVersion, err := configClient.ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			pna.notSupportedReason = "clusterversion/version not found and no image pull spec specified."
			return nil
		}
		if err != nil {
			return err
		}
		pna.payloadImagePullSpec = clusterVersion.Status.History[0].Image
	}

	// runImageExtract extracts src from specified image to dst
	cmd := exec.Command("oc", "adm", "release", "info", pna.payloadImagePullSpec, "--image-for=tests")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.Stdout = out
	cmd.Stderr = errOut
	if err := cmd.Run(); err != nil {
		pna.notSupportedReason = fmt.Sprintf("unable to determine openshift-tests image: %v: %v", err, errOut.String())
		return nil
	}
	openshiftTestsImagePullSpec := strings.TrimSpace(out.String())
	fmt.Printf("openshift-tests image pull spec is %v\n", openshiftTestsImagePullSpec)

	var err error

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

	podNetworkToPodNetworkPollerDeployment.Spec.Replicas = &numNodes
	podNetworkToPodNetworkPollerDeployment.Spec.Template.Spec.Containers[0].Image = openshiftTestsImagePullSpec
	if _, err = pna.kubeClient.AppsV1().Deployments(pna.namespaceName).Create(context.Background(), podNetworkToPodNetworkPollerDeployment, metav1.CreateOptions{}); err != nil {
		return err
	}
	podNetworkToHostNetworkPollerDeployment.Spec.Replicas = &numNodes
	podNetworkToHostNetworkPollerDeployment.Spec.Template.Spec.Containers[0].Image = openshiftTestsImagePullSpec
	if _, err = pna.kubeClient.AppsV1().Deployments(pna.namespaceName).Create(context.Background(), podNetworkToHostNetworkPollerDeployment, metav1.CreateOptions{}); err != nil {
		return err
	}
	hostNetworkToPodNetworkPollerDeployment.Spec.Replicas = &numNodes
	hostNetworkToPodNetworkPollerDeployment.Spec.Template.Spec.Containers[0].Image = openshiftTestsImagePullSpec
	if _, err = pna.kubeClient.AppsV1().Deployments(pna.namespaceName).Create(context.Background(), hostNetworkToPodNetworkPollerDeployment, metav1.CreateOptions{}); err != nil {
		return err
	}
	hostNetworkToHostNetworkPollerDeployment.Spec.Replicas = &numNodes
	hostNetworkToHostNetworkPollerDeployment.Spec.Template.Spec.Containers[0].Image = openshiftTestsImagePullSpec
	if _, err = pna.kubeClient.AppsV1().Deployments(pna.namespaceName).Create(context.Background(), hostNetworkToHostNetworkPollerDeployment, metav1.CreateOptions{}); err != nil {
		return err
	}

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

	hostNetworkTargetDeployment.Spec.Replicas = &numNodes
	if _, err := pna.kubeClient.AppsV1().Deployments(pna.namespaceName).Create(context.Background(), hostNetworkTargetDeployment, metav1.CreateOptions{}); err != nil {
		return err
	}
	if _, err := pna.kubeClient.CoreV1().Services(pna.namespaceName).Create(context.Background(), hostNetworkTargetService, metav1.CreateOptions{}); err != nil {
		return err
	}

	for _, deployment := range []*appsv1.Deployment{podNetworkServicePollerDep, hostNetworkServicePollerDep} {
		deployment.Spec.Replicas = &numNodes
		deployment.Spec.Template.Spec.Containers[0].Image = openshiftTestsImagePullSpec
		for i, env := range deployment.Spec.Template.Spec.Containers[0].Env {
			if env.Name == "SERVICE_CLUSTER_IP" {
				deployment.Spec.Template.Spec.Containers[0].Env[i].Value = service.Spec.ClusterIP
			}
		}
		if _, err = pna.kubeClient.AppsV1().Deployments(pna.namespaceName).Create(context.Background(), deployment, metav1.CreateOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (pna *podNetworkAvalibility) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if len(pna.notSupportedReason) > 0 {
		return nil, []*junitapi.JUnitTestCase{
			{
				Name: "[sig-network] can collect pod-to-pod network disruption",
				SkipMessage: &junitapi.SkipMessage{
					Message: pna.notSupportedReason,
				},
			},
		}, nil
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
	pollerPods, err := pna.kubeClient.CoreV1().Pods(pna.namespaceName).List(ctx, metav1.ListOptions{
		LabelSelector: labels.NewSelector().Add(*pollerLabel).Add(*typeLabel).String(),
	})
	if err != nil {
		return nil, nil, []error{err}
	}

	retIntervals := monitorapi.Intervals{}
	junits := []*junitapi.JUnitTestCase{}
	errs := []error{}
	buf := &bytes.Buffer{}
	podsWithoutIntervals := []string{}
	for _, pollerPod := range pollerPods.Items {
		fmt.Fprintf(buf, "\n\nLogs for -n %v pod/%v\n", pollerPod.Namespace, pollerPod.Name)
		req := pna.kubeClient.CoreV1().Pods(pna.namespaceName).GetLogs(pollerPod.Name, &corev1.PodLogOptions{})
		if err != nil {
			errs = append(errs, err)
			continue
		}
		logStream, err := req.Stream(ctx)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		foundInterval := false
		scanner := bufio.NewScanner(logStream)
		for scanner.Scan() {
			line := scanner.Bytes()
			buf.Write(line)
			buf.Write([]byte("\n"))
			if len(line) == 0 {
				continue
			}

			// not all lines are json, ignore errors.
			if currInterval, err := monitorserialization.IntervalFromJSON(line); err == nil {
				retIntervals = append(retIntervals, *currInterval)
				foundInterval = true
			}
		}
		if !foundInterval {
			podsWithoutIntervals = append(podsWithoutIntervals, pollerPod.Name)
		}
	}

	failures := []string{}
	if len(podsWithoutIntervals) > 0 {
		failures = append(failures, fmt.Sprintf("%d pods lacked sampler output: [%v]", len(podsWithoutIntervals), strings.Join(podsWithoutIntervals, ", ")))
	}
	if len(pollerPods.Items) == 0 {
		failures = append(failures, "no pods found for poller %q", typeOfConnection)
	}

	logJunit := &junitapi.JUnitTestCase{
		Name:      fmt.Sprintf("[sig-network] can collect %v poller pod logs", typeOfConnection),
		SystemOut: string(buf.Bytes()),
	}
	if len(failures) > 0 {
		logJunit.FailureOutput = &junitapi.FailureOutput{
			Output: strings.Join(failures, "\n"),
		}
	}
	junits = append(junits, logJunit)

	return retIntervals, junits, errs
}

func (pna *podNetworkAvalibility) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (constructedIntervals monitorapi.Intervals, err error) {
	return nil, nil
}

func (pna *podNetworkAvalibility) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (pna *podNetworkAvalibility) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (pna *podNetworkAvalibility) Cleanup(ctx context.Context) error {
	if len(pna.namespaceName) > 0 && pna.kubeClient != nil {
		if err := pna.kubeClient.CoreV1().Namespaces().Delete(ctx, pna.namespaceName, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}
	return nil
}

package disruptionpodnetwork

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	_ "embed"
	"fmt"
	"time"

	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	"github.com/openshift/origin/pkg/invariants"
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

	namespace                  *corev1.Namespace
	pollerRoleBinding          *rbacv1.RoleBinding
	podNetworkPollerDeployment *appsv1.Deployment
	podNetworkTargetDeployment *appsv1.Deployment
	podNetworkTargetService    *corev1.Service
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
	podNetworkPollerDeployment = resourceread.ReadDeploymentV1OrDie(yamlOrDie("pod-network-poller-deployment.yaml"))
	podNetworkTargetDeployment = resourceread.ReadDeploymentV1OrDie(yamlOrDie("pod-network-target-deployment.yaml"))
	podNetworkTargetService = resourceread.ReadServiceV1OrDie(yamlOrDie("pod-network-target-service.yaml"))
}

type podNetworkAvalibility struct {
	namespaceName string
	kubeClient    kubernetes.Interface
}

func NewPodNetworkAvalibilityInvariant() invariants.InvariantTest {
	return &podNetworkAvalibility{}
}

func (pna *podNetworkAvalibility) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
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

	// disable the poller until can get the image sorted out.
	// TODO re-enable
	if false {
		podNetworkPollerDeployment.Spec.Replicas = &numNodes
		if _, err = pna.kubeClient.AppsV1().Deployments(pna.namespaceName).Create(context.Background(), podNetworkPollerDeployment, metav1.CreateOptions{}); err != nil {
			return err
		}
	}

	// force the image to use the "normal" global mapping.
	originalAgnhost := k8simage.GetOriginalImageConfigs()[k8simage.Agnhost]
	podNetworkTargetDeployment.Spec.Replicas = &numNodes
	podNetworkTargetDeployment.Spec.Template.Spec.Containers[0].Image = image.LocationFor(originalAgnhost.GetE2EImage())
	if _, err := pna.kubeClient.AppsV1().Deployments(pna.namespaceName).Create(context.Background(), podNetworkTargetDeployment, metav1.CreateOptions{}); err != nil {
		return err
	}

	if _, err := pna.kubeClient.CoreV1().Services(pna.namespaceName).Create(context.Background(), podNetworkTargetService, metav1.CreateOptions{}); err != nil {
		return err
	}

	return nil
}

func (pna *podNetworkAvalibility) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	pollerLabel, err := labels.NewRequirement("network.openshift.io/disruption-actor", selection.Equals, []string{"poller"})
	if err != nil {
		return nil, nil, err
	}
	pollerPods, err := pna.kubeClient.CoreV1().Pods(pna.namespaceName).List(ctx, metav1.ListOptions{
		LabelSelector: labels.NewSelector().Add(*pollerLabel).String(),
	})
	if err != nil {
		return nil, nil, err
	}

	retIntervals := monitorapi.Intervals{}
	junits := []*junitapi.JUnitTestCase{}
	errs := []error{}
	buf := &bytes.Buffer{}
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

		scanner := bufio.NewScanner(logStream)
		for scanner.Scan() {
			line := scanner.Bytes()
			fmt.Fprintln(buf, line)
			if len(line) == 0 {
				continue
			}

			// not all lines are json, ignore errors.
			if currInterval, err := monitorserialization.IntervalFromJSON(line); err == nil {
				retIntervals = append(retIntervals, *currInterval)
			}
		}
	}
	junits = append(junits, &junitapi.JUnitTestCase{
		Name:      "[sig-network] poller pod logs",
		SystemOut: string(buf.Bytes()),
	})

	return retIntervals, junits, utilerrors.NewAggregate(errs)
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

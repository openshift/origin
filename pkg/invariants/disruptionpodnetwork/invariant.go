package disruptionpodnetwork

import (
	"context"
	_ "embed"
	"time"

	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	"github.com/openshift/origin/pkg/invariants"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	// openshift-ovn-kubernetes will be supported ongoing so that is the JIRA owner for now
	JIRAOwner     = "Network / ovn-kubernetes"
	InvariantName = "pod-network-avalibility"
)

var (
	//go:embed namespace.yaml
	namespaceYaml []byte
	namespace     *corev1.Namespace
)

func init() {
	namespace = resourceread.ReadNamespaceV1OrDie(namespaceYaml)
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

	return nil
}

func (pna *podNetworkAvalibility) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
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

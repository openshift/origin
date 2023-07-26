package cpu_partitioning

import (
	"context"

	o "github.com/onsi/gomega"
	t "github.com/onsi/gomega/types"

	v1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	resourceLabel            = "management.workload.openshift.io/cores"
	namespaceAnnotationKey   = "workload.openshift.io/allowed"
	workloadAnnotations      = "target.workload.openshift.io/management"
	WorkloadAnnotationPrefix = "resources.workload.openshift.io"
	workloadAnnotationsRegex = WorkloadAnnotationPrefix + "/.*"

	expectedMessage    = "expected to"
	notExpectedMessage = "expected not to"

	namespaceMachineConfigOperator = "openshift-machine-config-operator"
	nodeWorkerLabel                = "node-role.kubernetes.io/worker"
	nodeMasterLabel                = "node-role.kubernetes.io/master"
)

var (
	namespaceAnnotation     = map[string]string{namespaceAnnotationKey: "management"}
	deploymentPodAnnotation = map[string]string{workloadAnnotations: `{"effect": "PreferredDuringScheduling"}`}
)

func getCpuPartitionedStatus(oc *exutil.CLI) v1.CPUPartitioningMode {
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(),
		"cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	return infra.Status.CPUPartitioning
}

// adjustMatcherAndMessageForCluster will adjust the logic for the matcher depending on CPU partitioned state of the cluster
// and returns a helper message to use for logging.
func adjustMatcherAndMessageForCluster(isClusterCPUPartitioned bool, matcher t.GomegaMatcher) (t.GomegaMatcher, string) {
	if !isClusterCPUPartitioned {
		return o.Not(matcher), notExpectedMessage
	}
	return matcher, expectedMessage
}

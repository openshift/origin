package cpu_partitioning

import (
	"context"

	o "github.com/onsi/gomega"

	v1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

const (
	resourceLabel            = "management.workload.openshift.io/cores"
	namespaceAnnotationKey   = "workload.openshift.io/allowed"
	workloadAnnotations      = "target.workload.openshift.io/management"
	workloadAnnotationsRegex = "resources.workload.openshift.io/.*"
)

var (
	namespaceAnnotation     = map[string]string{namespaceAnnotationKey: "management"}
	deploymentPodAnnotation = map[string]string{workloadAnnotations: `{"effect": "PreferredDuringScheduling"}`}
)

func skipNonCPUPartitionedCluster(oc *exutil.CLI) {
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(),
		"cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	if infra.Status.CPUPartitioning != v1.CPUPartitioningAllNodes {
		e2eskipper.Skipf("Tests are only valid for clusters with CPUPartitioning enabled.")
	}
}

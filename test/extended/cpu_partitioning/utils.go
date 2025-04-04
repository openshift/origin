package cpu_partitioning

import (
	"context"
	"encoding/json"
	"strings"

	o "github.com/onsi/gomega"
	t "github.com/onsi/gomega/types"

	v1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/openshift/origin/test/extended/util/image"
)

const (
	resourceLabel            = "management.workload.openshift.io/cores"
	namespaceAnnotationKey   = "workload.openshift.io/allowed"
	workloadAnnotations      = "target.workload.openshift.io/management"
	workloadAnnotationPrefix = "resources.workload.openshift.io"
	workloadAnnotationsRegex = workloadAnnotationPrefix + "/.*"

	expectedMessage    = "expected to"
	notExpectedMessage = "expected not to"

	namespaceMachineConfigOperator = "openshift-machine-config-operator"
	nodeWorkerLabel                = "node-role.kubernetes.io/worker"
	nodeMasterLabel                = "node-role.kubernetes.io/master"
	nodeArbiterLabel               = "node-role.kubernetes.io/arbiter"

	milliCPUToCPU = 1000
	// 100000 microseconds is equivalent to 100ms
	defaultQuotaPeriod = 100000
	// 1000 microseconds is equivalent to 1ms
	// defined here:
	// https://github.com/torvalds/linux/blob/cac03ac368fabff0122853de2422d4e17a32de08/kernel/sched/core.c#L10546
	minQuotaPeriod = 1000
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

// milliCPUToQuota converts milliCPU to CFS quota and period values.
// Input parameters and resulting value is number of microseconds.
func milliCPUToQuota(milliCPU, period int64) (quota int64) {
	if milliCPU == 0 {
		return quota
	}

	if period == 0 {
		period = defaultQuotaPeriod
	}

	// We then convert the milliCPU to a value normalized over a period.
	quota = (milliCPU * period) / milliCPUToCPU

	// quota needs to be a minimum of 1ms.
	if quota < minQuotaPeriod {
		quota = minQuotaPeriod
	}
	return quota
}

func createManagedDeployment(oc *exutil.CLI, requests, limits corev1.ResourceList) (*appv1.Deployment, error) {
	zero := int64(0)
	depLabels := map[string]string{"app": "workload"}
	name := "busy-box-deployment"

	container := corev1.Container{
		Name:  "busy-work",
		Image: image.ShellImage(),
		Command: []string{
			"/bin/bash",
			"-c",
			`while true; do echo "Busy working, cycling through the ones and zeros"; sleep 5; done`,
		},
	}

	if requests != nil {
		container.Resources.Requests = requests
	}
	if limits != nil {
		container.Resources.Limits = limits
	}

	deployment := &appv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: depLabels,
		},
		Spec: appv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: depLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      depLabels,
					Annotations: deploymentPodAnnotation,
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &zero,
					Containers: []corev1.Container{
						container,
					},
				},
			},
		},
	}

	return oc.KubeClient().AppsV1().
		Deployments(oc.Namespace()).
		Create(context.Background(), deployment, metav1.CreateOptions{})
}

func getWorkloadAnnotationResource(annotations map[string]string) (map[string]crioCPUResource, error) {
	resources := map[string]crioCPUResource{}
	for k, v := range annotations {
		if strings.Contains(k, workloadAnnotationPrefix) {
			r := crioCPUResource{}
			if err := json.Unmarshal([]byte(v), &r); err != nil {
				return resources, err
			}
			resources[strings.TrimPrefix(k, workloadAnnotationPrefix+"/")] = r
		}
	}
	return resources, nil
}

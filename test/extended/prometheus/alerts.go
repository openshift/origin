package prometheus

import (
	"time"

	g "github.com/onsi/ginkgo"

	v1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	intstr "k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"

	clientset "k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/pod"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:PodDisruptionBudgetAtLimitAlert][Conformance] Prometheus", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLIWithoutNamespace("prometheus")

		url, bearerToken string
	)

	g.BeforeEach(func() {
		var ok bool
		url, bearerToken, ok = locatePrometheus(oc)
		if !ok {
			e2e.Skipf("Prometheus could not be located on this cluster, skipping prometheus test")
		}
	})
	// This alert is managed by cluster-kube-controller-manager-operator
	// https://github.com/openshift/cluster-kube-controller-manager-operator/blob/master/manifests/0000_90_kube-controller-manager-operator_05_alert-pdb.yaml
	// Check for 'pending' rather than 'firing' because alert will remain pending for 15m according to the alert definition above.
	g.Describe("when installed on the cluster", func() {
		g.It("should have a PodDisruptionBudgetAtLimit alert in pending state if pdbMinAvailable exists and MinAvailable pods", func() {
			oc.SetupProject()
			ns := oc.Namespace()
			labels := map[string]string{"app": "pdbtest"}
			execPod := createPodOrFail(oc.AdminKubeClient(), ns, "execpod", labels)
			defer func() { oc.AdminKubeClient().CoreV1().Pods(ns).Delete(execPod.Name, metav1.NewDeleteOptions(1)) }()
			pdbCreateMinAvailable(oc, ns, labels)

			tests := map[string]bool{
				// should have pdb alert if pdb created and at limit
				`ALERTS{alertstate="pending",alertname="PodDisruptionBudgetAtLimit",severity="warning"} == 1`: true,
			}
			runQueries(tests, oc, ns, execPod.Name, url, bearerToken)

			e2e.Logf("PodDisruptionBudget alert is firing")
		})
	})
})

func pdbCreateMinAvailable(oc *exutil.CLI, ns string, labels map[string]string) {
	minAvailable := intstr.FromInt(1)
	pdb := policyv1beta1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
		Spec: policyv1beta1.PodDisruptionBudgetSpec{
			MinAvailable: &minAvailable,
			Selector:     &metav1.LabelSelector{MatchLabels: labels},
		},
	}
	_, err := oc.AdminPolicyClient().PodDisruptionBudgets(ns).Create(&pdb)
	e2e.ExpectNoError(err, "Waiting for the pdb to be created with minAvailable %d in namespace %s", minAvailable, ns)
	wait.PollImmediate(10*time.Second, 4*time.Minute, func() (bool, error) {
		pdb, err := oc.AdminPolicyClient().PodDisruptionBudgets(ns).Get(ns, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if pdb.Status.ObservedGeneration < pdb.Generation {
			return false, nil
		}
		return true, nil
	})
	e2e.ExpectNoError(err, "Waiting for the pdb in namespace %s", ns)
}

func createPodOrFail(client clientset.Interface, ns, generateName string, labels map[string]string) *v1.Pod {
	return pod.CreateExecPodOrFail(client, ns, generateName, func(pod *v1.Pod) {
		pod.ObjectMeta.Labels = labels
		pod.Spec.Containers[0].Image = "centos:7"
		pod.Spec.Containers[0].Command = []string{"sh", "-c", "trap exit TERM; while true; do sleep 5; done"}
		pod.Spec.Containers[0].Args = nil

	})
}

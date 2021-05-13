package pods

import (
	"context"
	"fmt"
	"k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"reflect"
	"strings"

	g "github.com/onsi/ginkgo"
	oapi "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// getAllControlPlaneStaticPods gets the static pod
func getAllControlPlaneStaticPods(kubeClient kubernetes.Interface) (sets.String, error) {
	pod, err := kubeClient.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var controlPlanePods = sets.NewString()
	for _, pod := range pod.Items {
		_, isStaticPod := pod.Annotations["kubernetes.io/config.mirror"]
		_, isControlPlanePod := pod.Annotations["target.workload.openshift.io/management"]
		if isStaticPod && isControlPlanePod && strings.Contains(pod.Namespace, "openshift-") {
			staticPodName := strings.Split(pod.Name, "-ip")[0]
			if staticPodName == "etcd" {
				// etcd pod name needs to be appended with -quorum as it has quorum guard associated
				staticPodName = staticPodName+"-quorum"
			}
			controlPlanePods.Insert(staticPodName)
		}
	}
	return controlPlanePods, nil
}

var _ = g.Describe("[sig-arch] Managed cluster", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("ensure-pdb")

	g.It("all platform components should have PDBs configured", func() {
		kubeClient := oc.AdminKubeClient()

		deployments, err := kubeClient.AppsV1().Deployments("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			e2e.Failf("unable to list deployments: %v", err)
		}
		statefulsets, err := kubeClient.AppsV1().StatefulSets("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			e2e.Failf("unable to list statefulsets: %v", err)
		}
		// ovn-kube, openshift-controller-manager are running as DS.
		daemonsets, err := kubeClient.AppsV1().DaemonSets("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			e2e.Failf("unable to list daemonsets: %v", err)
		}

		var items []runtime.Object
		for i := range deployments.Items {
			items = append(items, &deployments.Items[i])
		}
		for i := range statefulsets.Items {
			items = append(items, &statefulsets.Items[i])
		}
		for i := range daemonsets.Items {
			items = append(items, &daemonsets.Items[i])
		}
		// iterate over the references to find valid images
		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		if err != nil {
			e2e.Failf("unable to get infrastructure: %v", err)
		}
		var inHAMode bool
		if infra.Status.InfrastructureTopology == oapi.HighlyAvailableTopologyMode {
			inHAMode = true
		}
		//controlPlaneStaticPodNames := sets.NewString("etcd", "kube-apiserver", "kube-controller-manager",
		//	"openshift-kube-scheduler")
		controlPlaneStaticPodNames, err := getAllControlPlaneStaticPods(kubeClient)
		if err != nil {
			e2e.Failf("unable to get names of static pods: %v", err)
		}
		// knownBugs := map[string]string{} // Create bugs in future
		pdbMissingWorkloads := []string{}
		pdbMisconfiguredWorkloads := []string{}
		operatorsHavingPDBS := []string{}
		for _, workload := range items {
			var meta *metav1.ObjectMeta
			var selectorLabels *metav1.LabelSelector
			var replicas int32
			switch t := workload.(type) {
			case *appsv1.Deployment:
				t.Kind = "Deployment"
				meta = &t.ObjectMeta
				selectorLabels = t.Spec.Selector
				replicas = t.Status.Replicas
			case *appsv1.StatefulSet:
				t.Kind = "StatefulSet"
				meta = &t.ObjectMeta
				selectorLabels = t.Spec.Selector
				replicas = t.Status.Replicas
			case *appsv1.DaemonSet:
				t.Kind = "DaemonSet"
				meta = &t.ObjectMeta
				selectorLabels = t.Spec.Selector
				replicas = t.Status.DesiredNumberScheduled
			default:
				panic("not an object")
			}
			if !strings.Contains(meta.Namespace, "openshift-") {
				// Not every component is following the convention of -operator at the end, there are some namespaces
				// like openshift-monitoring and openshift-network-diagnostics and some of them run on worker nodes
				// as well. So, need to figure out how to exclude operator components and components running on worker node
				continue
			}

			key := fmt.Sprintf("%s/%s/%s", workload.GetObjectKind().GroupVersionKind().Kind, meta.Namespace, meta.Name)
			var pdbs *v1beta1.PodDisruptionBudgetList
			isPDBConfigured := false
			var currentPDB v1beta1.PodDisruptionBudget
			pdbs, err = kubeClient.PolicyV1beta1().PodDisruptionBudgets(meta.Namespace).List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				e2e.Failf("Unexpected error getting PDBs: %v", err)
			}
			for _, pdb := range pdbs.Items {
				if reflect.DeepEqual(pdb.Spec.Selector, selectorLabels) {
					isPDBConfigured = true
					currentPDB = pdb
					operatorsHavingPDBS = append(operatorsHavingPDBS, key)
					// The convention we came up with for static pod control plane components is to have a deployment
					// doing health check for the corresponding component. This can be changed to labels later.
					// As of now, I am assuming if the deployment name has `-guard` in the name and it runs in
					// `openshift-*` namespace, it can be considered a deployment guarding control plane static pod
					if strings.Contains(meta.Name, "-guard") &&
						controlPlaneStaticPodNames.Has(meta.Name[:strings.LastIndex(meta.Name, "-")]) {
						e2e.Logf("The workload %s is having a PDB %s to monitor the health of control plane component %s",
							key, pdb.Name, meta.Name[:strings.LastIndex(meta.Name, "-")])
					} else {
						e2e.Logf("Workload %s has PDB %s configured", key, pdb.Name)
					}
					break
				}
			}
			if inHAMode {
				key = "HA/" + key
				if !isPDBConfigured {
					pdbMissingWorkloads = append(pdbMissingWorkloads, key)

				} else if isPDBConfigured && replicas > 2 {
					// We expect atleast 1 pod to be available while evicting/draining.
					if currentPDB.Spec.MaxUnavailable != nil && currentPDB.Spec.MaxUnavailable.IntValue() < 1 {
						pdbMisconfiguredWorkloads = append(pdbMisconfiguredWorkloads, key)
					} else if currentPDB.Spec.MinAvailable != nil && currentPDB.Spec.MinAvailable.IntValue() < 1 {
						pdbMisconfiguredWorkloads = append(pdbMisconfiguredWorkloads, key)
					}
				}
			} else {
				// We don't do upgrades in case of SNO but to be future proof, we are adding this case
				key = "non-HA/" + key
				if !isPDBConfigured {
					e2e.Logf("Workload %s doesn't have any PDBs associated which is fine in case of SNO", key)
				} else {
					// In case of SNO, if minAvailable replicas are greater than or equal to 1, we cannot
					// drain causing upgrades to fail.
					if currentPDB.Spec.MinAvailable.IntValue() >= 1 {
						pdbMisconfiguredWorkloads = append(pdbMisconfiguredWorkloads, key)
					}
				}
			}
		}
		result.Flakef("Workloads missing pdbs\n%s", strings.Join(pdbMissingWorkloads, "\n"))
		result.Flakef("Workloads with misconfigured pdbs\n%s", strings.Join(pdbMisconfiguredWorkloads, "\n"))
	})
})

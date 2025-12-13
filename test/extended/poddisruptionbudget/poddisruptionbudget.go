package poddisruptionbudget

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	policyv1clientset "k8s.io/client-go/kubernetes/typed/policy/v1"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-apps] poddisruptionbudgets", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithPodSecurityLevel("poddisruptionbudgets", admissionapi.LevelRestricted)

	const (
		// should be higher than the pod start time, so first pod is still not ready when a seconds one starts with a big delay (might take a long time to pull image)
		podBecomesReadyTimeout = 5*time.Minute + 10*time.Second
	)
	const (
		ifHealthyBudgetPDBName   = "if-healthy-budget-policy"
		alwaysAllowPolicyPDBName = "always-allow-policy"
	)

	var (
		nginxWithDelayedReadyDeployment = exutil.FixturePath("testdata", "poddisruptionbudgets", "nginx-with-delayed-ready-deployment.yaml")
		ifHealthyBudgetPolicyPDB        = exutil.FixturePath("testdata", "poddisruptionbudgets", "if-healthy-budget-policy-pdb.yaml")
		alwaysAllowPolicyPDB            = exutil.FixturePath("testdata", "poddisruptionbudgets", "always-allow-policy-pdb.yaml")
	)

	g.Describe("with unhealthyPodEvictionPolicy", func() {
		var podsLabelSelector labels.Selector

		g.BeforeEach(func() {
			podsLabelSelector = labels.SelectorFromSet(labels.Set{"app": "nginx-with-delayed-ready"})
		})

		g.It(fmt.Sprintf("should evict according to the IfHealthyBudget policy"), g.Label("Size:L"), func() {
			g.By(fmt.Sprintf("calling oc create -f %q", ifHealthyBudgetPolicyPDB))
			err := oc.Run("create").Args("-f", ifHealthyBudgetPolicyPDB).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("calling oc create -f %q", nginxWithDelayedReadyDeployment))
			err = oc.Run("create").Args("-f", nginxWithDelayedReadyDeployment).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("wait for a PDB to initialize its status")
			expectedStatus := policyv1.PodDisruptionBudgetStatus{
				DisruptionsAllowed: 0,
				CurrentHealthy:     0,
				DesiredHealthy:     2,
				ExpectedPods:       3,
			}
			pdb, err := waitForPDBStatus(oc.KubeClient().PolicyV1(), metav1.ObjectMeta{Name: ifHealthyBudgetPDBName, Namespace: oc.Namespace()}, e2e.PodListTimeout, expectedStatus)
			if err != nil {
				g.Fail(fmt.Sprintf("error occurred while waiting for PDB status: %v\nexpected %#v, got %#v", err, expectedStatus, pdbStatus(pdb)))
			}

			g.By("wait for pods to be running")
			pods, err := exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), podsLabelSelector, exutil.CheckPodIsRunning, 3, e2e.PodStartTimeout)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("eviction should not be possible for unready pods guarded by a PDB when none is ready")
			for _, pod := range pods {
				eviction := newV1Eviction(oc.Namespace(), pod, metav1.DeleteOptions{})
				err := oc.KubeClient().PolicyV1().Evictions(oc.Namespace()).Evict(context.Background(), eviction)
				o.Expect(err).To(o.HaveOccurred())
			}

			g.By("wait for PDB to become stable")
			expectedStatus = policyv1.PodDisruptionBudgetStatus{
				DisruptionsAllowed: 1,
				CurrentHealthy:     3,
				DesiredHealthy:     2,
				ExpectedPods:       3,
			}
			pdb, err = waitForPDBStatus(oc.KubeClient().PolicyV1(), metav1.ObjectMeta{Name: ifHealthyBudgetPDBName, Namespace: oc.Namespace()}, e2e.PodStartTimeout+podBecomesReadyTimeout, expectedStatus)
			if err != nil {
				g.Fail(fmt.Sprintf("error occurred while waiting for PDB status: %v\nexpected %#v, got %#v", err, expectedStatus, pdbStatus(pdb)))
			}

			g.By("check pods are ready")
			pods, err = exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), podsLabelSelector, func(pod v1.Pod) bool {
				return exutil.CheckPodIsReady(pod) && pod.DeletionTimestamp == nil
			}, 3, e2e.PodStartTimeout+podBecomesReadyTimeout)

			shouldTerminatePods := sets.New[string]()

			g.By("only one disruption of ready pods allowed")
			firstToEvictPod := pods[0]
			eviction := newV1Eviction(oc.Namespace(), firstToEvictPod, metav1.DeleteOptions{})
			err = oc.KubeClient().PolicyV1().Evictions(oc.Namespace()).Evict(context.Background(), eviction)
			o.Expect(err).NotTo(o.HaveOccurred())
			shouldTerminatePods.Insert(firstToEvictPod)

			g.By("second eviction should not succeed")
			secondToEvictPod := pods[1]
			eviction = newV1Eviction(oc.Namespace(), secondToEvictPod, metav1.DeleteOptions{})
			err = oc.KubeClient().PolicyV1().Evictions(oc.Namespace()).Evict(context.Background(), eviction)
			o.Expect(err).To(o.HaveOccurred())

			g.By("wait for the new pod to be running (running but not ready)")
			pods, err = exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), podsLabelSelector, func(pod v1.Pod) bool {
				return exutil.CheckPodIsRunning(pod) && !exutil.CheckPodIsReady(pod) && pod.DeletionTimestamp == nil
			}, 1, e2e.PodStartTimeout)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("wait for PDB to update CurrentHealthy == 2 and DisruptionsAllowed == 0")
			expectedStatus = policyv1.PodDisruptionBudgetStatus{
				DisruptionsAllowed: 0,
				CurrentHealthy:     2,
				DesiredHealthy:     2,
				ExpectedPods:       3,
			}
			pdb, err = waitForPDBStatus(oc.KubeClient().PolicyV1(), metav1.ObjectMeta{Name: ifHealthyBudgetPDBName, Namespace: oc.Namespace()}, e2e.PodStartTimeout, expectedStatus)
			if err != nil {
				g.Fail(fmt.Sprintf("error occurred while waiting for PDB status: %v\nexpected %#v, got %#v", err, expectedStatus, pdbStatus(pdb)))
			}

			g.By("eviction of the new running but not ready pod should succeed")
			thirdNewToEvictPod := pods[0]
			eviction = newV1Eviction(oc.Namespace(), thirdNewToEvictPod, metav1.DeleteOptions{})
			err = oc.KubeClient().PolicyV1().Evictions(oc.Namespace()).Evict(context.Background(), eviction)
			o.Expect(err).NotTo(o.HaveOccurred())
			shouldTerminatePods.Insert(thirdNewToEvictPod)

			g.By("wait until all evicted pods are either gone or terminating")
			pods, err = exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), podsLabelSelector, func(pod v1.Pod) bool {
				return shouldTerminatePods.Has(pod.Name) && pod.DeletionTimestamp == nil
			}, 0, e2e.PodDeleteTimeout)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("wait for PDB to become stable again")
			expectedStatus = policyv1.PodDisruptionBudgetStatus{
				DisruptionsAllowed: 1,
				CurrentHealthy:     3,
				DesiredHealthy:     2,
				ExpectedPods:       3,
			}
			pdb, err = waitForPDBStatus(oc.KubeClient().PolicyV1(), metav1.ObjectMeta{Name: ifHealthyBudgetPDBName, Namespace: oc.Namespace()}, e2e.PodStartTimeout+podBecomesReadyTimeout, expectedStatus)
			if err != nil {
				g.Fail(fmt.Sprintf("error occurred while waiting for PDB status: %v\nexpected %#v, got %#v", err, expectedStatus, pdbStatus(pdb)))
			}
		})

		g.It(fmt.Sprintf("should evict according to the AlwaysAllow policy"), g.Label("Size:L"), func() {
			g.By(fmt.Sprintf("calling oc create -f %q", alwaysAllowPolicyPDB))
			err := oc.Run("create").Args("-f", alwaysAllowPolicyPDB).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("calling oc create -f %q", nginxWithDelayedReadyDeployment))
			err = oc.Run("create").Args("-f", nginxWithDelayedReadyDeployment).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("wait for a PDB to initialize its status")
			expectedStatus := policyv1.PodDisruptionBudgetStatus{
				DisruptionsAllowed: 0,
				CurrentHealthy:     0,
				DesiredHealthy:     2,
				ExpectedPods:       3,
			}
			pdb, err := waitForPDBStatus(oc.KubeClient().PolicyV1(), metav1.ObjectMeta{Name: alwaysAllowPolicyPDBName, Namespace: oc.Namespace()}, e2e.PodListTimeout, expectedStatus)
			if err != nil {
				g.Fail(fmt.Sprintf("error occurred while waiting for PDB status: %v\nexpected %#v, got %#v", err, expectedStatus, pdbStatus(pdb)))
			}

			g.By("wait for pods to be running")
			pods, err := exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), podsLabelSelector, exutil.CheckPodIsRunning, 3, e2e.PodStartTimeout)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("evict unready pods guarded by a PDB")
			shouldTerminatePods := sets.New[string]()
			for _, pod := range pods {
				eviction := newV1Eviction(oc.Namespace(), pod, metav1.DeleteOptions{})
				err := oc.KubeClient().PolicyV1().Evictions(oc.Namespace()).Evict(context.Background(), eviction)
				o.Expect(err).NotTo(o.HaveOccurred())
				shouldTerminatePods.Insert(pod)
			}

			g.By("wait until all evicted pods are either gone or terminating")
			pods, err = exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), podsLabelSelector, func(pod v1.Pod) bool {
				return shouldTerminatePods.Has(pod.Name) && pod.DeletionTimestamp == nil
			}, 0, e2e.PodDeleteTimeout)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("wait for PDB to become stable")
			expectedStatus = policyv1.PodDisruptionBudgetStatus{
				DisruptionsAllowed: 1,
				CurrentHealthy:     3,
				DesiredHealthy:     2,
				ExpectedPods:       3,
			}
			pdb, err = waitForPDBStatus(oc.KubeClient().PolicyV1(), metav1.ObjectMeta{Name: alwaysAllowPolicyPDBName, Namespace: oc.Namespace()}, e2e.PodStartTimeout+podBecomesReadyTimeout, expectedStatus)
			if err != nil {
				g.Fail(fmt.Sprintf("error occurred while waiting for PDB status: %v\nexpected %#v, got %#v", err, expectedStatus, pdbStatus(pdb)))
			}

			g.By("check pods are ready")
			pods, err = exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), podsLabelSelector, func(pod v1.Pod) bool {
				return exutil.CheckPodIsReady(pod) && pod.DeletionTimestamp == nil
			}, 3, e2e.PodStartTimeout+podBecomesReadyTimeout)

			g.By("only one disruption of ready pods allowed")
			firstToEvictPod := pods[0]
			eviction := newV1Eviction(oc.Namespace(), firstToEvictPod, metav1.DeleteOptions{})
			errOne := oc.KubeClient().PolicyV1().Evictions(oc.Namespace()).Evict(context.Background(), eviction)
			o.Expect(errOne).NotTo(o.HaveOccurred())

			g.By("second eviction should not succeed")
			secondToEvictPod := pods[1]
			eviction = newV1Eviction(oc.Namespace(), secondToEvictPod, metav1.DeleteOptions{})
			errTwo := oc.KubeClient().PolicyV1().Evictions(oc.Namespace()).Evict(context.Background(), eviction)
			o.Expect(errTwo).To(o.HaveOccurred())
		})
	})
})

func newV1Eviction(ns, evictionName string, deleteOption metav1.DeleteOptions) *policyv1.Eviction {
	return &policyv1.Eviction{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "policy/v1",
			Kind:       "Eviction",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      evictionName,
			Namespace: ns,
		},
		DeleteOptions: &deleteOption,
	}
}

func pdbStatus(pdb *policyv1.PodDisruptionBudget) *policyv1.PodDisruptionBudgetStatus {
	if pdb == nil {
		return nil
	}
	return &pdb.Status
}

func waitForPDBStatus(policyClient policyv1clientset.PolicyV1Interface, objMeta metav1.ObjectMeta, timeout time.Duration, expectedStatus policyv1.PodDisruptionBudgetStatus) (*policyv1.PodDisruptionBudget, error) {
	return waitForPDB(policyClient, objMeta, timeout, func(p *policyv1.PodDisruptionBudget) (bool, error) {
		return p.Generation == p.Status.ObservedGeneration &&
				p.Status.DisruptionsAllowed == expectedStatus.DisruptionsAllowed &&
				p.Status.CurrentHealthy == expectedStatus.CurrentHealthy &&
				p.Status.DesiredHealthy == expectedStatus.DesiredHealthy &&
				p.Status.ExpectedPods == expectedStatus.ExpectedPods,
			nil
	})

}

func waitForPDB(policyClient policyv1clientset.PolicyV1Interface, objMeta metav1.ObjectMeta, timeout time.Duration, condition func(pdb *policyv1.PodDisruptionBudget) (bool, error)) (*policyv1.PodDisruptionBudget, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	fieldSelector := fields.OneTermEqualSelector("metadata.name", objMeta.Name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (object runtime.Object, e error) {
			options.FieldSelector = fieldSelector
			return policyClient.PodDisruptionBudgets(objMeta.Namespace).List(context.Background(), options)
		},
		WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
			options.FieldSelector = fieldSelector
			return policyClient.PodDisruptionBudgets(objMeta.Namespace).Watch(context.Background(), options)
		},
	}

	event, err := watchtools.UntilWithSync(ctx, lw, &policyv1.PodDisruptionBudget{}, nil, func(event watch.Event) (bool, error) {
		if event.Type != watch.Modified && (objMeta.ResourceVersion == "" && event.Type != watch.Added) {
			return true, fmt.Errorf("different kind of event appeared while waiting for PodDisruptionBudget modification: event: %#v", event)
		}
		return condition(event.Object.(*policyv1.PodDisruptionBudget))
	})
	if err != nil {
		if event != nil {
			return event.Object.(*policyv1.PodDisruptionBudget), err
		}
		return nil, err
	}
	return event.Object.(*policyv1.PodDisruptionBudget), nil
}

package scheduling

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/pkg/synthetictests/platformidentification"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// requirePodsOnDifferentNodesTest fails if any pods with the prefix in the
// namespace are scheduled to the same node
type requirePodsOnDifferentNodesTest struct {
	namespace  string
	deployment string
}

// getPodDeploymentName returns the name of the deployment associated with a
// given pod. To do that, it needs a list of all replicasets in the pod's
// namespace. If the Pod is not part of a Deployment, an empty string is
// returned
func getPodDeploymentName(pod *corev1.Pod, namespaceReplicaSets *appsv1.ReplicaSetList) string {
	podOwnerReplicaSetName := ""
	for _, owner := range pod.OwnerReferences {
		if owner.APIVersion == "apps/v1" && owner.Kind == "ReplicaSet" {
			podOwnerReplicaSetName = owner.Name
			break
		}
	}

	if podOwnerReplicaSetName == "" {
		// Pod is not owned by a replicaset, so it doesn't have a deployment
		// associated with it
		return ""
	}

	podOwnerReplicaSet := (*appsv1.ReplicaSet)(nil)
	for _, namespaceReplicaSet := range namespaceReplicaSets.Items {
		namespaceReplicaSet := namespaceReplicaSet
		if namespaceReplicaSet.Name == podOwnerReplicaSetName {
			podOwnerReplicaSet = &namespaceReplicaSet
			break
		}
	}

	if podOwnerReplicaSet == nil {
		e2e.Failf("Could not find ReplicaSet resource associated with the Pod ReplicaSet owner reference")
	}

	for _, replicaSetOwner := range podOwnerReplicaSet.OwnerReferences {
		if replicaSetOwner.APIVersion == "apps/v1" && replicaSetOwner.Kind == "Deployment" {
			return replicaSetOwner.Name
		}
	}

	// Pod is part of a ReplicaSet which is not owned by any deployment
	return ""
}

func (p requirePodsOnDifferentNodesTest) getDeploymentPods(ctx context.Context, oc *exutil.CLI) []*corev1.Pod {
	pods, err := oc.KubeFramework().ClientSet.CoreV1().Pods(p.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		e2e.Failf("unable to list pods: %v", err)
	}

	replicaSets, err := oc.KubeFramework().ClientSet.AppsV1().ReplicaSets(p.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		e2e.Failf("unable to list replicaSets: %v", err)
	}

	deploymentPods := make([]*corev1.Pod, 0, 2)
	for _, pod := range pods.Items {
		pod := pod
		podDeploymentName := getPodDeploymentName(&pod, replicaSets)

		if p.deployment == podDeploymentName {
			deploymentPods = append(deploymentPods, &pod)
		}
	}

	return deploymentPods
}

func (p requirePodsOnDifferentNodesTest) run(ctx context.Context, oc *exutil.CLI, jobType *platformidentification.JobType) {
	deploymentPods := p.getDeploymentPods(ctx, oc)

	if len(deploymentPods) == 0 {
		// This is not a bug. Not all deployments are available all the time.
		// For example, the openshift-etcd/etcd-quorum-guard deployment is not
		// created for SingleReplica ControlPlaneTopology.
		return
	}

	nodeNameMap := map[string]string{}
	var err error
	for _, pod := range deploymentPods {
		if podName, ok := nodeNameMap[pod.Spec.NodeName]; ok {
			err = fmt.Errorf("ns/%s pod %s and pod %s are running on the same node: %s", p.namespace, pod.Name, podName, pod.Spec.NodeName)
			if err2 := jobType.MostRecentlyCompletedVersionIsAtLeast("4.11"); err2 != nil {
				result.Flakef("%v, but separation was inconsistent before 4.11, and %v", err, err2)
				return
			}
		} else {
			nodeNameMap[pod.Spec.NodeName] = pod.Name
		}
	}

	if err != nil {
		e2e.Fail(err.Error())
	}
}

var _ = g.Describe("[sig-scheduling][Early]", func() {
	defer g.GinkgoRecover()
	ctx := context.TODO()

	oc := exutil.NewCLI("scheduling-pod-check")

	restConfig, err := exutil.GetClientConfig(exutil.KubeConfigPath())
	if err != nil {
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	jobType, err := platformidentification.GetJobType(ctx, restConfig)
	if err != nil {
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	g.BeforeEach(func() {
		var err error
		_, err = exutil.WaitForRouterServiceIP(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("The HAProxy router pods", func() {
		g.It("should be scheduled on different nodes", func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-ingress", deployment: "router-default"}.run(ctx, oc, jobType)
		})
	})

	g.Describe("The openshift-apiserver pods", func() {
		g.It("should be scheduled on different nodes", func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-apiserver", deployment: "apiserver"}.run(ctx, oc, jobType)
		})
	})

	g.Describe("The openshift-authentication pods", func() {
		g.It("should be scheduled on different nodes", func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-authentication", deployment: "oauth-openshift"}.run(ctx, oc, jobType)
		})
	})

	g.Describe("The openshift-console pods", func() {
		g.It("should be scheduled on different nodes", func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-console", deployment: "console"}.run(ctx, oc, jobType)
		})
	})

	g.Describe("The openshift-console pods", func() {
		g.It("should be scheduled on different nodes", func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-console", deployment: "downloads"}.run(ctx, oc, jobType)
		})
	})

	g.Describe("The openshift-etcd pods", func() {
		g.It("should be scheduled on different nodes", func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-etcd", deployment: "etcd-quorum-guard"}.run(ctx, oc, jobType)
		})
	})

	g.Describe("The openshift-image-registry pods", func() {
		g.It("should be scheduled on different nodes", func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-image-registry", deployment: "image-registry"}.run(ctx, oc, jobType)
		})
	})

	g.Describe("The openshift-monitoring pods", func() {
		g.It("should be scheduled on different nodes", func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-monitoring", deployment: "prometheus-adapter"}.run(ctx, oc, jobType)
		})
	})

	g.Describe("The openshift-monitoring pods", func() {
		g.It("should be scheduled on different nodes", func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-monitoring", deployment: "thanos-querier"}.run(ctx, oc, jobType)
		})
	})

	g.Describe("The openshift-oauth-apiserver pods", func() {
		g.It("should be scheduled on different nodes", func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-oauth-apiserver", deployment: "apiserver"}.run(ctx, oc, jobType)
		})
	})

	g.Describe("The openshift-operator-lifecycle-manager pods", func() {
		g.It("should be scheduled on different nodes", func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-operator-lifecycle-manager", deployment: "packageserver"}.run(ctx, oc, jobType)
		})
	})
})

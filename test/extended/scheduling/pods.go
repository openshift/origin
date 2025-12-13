package scheduling

import (
	"context"
	"encoding/json"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/api/errors"
	e2e "k8s.io/kubernetes/test/e2e/framework"

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

func (p requirePodsOnDifferentNodesTest) getDeploymentPods(oc *exutil.CLI) (*appsv1.Deployment, []*corev1.Pod) {
	pods, err := oc.KubeFramework().ClientSet.CoreV1().Pods(p.namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		e2e.Failf("unable to list pods: %v", err)
	}
	deployment, err := oc.KubeFramework().ClientSet.AppsV1().Deployments(p.namespace).Get(context.Background(), p.deployment, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		e2e.Failf("unable to get deployment: %v", err)
	}

	replicaSets, err := oc.KubeFramework().ClientSet.AppsV1().ReplicaSets(p.namespace).List(context.Background(), metav1.ListOptions{})
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

	return deployment, deploymentPods
}

func (p requirePodsOnDifferentNodesTest) run(oc *exutil.CLI) {
	deployment, deploymentPods := p.getDeploymentPods(oc)

	if len(deploymentPods) == 0 {
		// This is not a bug. Not all deployments are available all the time.
		// For example, the openshift-etcd/etcd-quorum-guard deployment is not
		// created for SingleReplica ControlPlaneTopology.
		return
	}

	nodeNameMap := map[string]string{}
	for _, pod := range deploymentPods {
		if len(pod.Spec.NodeName) == 0 {
			logrus.Warnf("ns/%s pod %s has not been scheduled onto a node yet", p.namespace, pod.Name)
		} else if podName, ok := nodeNameMap[pod.Spec.NodeName]; ok {
			bytes, err := json.Marshal(deployment)
			if err != nil {
				logrus.Warnf("unable to marshal data %v", err)
			}
			e2e.Failf("ns/%s pod %s and pod %s are running on the same node: %s: deployment: %v", p.namespace, pod.Name, podName, pod.Spec.NodeName, string(bytes))
		} else {
			nodeNameMap[pod.Spec.NodeName] = pod.Name
		}
	}
}

var _ = g.Describe("[sig-scheduling][Early]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("scheduling-pod-check")

	g.BeforeEach(func() {
		_, err := exutil.WaitForRouterServiceIP(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("The HAProxy router pods [apigroup:route.openshift.io]", func() {
		g.It("should be scheduled on different nodes", g.Label("Size:S"), func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-ingress", deployment: "router-default"}.run(oc)
		})
	})

	g.Describe("The openshift-apiserver pods [apigroup:authorization.openshift.io][apigroup:build.openshift.io][apigroup:image.openshift.io][apigroup:project.openshift.io][apigroup:quota.openshift.io][apigroup:route.openshift.io][apigroup:security.openshift.io][apigroup:template.openshift.io]", func() {
		g.It("should be scheduled on different nodes", g.Label("Size:S"), func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-apiserver", deployment: "apiserver"}.run(oc)
		})
	})

	g.Describe("The openshift-authentication pods [apigroup:oauth.openshift.io]", func() {
		g.It("should be scheduled on different nodes", g.Label("Size:S"), func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-authentication", deployment: "oauth-openshift"}.run(oc)
		})
	})

	g.Describe("The openshift-console console pods [apigroup:console.openshift.io]", func() {
		g.It("should be scheduled on different nodes", g.Label("Size:S"), func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-console", deployment: "console"}.run(oc)
		})
	})

	g.Describe("The openshift-console downloads pods [apigroup:console.openshift.io]", func() {
		g.It("should be scheduled on different nodes", g.Label("Size:S"), func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-console", deployment: "downloads"}.run(oc)
		})
	})

	g.Describe("The openshift-etcd pods [apigroup:operator.openshift.io]", func() {
		g.It("should be scheduled on different nodes", g.Label("Size:S"), func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-etcd", deployment: "etcd-quorum-guard"}.run(oc)
		})
	})

	g.Describe("The openshift-image-registry pods [apigroup:imageregistry.operator.openshift.io]", func() {
		g.It("should be scheduled on different nodes", g.Label("Size:S"), func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-image-registry", deployment: "image-registry"}.run(oc)
		})
	})

	g.Describe("The openshift-monitoring prometheus-adapter pods [apigroup:monitoring.coreos.com]", func() {
		g.It("should be scheduled on different nodes", g.Label("Size:S"), func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-monitoring", deployment: "prometheus-adapter"}.run(oc)
		})
	})

	g.Describe("The openshift-monitoring thanos-querier pods [apigroup:monitoring.coreos.com]", func() {
		g.It("should be scheduled on different nodes", g.Label("Size:S"), func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-monitoring", deployment: "thanos-querier"}.run(oc)
		})
	})

	g.Describe("The openshift-oauth-apiserver pods [apigroup:oauth.openshift.io][apigroup:user.openshift.io]", func() {
		g.It("should be scheduled on different nodes", g.Label("Size:S"), func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-oauth-apiserver", deployment: "apiserver"}.run(oc)
		})
	})

	g.Describe("The openshift-operator-lifecycle-manager pods [apigroup:packages.operators.coreos.com]", func() {
		g.It("should be scheduled on different nodes", g.Label("Size:S"), func() {
			requirePodsOnDifferentNodesTest{namespace: "openshift-operator-lifecycle-manager", deployment: "packageserver"}.run(oc)
		})
	})
})

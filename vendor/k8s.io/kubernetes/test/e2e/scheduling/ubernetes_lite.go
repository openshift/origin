/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package scheduling

import (
	"fmt"
	"math"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	testutils "k8s.io/kubernetes/test/utils"
	imageutils "k8s.io/kubernetes/test/utils/image"
)

var _ = SIGDescribe("Multi-AZ Clusters", func() {
	f := framework.NewDefaultFramework("multi-az")
	var zoneCount int
	var err error
	image := framework.ServeHostnameImage
	ginkgo.BeforeEach(func() {
		framework.SkipUnlessProviderIs("gce", "gke", "aws")
		if zoneCount <= 0 {
			zoneCount, err = getZoneCount(f.ClientSet)
			framework.ExpectNoError(err)
		}
		ginkgo.By(fmt.Sprintf("Checking for multi-zone cluster.  Zone count = %d", zoneCount))
		msg := fmt.Sprintf("Zone count is %d, only run for multi-zone clusters, skipping test", zoneCount)
		framework.SkipUnlessAtLeast(zoneCount, 2, msg)
		// TODO: SkipUnlessDefaultScheduler() // Non-default schedulers might not spread
		allNodesReady(f.ClientSet, time.Minute)
	})
	ginkgo.It("should spread the pods of a service across zones", func() {
		SpreadServiceOrFail(f, (2*zoneCount)+1, image)
	})

	ginkgo.It("should spread the pods of a replication controller across zones", func() {
		SpreadRCOrFail(f, int32((2*zoneCount)+1), image, []string{"serve-hostname"})
	})
})

// SpreadServiceOrFail check that the pods comprising a service
// get spread evenly across available zones
func SpreadServiceOrFail(f *framework.Framework, replicaCount int, image string) {
	// First create the service
	serviceName := "test-service"
	serviceSpec := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: f.Namespace.Name,
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"service": serviceName,
			},
			Ports: []v1.ServicePort{{
				Port:       80,
				TargetPort: intstr.FromInt(80),
			}},
		},
	}
	_, err := f.ClientSet.CoreV1().Services(f.Namespace.Name).Create(serviceSpec)
	framework.ExpectNoError(err)

	// Now create some pods behind the service
	podSpec := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   serviceName,
			Labels: map[string]string{"service": serviceName},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "test",
					Image: imageutils.GetPauseImageName(),
				},
			},
		},
	}

	// Caution: StartPods requires at least one pod to replicate.
	// Based on the callers, replicas is always positive number: zoneCount >= 0 implies (2*zoneCount)+1 > 0.
	// Thus, no need to test for it. Once the precondition changes to zero number of replicas,
	// test for replicaCount > 0. Otherwise, StartPods panics.
	framework.ExpectNoError(testutils.StartPods(f.ClientSet, replicaCount, f.Namespace.Name, serviceName, *podSpec, false, framework.Logf))

	// Wait for all of them to be scheduled
	selector := labels.SelectorFromSet(labels.Set(map[string]string{"service": serviceName}))
	pods, err := e2epod.WaitForPodsWithLabelScheduled(f.ClientSet, f.Namespace.Name, selector)
	framework.ExpectNoError(err)

	// Now make sure they're spread across zones
	zoneNames, err := framework.GetClusterZones(f.ClientSet)
	framework.ExpectNoError(err)

	checkZoneSpreading(f.ClientSet, pods, zoneNames.List())
}

// Find the name of the zone in which a Node is running
func getZoneNameForNode(node v1.Node) (string, error) {
	for key, value := range node.Labels {
		if key == v1.LabelZoneFailureDomain {
			return value, nil
		}
	}
	return "", fmt.Errorf("Zone name for node %s not found. No label with key %s",
		node.Name, v1.LabelZoneFailureDomain)
}

// Return the number of zones in which we have nodes in this cluster.
func getZoneCount(c clientset.Interface) (int, error) {
	zoneNames, err := framework.GetClusterZones(c)
	if err != nil {
		return -1, err
	}
	return len(zoneNames), nil
}

// Find the name of the zone in which the pod is scheduled
func getZoneNameForPod(c clientset.Interface, pod v1.Pod) (string, error) {
	ginkgo.By(fmt.Sprintf("Getting zone name for pod %s, on node %s", pod.Name, pod.Spec.NodeName))
	node, err := c.CoreV1().Nodes().Get(pod.Spec.NodeName, metav1.GetOptions{})
	framework.ExpectNoError(err)
	return getZoneNameForNode(*node)
}

// Determine whether a set of pods are approximately evenly spread
// across a given set of zones
func checkZoneSpreading(c clientset.Interface, pods *v1.PodList, zoneNames []string) {
	podsPerZone := make(map[string]int)
	// FIXME: Not all clusters are required to allow scheduling across all nodes, nor
	//   is there a guarantee that users can schedule across all nodes. The combination
	//   of those two requires that we only consider zones that we actually scheduled on
	// TODO: the upstream test needs to take as input the default schedulable nodes from
	//   the caller and use that to calculate zones
	// for _, zoneName := range zoneNames {
	// 	podsPerZone[zoneName] = 0
	// }
	for _, pod := range pods.Items {
		if pod.DeletionTimestamp != nil {
			continue
		}
		zoneName, err := getZoneNameForPod(c, pod)
		framework.ExpectNoError(err)
		podsPerZone[zoneName] = podsPerZone[zoneName] + 1
	}
	minPodsPerZone := math.MaxInt32
	maxPodsPerZone := 0
	for _, podCount := range podsPerZone {
		if podCount < minPodsPerZone {
			minPodsPerZone = podCount
		}
		if podCount > maxPodsPerZone {
			maxPodsPerZone = podCount
		}
	}
	gomega.Expect(len(podsPerZone)).To(gomega.BeNumerically(">", 1), "Test requires more than one zone of scheduling: %#v", podsPerZone)
	gomega.Expect(minPodsPerZone).To(gomega.BeNumerically("~", maxPodsPerZone, 1),
		"Pods were not evenly spread across zones.  %d in one zone and %d in another zone",
		minPodsPerZone, maxPodsPerZone)
}

// SpreadRCOrFail Check that the pods comprising a replication
// controller get spread evenly across available zones
func SpreadRCOrFail(f *framework.Framework, replicaCount int32, image string, args []string) {
	name := "ubelite-spread-rc-" + string(uuid.NewUUID())
	ginkgo.By(fmt.Sprintf("Creating replication controller %s", name))
	controller, err := f.ClientSet.CoreV1().ReplicationControllers(f.Namespace.Name).Create(&v1.ReplicationController{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: f.Namespace.Name,
			Name:      name,
		},
		Spec: v1.ReplicationControllerSpec{
			Replicas: &replicaCount,
			Selector: map[string]string{
				"name": name,
			},
			Template: &v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"name": name},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  name,
							Image: image,
							Args:  args,
							Ports: []v1.ContainerPort{{ContainerPort: 9376}},
						},
					},
				},
			},
		},
	})
	framework.ExpectNoError(err)
	// Cleanup the replication controller when we are done.
	defer func() {
		// Resize the replication controller to zero to get rid of pods.
		if err := framework.DeleteRCAndWaitForGC(f.ClientSet, f.Namespace.Name, controller.Name); err != nil {
			framework.Logf("Failed to cleanup replication controller %v: %v.", controller.Name, err)
		}
	}()
	// List the pods, making sure we observe all the replicas.
	selector := labels.SelectorFromSet(labels.Set(map[string]string{"name": name}))
	_, err = e2epod.PodsCreated(f.ClientSet, f.Namespace.Name, name, replicaCount)
	framework.ExpectNoError(err)

	// Wait for all of them to be scheduled
	ginkgo.By(fmt.Sprintf("Waiting for %d replicas of %s to be scheduled.  Selector: %v", replicaCount, name, selector))
	pods, err := e2epod.WaitForPodsWithLabelScheduled(f.ClientSet, f.Namespace.Name, selector)
	framework.ExpectNoError(err)

	// Now make sure they're spread across zones
	zoneNames, err := framework.GetClusterZones(f.ClientSet)
	framework.ExpectNoError(err)
	checkZoneSpreading(f.ClientSet, pods, zoneNames.List())
}

func allNodesReady(c clientset.Interface, timeout time.Duration) error {
	framework.Logf("Waiting up to %v for all (but %d) nodes to be ready", timeout, 0)

	var notReady []*v1.Node
	err := wait.PollImmediate(framework.Poll, timeout, func() (bool, error) {
		notReady = nil
		// It should be OK to list unschedulable Nodes here.
		nodes, err := c.CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			if testutils.IsRetryableAPIError(err) {
				return false, nil
			}
			return false, err
		}
		for i := range nodes.Items {
			node := &nodes.Items[i]
			if !e2enode.IsConditionSetAsExpected(node, v1.NodeReady, true) {
				notReady = append(notReady, node)
			}
		}
		// Framework allows for <TestContext.AllowedNotReadyNodes> nodes to be non-ready,
		// to make it possible e.g. for incorrect deployment of some small percentage
		// of nodes (which we allow in cluster validation). Some nodes that are not
		// provisioned correctly at startup will never become ready (e.g. when something
		// won't install correctly), so we can't expect them to be ready at any point.
		return len(notReady) <= 0, nil
	})

	if err != nil && err != wait.ErrWaitTimeout {
		return err
	}

	if len(notReady) > 0 {
		msg := ""
		for _, node := range notReady {
			msg = fmt.Sprintf("%s, %s", msg, node.Name)
		}
		return fmt.Errorf("Not ready nodes: %#v", msg)
	}
	return nil
}

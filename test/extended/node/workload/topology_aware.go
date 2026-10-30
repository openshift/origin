package workload

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	schedulingv1beta1 "k8s.io/api/scheduling/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

// topologyKey is a label the tests apply themselves rather than relying on a real cluster
// topology. Reusing topology.kubernetes.io/zone would make the outcome depend on how the
// cluster happens to be spread across zones.
const topologyKey = "workload-scheduling.openshift.io/rack"

// TopologyAwareWorkloadScheduling guards spec.schedulingConstraints on a PodGroup: the gang
// must land entirely inside one domain of the given topology key. Upstream has no e2e for
// this gate, only integration tests in
// test/integration/scheduler/podgroup/topology_aware_scheduling/.
var _ = g.Describe("[sig-scheduling][OCPFeatureGate:TopologyAwareWorkloadScheduling][Suite:openshift/workload-scheduling][Serial]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("workload-topology", admissionapi.LevelRestricted)

	var (
		client  kubernetes.Interface
		workers []corev1.Node
	)

	g.BeforeEach(func(ctx context.Context) {
		client = oc.KubeFramework().ClientSet

		var err error
		workers, err = schedulableWorkers(ctx, client)
		framework.ExpectNoError(err, "failed to list schedulable worker nodes")
		if len(workers) < 3 {
			g.Skip(fmt.Sprintf("topology-aware scheduling tests need 3 schedulable workers to build two domains, found %d", len(workers)))
		}
	})

	// newConstrainedPodGroup builds a gang PodGroup constrained to a single topology domain
	// and verifies the apiserver kept the field, which it drops when the gate is off.
	newConstrainedPodGroup := func(ctx context.Context, name string, minCount int32) string {
		pg := newPodGroup(name, oc.Namespace(), "", minCount)
		pg.Spec.SchedulingConstraints = &schedulingv1beta1.PodGroupSchedulingConstraints{
			Topology: []schedulingv1beta1.TopologyConstraint{{Key: topologyKey}},
		}
		createPodGroup(ctx, client, pg)

		stored, err := client.SchedulingV1beta1().PodGroups(oc.Namespace()).Get(ctx, name, metav1.GetOptions{})
		framework.ExpectNoError(err, "failed to read back PodGroup %s", name)
		o.Expect(stored.Spec.SchedulingConstraints).NotTo(o.BeNil(),
			"spec.schedulingConstraints was dropped; the TopologyAwareWorkloadScheduling gate is not enabled on this cluster")
		o.Expect(stored.Spec.SchedulingConstraints.Topology).To(o.HaveLen(1))
		o.Expect(stored.Spec.SchedulingConstraints.Topology[0].Key).To(o.Equal(topologyKey))
		return name
	}

	g.It("should place an entire gang inside a single topology domain", func(ctx context.Context) {
		// rack-a spans two nodes with one slot each, rack-b has a single node with one
		// slot. A gang of two fits only in rack-a. Without the constraint the scheduler
		// could satisfy it with one pod in each rack, so this distinguishes the feature
		// from plain gang scheduling.
		rackB := workers[2].Name
		labelTopologyDomain(ctx, client, workers[0].Name, topologyKey, "rack-a")
		labelTopologyDomain(ctx, client, workers[1].Name, topologyKey, "rack-a")
		labelTopologyDomain(ctx, client, rackB, topologyKey, "rack-b")

		grantGangSlots(ctx, client, workers[0].Name, 1)
		grantGangSlots(ctx, client, workers[1].Name, 1)
		grantGangSlots(ctx, client, rackB, 1)

		pgName := newConstrainedPodGroup(ctx, "pg-single-domain", 2)

		g.By("Creating a gang of two pods")
		names := createPods(ctx, client, oc.Namespace(), []*corev1.Pod{
			newGangPod("tas-1", oc.Namespace(), pgName, ""),
			newGangPod("tas-2", oc.Namespace(), pgName, ""),
		})

		g.By("Verifying the gang schedules")
		for _, name := range names {
			framework.ExpectNoError(
				e2epod.WaitForPodNameRunningInNamespace(ctx, client, name, oc.Namespace()),
				"pod %s should run once the gang is placed", name)
		}

		g.By("Verifying every pod landed in the same topology domain")
		domains := map[string]struct{}{}
		for _, name := range names {
			domains[domainOf(ctx, client, oc.Namespace(), name, topologyKey)] = struct{}{}
		}
		o.Expect(domains).To(o.HaveLen(1), "the gang should occupy exactly one topology domain, got %v", domains)
		for domain := range domains {
			o.Expect(domain).To(o.Equal("rack-a"),
				"rack-a is the only domain with room for the whole gang")
		}
	})

	g.It("should keep a gang pending when no single topology domain can hold it", func(ctx context.Context) {
		// One slot per rack, three racks: there is capacity for the gang in the cluster
		// as a whole but not within any one domain. The gang must not be placed at all --
		// a partial placement would be the interesting bug.
		for i := range workers[:3] {
			labelTopologyDomain(ctx, client, workers[i].Name, topologyKey, fmt.Sprintf("rack-%d", i))
			grantGangSlots(ctx, client, workers[i].Name, 1)
		}

		pgName := newConstrainedPodGroup(ctx, "pg-no-domain", 2)

		g.By("Creating a gang of two pods that no single rack can hold")
		names := createPods(ctx, client, oc.Namespace(), []*corev1.Pod{
			newGangPod("tas-nofit-1", oc.Namespace(), pgName, ""),
			newGangPod("tas-nofit-2", oc.Namespace(), pgName, ""),
		})

		g.By("Verifying the pods stay unschedulable")
		for _, name := range names {
			framework.ExpectNoError(
				e2epod.WaitForPodNameUnschedulableInNamespace(ctx, client, name, oc.Namespace()),
				"pod %s should stay unschedulable when no domain can hold the gang", name)
		}

		g.By("Verifying no pod was placed")
		expectNoneScheduled(ctx, client, oc.Namespace(), names)
	})
})

package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"

	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	clientset "k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2epv "k8s.io/kubernetes/test/e2e/framework/pv"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
	admissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/ptr"
)

const (
	gcpPdCSIDriverName        = "pd.csi.storage.gke.io"
	gcpPdCSITopologyKey       = "topology.gke.io/zone"
	gcpRegionalPDMinSize      = "200Gi" // min size for pd-standard regional-pd
	gcpRegionalTestFileName   = "testfile_regional"
	gcpRegionalTestContent    = "storage test regional-pd"
	gcpRegionalPodTimeout     = 15 * time.Minute
	labelNodeRoleWorker       = "node-role.kubernetes.io/worker"
	labelNodeRoleMaster       = "node-role.kubernetes.io/master"
	labelNodeRoleControlPlane = "node-role.kubernetes.io/control-plane"
)

// gcpFamiliesWithoutPDStandard lists GCP machine families that cannot attach a
// pd-standard Persistent Disk, including the regional pd-standard volume this
// test provisions. C4/C4A/C4D and N4 are Hyperdisk-only; C3/C3D support
// pd-ssd/pd-balanced but not pd-standard. On these families the attach fails with
// "Regional disks is not supported for <type> machine type", so the test is
// skipped rather than run to a deterministic failure. Keep this in sync with
// GCP's machine-family disk-support matrix.
var gcpFamiliesWithoutPDStandard = []string{"c3", "c3d", "c4", "c4a", "c4d", "n4"}

var _ = g.Describe(`[sig-storage][Jira:"Storage"][Driver: pd.csi.storage.gke.io]`, func() {
	defer g.GinkgoRecover()

	var (
		ctx = context.Background()
		oc  = exutil.NewCLIWithPodSecurityLevel("gce-pd-regional", admissionapi.LevelPrivileged)
	)

	g.BeforeEach(func() {
		if !e2e.ProviderIs("gce") {
			g.Skip("skipping, this test requires a GCP cluster")
		}

		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			g.Skip("skipping, this test is only expected to work with standalone clusters")
		}

		isStorageEnabled, err := exutil.IsCapabilityEnabled(oc, configv1.ClusterVersionCapabilityStorage)
		o.Expect(err).NotTo(o.HaveOccurred())
		if !isStorageEnabled {
			g.Skip("skipping, this test requires the Storage capability")
		}

		zones, err := e2enode.GetClusterZones(ctx, oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if zones.Len() < 2 {
			g.Skip("skipping, cluster has fewer than 2 zones")
		}

		// The regional pd-standard volume must attach to the worker that runs the
		// test pod. Some GCP families (e.g. N4) cannot attach pd-standard at all, so
		// skip rather than fail deterministically on such clusters.
		if anyNodeLacksPDStandard(ctx, oc.AdminKubeClient(), labelNodeRoleWorker) {
			g.Skip("skipping, worker nodes use a GCP machine family (e.g. n4) that cannot attach regional Persistent Disk (pd-standard)")
		}
	})

	g.It("regional PD should store data and sync across zones", func() {
		ns := oc.Namespace()
		f := oc.KubeFramework()
		client := oc.AdminKubeClient()
		workers, err := schedulableWorkersPerZone(ctx, client)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(workers).NotTo(o.BeEmpty(), "expected at least one schedulable worker")
		podOneZone := workers[0].Labels[v1.LabelTopologyZone]
		o.Expect(podOneZone).NotTo(o.BeEmpty(), "worker %q missing %s", workers[0].Name, v1.LabelTopologyZone)

		g.By("Creating a regional CSI StorageClass")
		sc := storageframework.GetStorageClass(gcpPdCSIDriverName, map[string]string{
			"type":             "pd-standard",
			"replication-type": "regional-pd",
		}, ptr.To(storagev1.VolumeBindingWaitForFirstConsumer), ns)
		sc.AllowVolumeExpansion = ptr.To(true)
		sc, err = client.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create StorageClass")
		g.DeferCleanup(func() {
			_ = client.StorageV1().StorageClasses().Delete(context.Background(), sc.Name, metav1.DeleteOptions{})
		})

		g.By("Creating a PVC with regional StorageClass")
		pvc := e2epv.MakePersistentVolumeClaim(e2epv.PersistentVolumeClaimConfig{
			ClaimSize:        gcpRegionalPDMinSize,
			StorageClassName: &sc.Name,
			AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
		}, ns)
		pvc, err = e2epv.CreatePVC(ctx, client, ns, pvc)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create PVC")
		g.DeferCleanup(func() {
			_ = e2epv.DeletePersistentVolumeClaim(context.Background(), client, pvc.Name, ns)
		})

		g.By("Creating pod one and waiting until it is Running")
		podOne, err := createGCPRegionalVolumePod(ctx, client, ns, pvc, map[string]string{v1.LabelTopologyZone: podOneZone}, false)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create pod one")
		g.DeferCleanup(func() {
			_ = e2epod.DeletePodWithWait(context.Background(), client, podOne)
		})

		g.By("Writing and reading data on pod one")
		writePath := fmt.Sprintf("%s/%s", e2epod.VolumeMountPath1, gcpRegionalTestFileName)
		_, _, err = e2epod.ExecShellInPodWithFullOutput(ctx, f, podOne.Name, fmt.Sprintf("echo -n %q > %s", gcpRegionalTestContent, writePath))
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to write test file on pod one")
		stdout, _, err := e2epod.ExecShellInPodWithFullOutput(ctx, f, podOne.Name, "cat "+writePath)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to read test file on pod one")
		o.Expect(strings.TrimSpace(stdout)).To(o.Equal(gcpRegionalTestContent))

		g.By("Verifying the regional PV has nodeAffinity for 2 zones")
		pvc, err = client.CoreV1().PersistentVolumeClaims(ns).Get(ctx, pvc.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pvc.Spec.VolumeName).NotTo(o.BeEmpty(), "PVC should be bound")
		pv, err := client.CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		volZones := pvTopologyZones(pv, gcpPdCSITopologyKey)
		o.Expect(volZones).To(o.HaveLen(2), "regional PD should list 2 zones in PV nodeAffinity, got %v", volZones)

		// Cross-zone attach may land on a control-plane node; skip when that
		// node type cannot attach pd-standard.
		if len(workers) < 2 {
			e2e.Logf("Skipping cross-zone sync verification: need workers in >=2 zones, got %d", len(workers))
			return
		}
		if anyNodeLacksPDStandard(ctx, client, labelNodeRoleControlPlane, labelNodeRoleMaster) {
			e2e.Logf("Skipping cross-zone sync verification: control-plane instance type may not attach pd-standard")
			return
		}

		g.By("Deleting pod one so the RWO volume can attach elsewhere")
		err = e2epod.DeletePodWithWait(ctx, client, podOne)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to delete pod one")

		otherZone := volZones[0]
		if otherZone == podOneZone {
			otherZone = volZones[1]
		}

		g.By(fmt.Sprintf("Creating pod two in zone %q and verifying data written by pod one", otherZone))
		podTwo, err := createGCPRegionalVolumePod(ctx, client, ns, pvc, map[string]string{v1.LabelTopologyZone: otherZone}, true)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create pod two")
		g.DeferCleanup(func() {
			_ = e2epod.DeletePodWithWait(context.Background(), client, podTwo)
		})

		stdout, _, err = e2epod.ExecShellInPodWithFullOutput(ctx, f, podTwo.Name, "cat "+writePath)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to read test file on pod two")
		o.Expect(strings.TrimSpace(stdout)).To(o.Equal(gcpRegionalTestContent),
			"pod two in a different zone should read data written by pod one")
	})
})

// createGCPRegionalVolumePod creates a pod that mounts pvc, optionally tolerating control-plane taints.
func createGCPRegionalVolumePod(ctx context.Context, client clientset.Interface, ns string, pvc *v1.PersistentVolumeClaim, nodeSelector map[string]string, tolerateControlPlane bool) (*v1.Pod, error) {
	pod, err := e2epod.MakeSecPod(&e2epod.Config{
		NS:            ns,
		PVCs:          []*v1.PersistentVolumeClaim{pvc},
		SecurityLevel: admissionapi.LevelPrivileged,
		NodeSelection: e2epod.NodeSelection{Selector: nodeSelector},
	})
	if err != nil {
		return nil, fmt.Errorf("unable to create pod: %w", err)
	}
	if tolerateControlPlane {
		pod.Spec.Tolerations = append(pod.Spec.Tolerations,
			v1.Toleration{Key: labelNodeRoleMaster, Operator: v1.TolerationOpExists, Effect: v1.TaintEffectNoSchedule},
			v1.Toleration{Key: labelNodeRoleControlPlane, Operator: v1.TolerationOpExists, Effect: v1.TaintEffectNoSchedule},
		)
	}

	pod, err = client.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("pod Create API error: %w", err)
	}
	if err := e2epod.WaitTimeoutForPodRunningInNamespace(ctx, client, pod.Name, ns, gcpRegionalPodTimeout); err != nil {
		return pod, fmt.Errorf("pod %q is not Running: %w", pod.Name, err)
	}
	pod, err = client.CoreV1().Pods(ns).Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		return pod, fmt.Errorf("pod Get API error: %w", err)
	}
	return pod, nil
}

// schedulableWorkersPerZone returns one ready, schedulable worker from each zone.
func schedulableWorkersPerZone(ctx context.Context, client clientset.Interface) ([]v1.Node, error) {
	nodes, err := e2enode.GetReadySchedulableNodes(ctx, client)
	if err != nil {
		return nil, err
	}
	seen := sets.New[string]()
	var workers []v1.Node
	for _, node := range nodes.Items {
		if _, ok := node.Labels[labelNodeRoleWorker]; !ok {
			continue
		}
		zone := node.Labels[v1.LabelTopologyZone]
		if zone == "" || seen.Has(zone) {
			continue
		}
		seen.Insert(zone)
		workers = append(workers, node)
	}
	return workers, nil
}

// anyNodeLacksPDStandard reports whether any node matching one of the given role
// label selectors uses a GCP machine family that cannot attach a pd-standard
// Persistent Disk (see gcpFamiliesWithoutPDStandard). A List error for a selector
// is treated as "not found" so the caller falls back to running the test.
func anyNodeLacksPDStandard(ctx context.Context, client clientset.Interface, roleSelectors ...string) bool {
	for _, selector := range roleSelectors {
		nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			continue
		}
		for i := range nodes.Items {
			if familyLacksPDStandard(nodeInstanceType(&nodes.Items[i])) {
				return true
			}
		}
	}
	return false
}

// nodeInstanceType returns the node's GCP machine type from its well-known labels,
// preferring the stable label and falling back to the beta label.
func nodeInstanceType(node *v1.Node) string {
	if t := node.Labels[v1.LabelInstanceTypeStable]; t != "" {
		return t
	}
	return node.Labels[v1.LabelInstanceType]
}

// familyLacksPDStandard reports whether instanceType belongs to a GCP machine
// family that cannot attach a pd-standard Persistent Disk.
func familyLacksPDStandard(instanceType string) bool {
	for _, family := range gcpFamiliesWithoutPDStandard {
		if instanceType == family || strings.HasPrefix(instanceType, family+"-") {
			return true
		}
	}
	return false
}

func pvTopologyZones(pv *v1.PersistentVolume, topologyKey string) []string {
	zones := sets.New[string]()
	if pv.Spec.NodeAffinity == nil || pv.Spec.NodeAffinity.Required == nil {
		return nil
	}
	for _, term := range pv.Spec.NodeAffinity.Required.NodeSelectorTerms {
		for _, expr := range term.MatchExpressions {
			if expr.Key == topologyKey {
				zones.Insert(expr.Values...)
			}
		}
	}
	return sets.List(zones)
}

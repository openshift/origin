package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"
	configv1 "github.com/openshift/api/config/v1"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	"github.com/openshift/origin/test/extended/imagepolicy"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	nodeutils "github.com/openshift/origin/test/extended/node"
	exutil "github.com/openshift/origin/test/extended/util"
)

// mirrorTestPool isolates mirror-set testing to a single-node custom MCP so rollouts
// do not wait on the full worker and master pools.
type mirrorTestPool struct {
	oc        *exutil.CLI
	mcClient  *machineconfigclient.Clientset
	PoolName  string
	NodeName  string
	nodeLabel string
}

func newMirrorTestPool(oc *exutil.CLI, ctx context.Context) *mirrorTestPool {
	poolName := fmt.Sprintf("mirror-test-%s", utilrand.String(5))
	nodeLabel := fmt.Sprintf("node-role.kubernetes.io/%s", poolName)

	mcClient, err := machineconfigclient.NewForConfig(oc.AdminConfig())
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to create MachineConfig client")

	nodeName := nodeutils.GetFirstReadyWorkerNode(oc)
	o.Expect(nodeName).NotTo(o.BeEmpty(), "no ready worker node found for custom MCP")
	err = nodeutils.EnsureNodeHasNoCustomRole(ctx, oc, nodeName)
	o.Expect(err).NotTo(o.HaveOccurred())

	testMCP := &mcfgv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: poolName,
			Labels: map[string]string{
				"machineconfiguration.openshift.io/pool": poolName,
			},
		},
		Spec: mcfgv1.MachineConfigPoolSpec{
			MachineConfigSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "machineconfiguration.openshift.io/role",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"worker", poolName},
					},
				},
			},
			NodeSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					nodeLabel: "",
				},
			},
		},
	}
	_, err = mcClient.MachineconfigurationV1().MachineConfigPools().Create(ctx, testMCP, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to create custom MachineConfigPool %s", poolName)

	// Build pool early so Teardown can clean up if labeling or readiness fails.
	pool := &mirrorTestPool{
		oc:        oc,
		mcClient:  mcClient,
		PoolName:  poolName,
		NodeName:  nodeName,
		nodeLabel: nodeLabel,
	}

	patchData := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:""}}}`, nodeLabel))
	_, err = oc.AdminKubeClient().CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, patchData, metav1.PatchOptions{})
	if err != nil {
		// MCP was created but labeling failed — clean up the MCP before failing.
		pool.Teardown()
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to label node %s for custom MCP", nodeName)
	}

	err = waitForMirrorTestMCPReady(ctx, mcClient, poolName, 10*time.Minute)
	if err != nil {
		pool.Teardown()
		o.Expect(err).NotTo(o.HaveOccurred(), "custom MachineConfigPool %s did not become ready", poolName)
	}
	e2e.Logf("Custom mirror test pool %s ready on node %s", poolName, nodeName)

	return pool
}

func (p *mirrorTestPool) currentSpec() string {
	return imagepolicy.GetMCPCurrentSpecConfigName(p.oc, p.PoolName)
}

func (p *mirrorTestPool) waitForRollout(initialSpec string) {
	imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(p.oc, p.PoolName, initialSpec)
}

// mirrorPoolState holds MCP spec.configuration.name before a cluster-scoped mirror change.
type mirrorPoolState struct {
	pool   string
	worker string
	master string
}

// captureClusterState snapshots custom pool, worker, and master MCP config names before a mutation.
func (p *mirrorTestPool) captureClusterState(ctx context.Context) mirrorPoolState {
	worker, err := p.mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, "worker", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get worker MCP")
	master, err := p.mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, "master", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get master MCP")
	return mirrorPoolState{
		pool:   p.currentSpec(),
		worker: worker.Spec.Configuration.Name,
		master: master.Spec.Configuration.Name,
	}
}

// waitForMirrorRollout waits until custom pool, worker, and master MCPs advance
// past before (config name change, spec==status, Updated=True), not just Updated polling.
func (p *mirrorTestPool) waitForMirrorRollout(before mirrorPoolState) {
	p.waitForRollout(before.pool)
	e2e.Logf("Waiting for worker and master MCPs to roll out mirror resource change")
	imagepolicy.WaitForMCPsConfigSpecChangeAndUpdated(p.oc, before.worker, before.master)
}

func (p *mirrorTestPool) readRegistriesConf(ctx context.Context) string {
	registriesConf, err := nodeutils.ExecOnNodeWithChroot(ctx, p.oc, p.NodeName, "cat", "/etc/containers/registries.conf")
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to read registries.conf from node %s", p.NodeName)
	return registriesConf
}

// Teardown restores the node to the worker pool and deletes the custom MCP.
// Runs every cleanup step; accumulated errors fail the test via Gomega.
func (p *mirrorTestPool) Teardown() {
	ctx := context.Background()
	var errs []string

	e2e.Logf("Teardown: removing label %s from node %s", p.nodeLabel, p.NodeName)
	removePatch := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:null}}}`, p.nodeLabel))
	_, patchErr := p.oc.AdminKubeClient().CoreV1().Nodes().Patch(ctx, p.NodeName, types.MergePatchType, removePatch, metav1.PatchOptions{})
	if patchErr != nil && !apierrors.IsNotFound(patchErr) {
		errs = append(errs, fmt.Sprintf("failed to remove label %s from node %s: %v", p.nodeLabel, p.NodeName, patchErr))
	}

	e2e.Logf("Teardown: waiting for node %s to transition back to worker pool", p.NodeName)
	waitErr := wait.PollUntilContextTimeout(ctx, 15*time.Second, 15*time.Minute, true, func(ctx context.Context) (bool, error) {
		currentNode, getErr := p.oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, p.NodeName, metav1.GetOptions{})
		if getErr != nil {
			return false, nil
		}
		currentConfig := currentNode.Annotations["machineconfiguration.openshift.io/currentConfig"]
		desiredConfig := currentNode.Annotations["machineconfiguration.openshift.io/desiredConfig"]
		return currentConfig != "" && !strings.Contains(currentConfig, p.PoolName) && currentConfig == desiredConfig, nil
	})
	if waitErr != nil {
		errs = append(errs, fmt.Sprintf("node %s did not transition back to worker pool: %v", p.NodeName, waitErr))
	}

	deleteErr := p.mcClient.MachineconfigurationV1().MachineConfigPools().Delete(ctx, p.PoolName, metav1.DeleteOptions{})
	if deleteErr != nil && !apierrors.IsNotFound(deleteErr) {
		errs = append(errs, fmt.Sprintf("failed to delete MachineConfigPool %s: %v", p.PoolName, deleteErr))
	}

	if deleteErr == nil || apierrors.IsNotFound(deleteErr) {
		e2e.Logf("Teardown: waiting for worker and master MCPs to stabilize")
		if stabilizeErr := nodeutils.WaitForMCP(ctx, p.mcClient, "worker", 10*time.Minute); stabilizeErr != nil {
			errs = append(errs, fmt.Sprintf("worker MCP did not stabilize during teardown: %v", stabilizeErr))
		}
		if stabilizeErr := nodeutils.WaitForMCP(ctx, p.mcClient, "master", 10*time.Minute); stabilizeErr != nil {
			errs = append(errs, fmt.Sprintf("master MCP did not stabilize during teardown: %v", stabilizeErr))
		}
	} else {
		errs = append(errs, fmt.Sprintf(
			"skipped worker/master MCP stabilization wait because MachineConfigPool %s deletion did not complete", p.PoolName))
	}

	o.Expect(errs).To(o.BeEmpty(), "mirror test pool %s teardown encountered failure(s):\n%s",
		p.PoolName, strings.Join(errs, "\n"))
}

// waitForMirrorTestMCPReady waits for a custom MCP to finish its initial bootstrap.
func waitForMirrorTestMCPReady(ctx context.Context, mcClient *machineconfigclient.Clientset, poolName string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		mcp, err := mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, poolName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		updating := false
		degraded := false
		updated := false
		for _, condition := range mcp.Status.Conditions {
			switch condition.Type {
			case "Updating":
				if condition.Status == corev1.ConditionTrue {
					updating = true
				}
			case "Degraded":
				if condition.Status == corev1.ConditionTrue {
					degraded = true
				}
			case "Updated":
				if condition.Status == corev1.ConditionTrue {
					updated = true
				}
			}
		}

		if degraded {
			return false, fmt.Errorf("MachineConfigPool %s is degraded", poolName)
		}

		isReady := !updating && updated &&
			mcp.Status.MachineCount > 0 &&
			mcp.Status.ReadyMachineCount == mcp.Status.MachineCount &&
			mcp.Spec.Configuration.Name == mcp.Status.Configuration.Name

		return isReady, nil
	})
}

// pollMCPSpecUnchanged polls the given MCP spec name every 15 seconds for the duration.
// It returns an error if the spec deviates from the baseline, or nil if it stayed stable.
func pollMCPSpecUnchanged(oc *exutil.CLI, pool, baselineSpec string, duration time.Duration) error {
	err := wait.PollImmediate(15*time.Second, duration, func() (bool, error) {
		if current := imagepolicy.GetMCPCurrentSpecConfigName(oc, pool); current != baselineSpec {
			return false, fmt.Errorf("MCP %s spec changed from %s to %s", pool, baselineSpec, current)
		}
		return false, nil
	})
	if err == wait.ErrWaitTimeout {
		return nil
	}
	return err
}

// author: asahay@redhat.com
var _ = g.Describe("[sig-node][Suite:openshift/disruptive-longrunning][Disruptive][Serial] ImageTagMirrorSet and ImageDigestMirrorSet", func() {
	var (
		oc = exutil.NewCLIWithoutNamespace("image-mirror-set")
	)

	g.BeforeEach(func(ctx context.Context) {
		nodeutils.SkipOnMicroShift(oc)
		nodeutils.EnsureNodesReady(ctx, oc)
	})

	g.It("[OTP] Create ImageDigestMirrorSet and ImageTagMirrorSet and verify registries.conf [OCP-57401]", func(ctx context.Context) {
		configClient := oc.AdminConfigClient().ConfigV1()
		suffix := utilrand.String(5)
		idmsName := fmt.Sprintf("digest-mirror-%s", suffix)
		itmsName := fmt.Sprintf("tag-mirror-%s", suffix)

		pool := newMirrorTestPool(oc, ctx)
		g.DeferCleanup(pool.Teardown)

		g.By("Step 1: Create an ImageDigestMirrorSet")
		idms := &configv1.ImageDigestMirrorSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: idmsName,
			},
			Spec: configv1.ImageDigestMirrorSetSpec{
				ImageDigestMirrors: []configv1.ImageDigestMirrors{
					{
						Source: "registry.redhat.io/openshift4",
						Mirrors: []configv1.ImageMirror{
							"mirror.example.com/redhat",
						},
						MirrorSourcePolicy: configv1.AllowContactingSource,
					},
					{
						Source: "registry.redhat.io/rhel8",
						Mirrors: []configv1.ImageMirror{
							"mirror.example.com/rhel8",
						},
						MirrorSourcePolicy: configv1.NeverContactSource,
					},
				},
			},
		}

		before := pool.captureClusterState(ctx)

		createdIDMS, err := configClient.ImageDigestMirrorSets().Create(ctx, idms, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create ImageDigestMirrorSet")
		e2e.Logf("ImageDigestMirrorSet %q created successfully", createdIDMS.Name)

		g.DeferCleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cancel()
			g.By("Cleanup: Delete IDMS and ITMS resources")
			cleanupBefore := pool.captureClusterState(cleanupCtx)
			if delErr := configClient.ImageTagMirrorSets().Delete(cleanupCtx, itmsName, metav1.DeleteOptions{}); delErr != nil {
				e2e.Logf("Warning: failed to delete ImageTagMirrorSet: %v", delErr)
			}
			if delErr := configClient.ImageDigestMirrorSets().Delete(cleanupCtx, idmsName, metav1.DeleteOptions{}); delErr != nil {
				e2e.Logf("Warning: failed to delete ImageDigestMirrorSet: %v", delErr)
			}
			pool.waitForMirrorRollout(cleanupBefore)
		})

		pool.waitForMirrorRollout(before)
		e2e.Logf("IDMS MCP rollout complete on custom pool %s", pool.PoolName)

		g.By("Step 2: Create an ImageTagMirrorSet")
		itms := &configv1.ImageTagMirrorSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: itmsName,
			},
			Spec: configv1.ImageTagMirrorSetSpec{
				ImageTagMirrors: []configv1.ImageTagMirrors{
					{
						Source: "registry.access.redhat.com/ubi8/ubi-minimal",
						Mirrors: []configv1.ImageMirror{
							"example.io/example/ubi-minimal",
							"example.com/example/ubi-minimal",
						},
						MirrorSourcePolicy: configv1.AllowContactingSource,
					},
					{
						Source: "registry.access.redhat.com/ubi8/ubi-minimal-1",
						Mirrors: []configv1.ImageMirror{
							"example.io/example/ubi-minimal",
						},
						MirrorSourcePolicy: configv1.NeverContactSource,
					},
				},
			},
		}

		itmsBefore := pool.captureClusterState(ctx)

		createdITMS, err := configClient.ImageTagMirrorSets().Create(ctx, itms, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create ImageTagMirrorSet")
		e2e.Logf("ImageTagMirrorSet %q created successfully", createdITMS.Name)

		g.By("Step 3: Wait for custom MCP and worker/master MCPs to finish rolling out")
		pool.waitForMirrorRollout(itmsBefore)
		e2e.Logf("Custom MCP %s finished rolling out after ITMS creation", pool.PoolName)

		g.By("Step 4: Verify /etc/containers/registries.conf on the custom pool node")
		registriesConf := pool.readRegistriesConf(ctx)
		e2e.Logf("registries.conf on %s: read %d bytes, asserting expected entries", pool.NodeName, len(registriesConf))

		g.By("Verify IDMS entries (digest-only mirrors)")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "registry.redhat.io/openshift4"`),
			"registries.conf should contain the IDMS source for openshift4")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "mirror.example.com/redhat"`),
			"registries.conf should contain the IDMS mirror for openshift4")
		o.Expect(registriesConf).To(o.ContainSubstring(`pull-from-mirror = "digest-only"`),
			"registries.conf should have pull-from-mirror set to digest-only for IDMS mirrors")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "registry.redhat.io/rhel8"`),
			"registries.conf should contain the IDMS source for rhel8")

		g.By("Verify ITMS entries (tag-only mirrors)")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "registry.access.redhat.com/ubi8/ubi-minimal"`),
			"registries.conf should contain the ITMS source for ubi-minimal")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "example.io/example/ubi-minimal"`),
			"registries.conf should contain the ITMS mirror location")
		o.Expect(registriesConf).To(o.ContainSubstring(`pull-from-mirror = "tag-only"`),
			"registries.conf should have pull-from-mirror set to tag-only for ITMS mirrors")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "registry.access.redhat.com/ubi8/ubi-minimal-1"`),
			"registries.conf should contain the ITMS source for ubi-minimal-1")

		g.By("Verify NeverContactSource entries are blocked")
		o.Expect(registriesConf).To(o.ContainSubstring("location = \"registry.access.redhat.com/ubi8/ubi-minimal-1\"\n  blocked = true"),
			"registry.access.redhat.com/ubi8/ubi-minimal-1 should be blocked (NeverContactSource)")
		o.Expect(registriesConf).To(o.ContainSubstring("location = \"registry.redhat.io/rhel8\"\n  blocked = true"),
			"registry.redhat.io/rhel8 should be blocked (NeverContactSource)")
	})

	// author: asahay@redhat.com
	g.It("[OTP] ICSP and IDMS/ITMS can coexist in cluster [OCP-70203]", ote.Informing(), func(ctx context.Context) {
		configClient := oc.AdminConfigClient().ConfigV1()
		operatorClient := oc.AdminOperatorClient().OperatorV1alpha1()
		suffix := utilrand.String(5)
		icspName1 := fmt.Sprintf("ubi8repo-%s", suffix)
		idmsName := fmt.Sprintf("digest-mirror-%s", suffix)
		itmsName := fmt.Sprintf("tag-mirror-%s", suffix)
		icspName2 := fmt.Sprintf("ubi9repo-%s", suffix)

		pool := newMirrorTestPool(oc, ctx)
		g.DeferCleanup(pool.Teardown)

		g.By("Step 1: Create ICSP with digest mirrors for ubi8/ubi-minimal and openshift5")
		icsp1 := &operatorv1alpha1.ImageContentSourcePolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:   icspName1,
				Labels: map[string]string{"e2e-test": "ocp-70203"},
			},
			Spec: operatorv1alpha1.ImageContentSourcePolicySpec{
				RepositoryDigestMirrors: []operatorv1alpha1.RepositoryDigestMirrors{
					{
						Source: "registry.access.redhat.com/ubi8/ubi-minimal",
						Mirrors: []string{
							"example.io/example/ubi-minimal",
							"example.com/example/ubi-minimal",
						},
					},
					{
						Source: "registry.redhat.io/openshift5",
						Mirrors: []string{
							"mirror.example.com/redhat",
						},
					},
				},
			},
		}

		before := pool.captureClusterState(ctx)

		_, err := operatorClient.ImageContentSourcePolicies().Create(ctx, icsp1, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create ICSP %s", icspName1)
		e2e.Logf("ICSP %s created successfully", icspName1)

		g.DeferCleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cancel()
			g.By("Cleanup: Delete any remaining test resources and wait for custom MCP to settle")
			cleanupBefore := pool.captureClusterState(cleanupCtx)
			toDelete := false

			for _, name := range []string{icspName1, icspName2} {
				if _, getErr := operatorClient.ImageContentSourcePolicies().Get(cleanupCtx, name, metav1.GetOptions{}); getErr == nil {
					if delErr := operatorClient.ImageContentSourcePolicies().Delete(cleanupCtx, name, metav1.DeleteOptions{}); delErr == nil {
						e2e.Logf("Cleanup: deleted ICSP %s", name)
						toDelete = true
					} else {
						e2e.Logf("Cleanup: warning - failed to delete ICSP %s: %v", name, delErr)
					}
				}
			}
			if _, getErr := configClient.ImageTagMirrorSets().Get(cleanupCtx, itmsName, metav1.GetOptions{}); getErr == nil {
				if delErr := configClient.ImageTagMirrorSets().Delete(cleanupCtx, itmsName, metav1.DeleteOptions{}); delErr == nil {
					e2e.Logf("Cleanup: deleted ITMS %s", itmsName)
					toDelete = true
				} else {
					e2e.Logf("Cleanup: warning - failed to delete ITMS %s: %v", itmsName, delErr)
				}
			}
			if _, getErr := configClient.ImageDigestMirrorSets().Get(cleanupCtx, idmsName, metav1.GetOptions{}); getErr == nil {
				if delErr := configClient.ImageDigestMirrorSets().Delete(cleanupCtx, idmsName, metav1.DeleteOptions{}); delErr == nil {
					e2e.Logf("Cleanup: deleted IDMS %s", idmsName)
					toDelete = true
				} else {
					e2e.Logf("Cleanup: warning - failed to delete IDMS %s: %v", idmsName, delErr)
				}
			}
			if toDelete {
				pool.waitForMirrorRollout(cleanupBefore)
			}
		})

		g.By("Step 2: Wait for custom MCP and worker/master MCPs rollout after ICSP creation and verify registries.conf")
		pool.waitForMirrorRollout(before)
		e2e.Logf("Custom MCP %s rollout complete after ICSP creation", pool.PoolName)

		registriesConf := pool.readRegistriesConf(ctx)
		e2e.Logf("registries.conf after ICSP creation: read %d bytes, asserting expected entries", len(registriesConf))

		o.Expect(registriesConf).To(o.ContainSubstring(`location = "registry.access.redhat.com/ubi8/ubi-minimal"`),
			"registries.conf should contain ICSP source for ubi8/ubi-minimal")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "example.io/example/ubi-minimal"`),
			"registries.conf should contain ICSP mirror example.io/example/ubi-minimal")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "example.com/example/ubi-minimal"`),
			"registries.conf should contain ICSP mirror example.com/example/ubi-minimal")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "registry.redhat.io/openshift5"`),
			"registries.conf should contain ICSP source for openshift5")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "mirror.example.com/redhat"`),
			"registries.conf should contain ICSP mirror mirror.example.com/redhat")
		o.Expect(registriesConf).To(o.ContainSubstring(`pull-from-mirror = "digest-only"`),
			"registries.conf should have pull-from-mirror = digest-only for ICSP entries")

		g.By("Step 3: Create IDMS with same registry/mirror config as ICSP (AllowContactingSource)")
		specBeforeIDMS := pool.currentSpec()
		idms := &configv1.ImageDigestMirrorSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:   idmsName,
				Labels: map[string]string{"e2e-test": "ocp-70203"},
			},
			Spec: configv1.ImageDigestMirrorSetSpec{
				ImageDigestMirrors: []configv1.ImageDigestMirrors{
					{
						Source: "registry.access.redhat.com/ubi8/ubi-minimal",
						Mirrors: []configv1.ImageMirror{
							"example.io/example/ubi-minimal",
							"example.com/example/ubi-minimal",
						},
						MirrorSourcePolicy: configv1.AllowContactingSource,
					},
					{
						Source: "registry.redhat.io/openshift5",
						Mirrors: []configv1.ImageMirror{
							"mirror.example.com/redhat",
						},
						MirrorSourcePolicy: configv1.AllowContactingSource,
					},
				},
			},
		}
		_, err = configClient.ImageDigestMirrorSets().Create(ctx, idms, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create IDMS %s", idmsName)
		e2e.Logf("IDMS %s created successfully", idmsName)

		g.By("Step 3 (verify): Confirm no new MC was generated after IDMS creation with same config as ICSP")
		o.Expect(pollMCPSpecUnchanged(oc, pool.PoolName, specBeforeIDMS, 2*time.Minute)).
			NotTo(o.HaveOccurred(), "unexpected MCP rollout after IDMS creation with same config as ICSP")
		e2e.Logf("Confirmed: custom MCP %s stable for 2 minutes after IDMS creation", pool.PoolName)

		g.By("Step 4.1: Delete ICSP - IDMS covers the same config so no new MC should be triggered")
		specBeforeICSPDelete := pool.currentSpec()
		err = operatorClient.ImageContentSourcePolicies().Delete(ctx, icspName1, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to delete ICSP %s", icspName1)
		e2e.Logf("ICSP %s deleted successfully", icspName1)

		g.By("Step 4.2: Confirm no new MC was generated after ICSP deletion (IDMS still covers the same config)")
		o.Expect(pollMCPSpecUnchanged(oc, pool.PoolName, specBeforeICSPDelete, 2*time.Minute)).
			NotTo(o.HaveOccurred(), "unexpected MCP rollout after ICSP deletion when IDMS covers the same config")
		e2e.Logf("Confirmed: custom MCP %s stable for 2 minutes after ICSP deletion", pool.PoolName)

		g.By("Step 5: Verify registries.conf is unchanged after ICSP deletion (IDMS maintains same mirror config)")
		registriesConfAfterICSPDelete := pool.readRegistriesConf(ctx)
		o.Expect(registriesConfAfterICSPDelete).To(o.Equal(registriesConf),
			"registries.conf should be unchanged after ICSP deletion when IDMS covers the same mirror config")
		e2e.Logf("Confirmed: registries.conf unchanged after ICSP deletion")

		g.By("Step 6: Create ITMS with tag mirrors for ubi9/ubi-minimal (different source from IDMS)")
		itmsBefore := pool.captureClusterState(ctx)
		itms := &configv1.ImageTagMirrorSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:   itmsName,
				Labels: map[string]string{"e2e-test": "ocp-70203"},
			},
			Spec: configv1.ImageTagMirrorSetSpec{
				ImageTagMirrors: []configv1.ImageTagMirrors{
					{
						Source: "registry.access.redhat.com/ubi9/ubi-minimal",
						Mirrors: []configv1.ImageMirror{
							"example.io/example/ubi-minimal-1",
							"example.com/example/ubi-minimal-1",
						},
						MirrorSourcePolicy: configv1.AllowContactingSource,
					},
				},
			},
		}
		_, err = configClient.ImageTagMirrorSets().Create(ctx, itms, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create ITMS %s", itmsName)
		e2e.Logf("ITMS %s created successfully", itmsName)

		pool.waitForMirrorRollout(itmsBefore)
		e2e.Logf("Custom MCP %s rollout complete after ITMS creation", pool.PoolName)

		g.By("Step 7: Verify registries.conf updated with ITMS tag-only entries alongside IDMS digest entries")
		registriesConfAfterITMS := pool.readRegistriesConf(ctx)
		e2e.Logf("registries.conf after ITMS creation: read %d bytes, asserting expected entries", len(registriesConfAfterITMS))

		o.Expect(registriesConfAfterITMS).To(o.ContainSubstring(`location = "registry.access.redhat.com/ubi9/ubi-minimal"`),
			"registries.conf should contain the ITMS source")
		o.Expect(registriesConfAfterITMS).To(o.ContainSubstring(`location = "example.io/example/ubi-minimal-1"`),
			"registries.conf should contain ITMS mirror example.io/example/ubi-minimal-1")
		o.Expect(registriesConfAfterITMS).To(o.ContainSubstring(`location = "example.com/example/ubi-minimal-1"`),
			"registries.conf should contain ITMS mirror example.com/example/ubi-minimal-1")
		o.Expect(registriesConfAfterITMS).To(o.ContainSubstring(`pull-from-mirror = "tag-only"`),
			"registries.conf should have pull-from-mirror = tag-only for ITMS entries")
		o.Expect(registriesConfAfterITMS).To(o.ContainSubstring(`location = "registry.access.redhat.com/ubi8/ubi-minimal"`),
			"registries.conf should still contain IDMS entries for ubi8/ubi-minimal")

		g.By("Step 8: Create second ICSP with digest mirrors for registry.example.com/example/myimage")
		icsp2Before := pool.captureClusterState(ctx)
		icsp2 := &operatorv1alpha1.ImageContentSourcePolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:   icspName2,
				Labels: map[string]string{"e2e-test": "ocp-70203"},
			},
			Spec: operatorv1alpha1.ImageContentSourcePolicySpec{
				RepositoryDigestMirrors: []operatorv1alpha1.RepositoryDigestMirrors{
					{
						Source: "registry.example.com/example/myimage",
						Mirrors: []string{
							"mirror.example.net/image",
						},
					},
				},
			},
		}
		_, err = operatorClient.ImageContentSourcePolicies().Create(ctx, icsp2, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create ICSP %s", icspName2)
		e2e.Logf("ICSP %s created successfully", icspName2)

		pool.waitForMirrorRollout(icsp2Before)
		e2e.Logf("Custom MCP %s rollout complete after second ICSP creation", pool.PoolName)

		g.By("Step 9: Verify registries.conf updated with ICSP2 entries alongside IDMS and ITMS entries")
		registriesConfAfterICSP2 := pool.readRegistriesConf(ctx)
		e2e.Logf("registries.conf after second ICSP creation: read %d bytes, asserting expected entries", len(registriesConfAfterICSP2))

		o.Expect(registriesConfAfterICSP2).To(o.ContainSubstring(`location = "registry.example.com/example/myimage"`),
			"registries.conf should contain the ICSP2 source")
		o.Expect(registriesConfAfterICSP2).To(o.ContainSubstring(`location = "mirror.example.net/image"`),
			"registries.conf should contain the ICSP2 mirror")
		o.Expect(registriesConfAfterICSP2).To(o.ContainSubstring(`location = "registry.access.redhat.com/ubi8/ubi-minimal"`),
			"registries.conf should still contain IDMS entries for ubi8/ubi-minimal")
		o.Expect(registriesConfAfterICSP2).To(o.ContainSubstring(`location = "registry.access.redhat.com/ubi9/ubi-minimal"`),
			"registries.conf should still contain ITMS entries for ubi9/ubi-minimal")

		g.By("Step 10: Delete IDMS and wait for custom MCP rollout")
		idmsDeleteBefore := pool.captureClusterState(ctx)
		err = configClient.ImageDigestMirrorSets().Delete(ctx, idmsName, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to delete IDMS %s", idmsName)
		e2e.Logf("IDMS %s deleted successfully", idmsName)

		pool.waitForMirrorRollout(idmsDeleteBefore)
		e2e.Logf("Custom MCP %s rollout complete after IDMS deletion", pool.PoolName)

		g.By("Step 11: Verify registries.conf - IDMS entries removed, ITMS and ICSP2 entries remain")
		registriesConfAfterIDMSDelete := pool.readRegistriesConf(ctx)
		e2e.Logf("registries.conf after IDMS deletion: read %d bytes, asserting expected entries", len(registriesConfAfterIDMSDelete))

		o.Expect(registriesConfAfterIDMSDelete).NotTo(o.ContainSubstring(`location = "registry.access.redhat.com/ubi8/ubi-minimal"`),
			"registries.conf should not contain IDMS source registry.access.redhat.com/ubi8/ubi-minimal after IDMS deletion")
		o.Expect(registriesConfAfterIDMSDelete).NotTo(o.ContainSubstring(`location = "registry.redhat.io/openshift5"`),
			"registries.conf should not contain IDMS source registry.redhat.io/openshift5 after IDMS deletion")
		o.Expect(registriesConfAfterIDMSDelete).To(o.ContainSubstring(`location = "registry.access.redhat.com/ubi9/ubi-minimal"`),
			"registries.conf should still contain ITMS entries for ubi9/ubi-minimal")
		o.Expect(registriesConfAfterIDMSDelete).To(o.ContainSubstring(`location = "registry.example.com/example/myimage"`),
			"registries.conf should still contain ICSP2 entries for registry.example.com/example/myimage")
	})

	// author: aksjadha@redhat.com
	g.It("[Skipped:Disconnected] Verify IDMS mirror configuration applied to Podman and CRI-O with fallback behavior [OCPNODE-4812]", func(ctx context.Context) {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to check if cluster is MicroShift")

		topology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get control plane topology")
		clusterType := string(*topology)

		if isMicroShift || clusterType == "External" {
			g.Skip("IDMS and MachineConfigPool are not available on MicroShift or External topology clusters")
		}

		configClient := oc.AdminConfigClient().ConfigV1()
		suffix := utilrand.String(5)
		idmsName := fmt.Sprintf("podman-crio-fallback-%s", suffix)
		testImage := "registry.access.redhat.com/ubi9/ubi-minimal"
		var pullImageByDigest string
		var testNode string
		var nodeRole string

		g.By("Select test node based on cluster topology")
		e2e.Logf("Cluster topology: %s", clusterType)

		// Only pause master pool if NOT running on SNO (SingleReplica)
		// SNO needs master MCP to rollout for IDMS changes
		if clusterType != "SingleReplica" {
			g.By("Pause master pool to avoid master drains during test")
			pauseMasterPool(oc)
			g.DeferCleanup(func() { unpauseMasterPool(oc) })
		}

		workerNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/worker",
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list worker nodes")

		var pureWorkerNode string
		if len(workerNodes.Items) > 0 {
			for _, node := range workerNodes.Items {
				if _, hasMasterLabel := node.Labels["node-role.kubernetes.io/master"]; !hasMasterLabel {
					pureWorkerNode = node.Name
					break
				}
			}
		}

		if pureWorkerNode != "" {
			testNode = pureWorkerNode
			nodeRole = "worker"
		} else if len(workerNodes.Items) > 0 {
			testNode = workerNodes.Items[0].Name
			nodeRole = "master"
		} else {
			masterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
				LabelSelector: "node-role.kubernetes.io/master",
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list master nodes")
			o.Expect(masterNodes.Items).NotTo(o.BeEmpty(), "No master nodes found")
			testNode = masterNodes.Items[0].Name
			nodeRole = "master"
		}
		e2e.Logf("Selected test node with role: %s", nodeRole)

		cleanupIDMS := func(ctx context.Context) {
			g.By("Cleanup: Delete IDMS and wait for MCP rollout")
			err := configClient.ImageDigestMirrorSets().Delete(ctx, idmsName, metav1.DeleteOptions{})
			if err == nil {
				e2e.Logf("IDMS %s deleted", idmsName)

				mcpName := "worker"
				if nodeRole == "master" {
					mcpName = "master"
				}

				e2e.Logf("Waiting for MCP %s rollout to start after IDMS deletion", mcpName)
				waitErr := wait.PollImmediate(5*time.Second, 3*time.Minute, func() (bool, error) {
					updating, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp/"+mcpName,
						"-o=jsonpath={.status.conditions[?(@.type=='Updating')].status}").Output()
					return fmt.Sprintf("%s", updating) == "True", nil
				})
				o.Expect(waitErr).NotTo(o.HaveOccurred(), "MCP rollout did not start after IDMS deletion")

				e2e.Logf("Waiting for MCP %s rollout to complete after IDMS deletion", mcpName)
				waitErr = wait.PollImmediate(10*time.Second, 10*time.Minute, func() (bool, error) {
					updated, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp/"+mcpName,
						"-o=jsonpath={.status.conditions[?(@.type=='Updated')].status}").Output()
					updating, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp/"+mcpName,
						"-o=jsonpath={.status.conditions[?(@.type=='Updating')].status}").Output()
					return fmt.Sprintf("%s", updated) == "True" && fmt.Sprintf("%s", updating) == "False", nil
				})
				o.Expect(waitErr).NotTo(o.HaveOccurred(), "MCP rollout did not complete after IDMS deletion")
			}

			if pullImageByDigest != "" {
				nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
					fmt.Sprintf("crictl rmi %s 2>/dev/null || true; podman rmi %s 2>/dev/null || true", pullImageByDigest, pullImageByDigest))
			}
			nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				fmt.Sprintf("crictl rmi %s 2>/dev/null || true; podman rmi %s 2>/dev/null || true", testImage, testImage))
		}
		g.DeferCleanup(cleanupIDMS)

		g.By("Step 1: Create ImageDigestMirrorSet with fake mirror and real source as fallback")
		idms := &configv1.ImageDigestMirrorSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: idmsName,
			},
			Spec: configv1.ImageDigestMirrorSetSpec{
				ImageDigestMirrors: []configv1.ImageDigestMirrors{
					{
						Source: "registry.access.redhat.com/ubi9/ubi-minimal",
						Mirrors: []configv1.ImageMirror{
							"quay.io/does-not-exist-idms-test/ubi9-minimal",
							"registry.access.redhat.com/ubi9/ubi-minimal",
						},
						MirrorSourcePolicy: configv1.AllowContactingSource,
					},
				},
			},
		}

		_, err = configClient.ImageDigestMirrorSets().Create(ctx, idms, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create IDMS")
		e2e.Logf("Created IDMS %s with fake mirror quay.io/does-not-exist-idms-test", idmsName)

		mcpName := "worker"
		if clusterType == "SingleReplica" {
			mcpName = "master"
		}

		g.By(fmt.Sprintf("Step 2: Wait for MCP %s rollout to start after IDMS creation", mcpName))
		waitErr := wait.PollImmediate(5*time.Second, 3*time.Minute, func() (bool, error) {
			updating, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp/"+mcpName,
				"-o=jsonpath={.status.conditions[?(@.type=='Updating')].status}").Output()
			return fmt.Sprintf("%s", updating) == "True", nil
		})
		o.Expect(waitErr).NotTo(o.HaveOccurred(), "MCP rollout did not start after IDMS creation")

		g.By(fmt.Sprintf("Step 3: Wait for MCP %s rollout to complete", mcpName))
		waitErr = wait.PollImmediate(10*time.Second, 10*time.Minute, func() (bool, error) {
			updated, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp/"+mcpName,
				"-o=jsonpath={.status.conditions[?(@.type=='Updated')].status}").Output()
			updating, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp/"+mcpName,
				"-o=jsonpath={.status.conditions[?(@.type=='Updating')].status}").Output()
			return fmt.Sprintf("%s", updated) == "True" && fmt.Sprintf("%s", updating) == "False", nil
		})
		o.Expect(waitErr).NotTo(o.HaveOccurred(), "MCP rollout did not complete")
		e2e.Logf("MCP %s rollout completed", mcpName)

		g.By("Step 4: Verify IDMS mirror configuration is present in node registries.conf")
		var mirrorFound bool
		waitErr = wait.PollImmediate(10*time.Second, 5*time.Minute, func() (bool, error) {
			mirrorConfig, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
				"cat /etc/containers/registries.conf | grep -A6 ubi-minimal || true")
			if err == nil && fmt.Sprintf("%s", mirrorConfig) != "" {
				configStr := fmt.Sprintf("%s", mirrorConfig)
				if len(configStr) > 0 {
					mirrorFound = true
					return true, nil
				}
			}
			return false, nil
		})
		o.Expect(waitErr).NotTo(o.HaveOccurred(), "IDMS mirror config not found in node registries.conf")
		o.Expect(mirrorFound).To(o.BeTrue(), "IDMS mirror was not applied to registries.conf on node")
		e2e.Logf("Verified IDMS mirror configuration in registries.conf")

		g.By("Step 5: Clear image cache from Podman and CRI-O shared storage")
		nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			fmt.Sprintf("crictl rmi %s 2>/dev/null || true; podman rmi %s 2>/dev/null || true", testImage, testImage))

		crioCheck, _ := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"crictl images | grep ubi-minimal || true")
		podmanCheck, _ := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"podman images | grep ubi-minimal || true")
		o.Expect(fmt.Sprintf("%s", crioCheck)).To(o.BeEmpty(), "CRI-O cache not cleared")
		o.Expect(fmt.Sprintf("%s", podmanCheck)).To(o.BeEmpty(), "Podman cache not cleared")

		g.By("Step 6: Pull image with CRI-O to get it into storage, then get digest using crictlImageDigest")
		_, err = nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "pull", testImage)
		o.Expect(err).NotTo(o.HaveOccurred(), "CRI-O pull failed for getting digest")

		digestHash, err := crictlImageDigest(ctx, oc, testNode, testImage)
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get image digest from CRI-O")
		o.Expect(digestHash).NotTo(o.BeEmpty(), "Image digest is empty")

		pullImageByDigest = fmt.Sprintf("%s@sha256:%s", testImage, digestHash)
		e2e.Logf("Image digest from CRI-O: sha256:%s", digestHash)
		e2e.Logf("Pull by digest reference: %s", pullImageByDigest)

		g.By("Step 7: Clear cache and pull image by digest via CRI-O to verify fake mirror was attempted")
		nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			fmt.Sprintf("crictl rmi %s 2>/dev/null || true; podman rmi %s 2>/dev/null || true", testImage, testImage))
		_, err = nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "pull", pullImageByDigest)
		o.Expect(err).NotTo(o.HaveOccurred(), "CRI-O pull by digest failed")

		crioLogs, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"journalctl -u crio --since '-2m'")
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get CRI-O journal logs")
		o.Expect(crioLogs).To(o.ContainSubstring("does-not-exist-idms-test"),
			"CRI-O did not attempt fake mirror - IDMS configuration was not applied by CRI-O")
		e2e.Logf("Verified CRI-O attempted fake mirror and fell back to real source")

		g.By("Step 8: Clear cache again and pull image by digest via Podman with debug logging")
		nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			fmt.Sprintf("crictl rmi %s 2>/dev/null || true; podman rmi %s 2>/dev/null || true", pullImageByDigest, pullImageByDigest))

		crioVerify, _ := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"crictl images | grep ubi-minimal || true")
		podmanVerify, _ := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			"podman images | grep ubi-minimal || true")
		o.Expect(fmt.Sprintf("%s", crioVerify)).To(o.BeEmpty(), "CRI-O cache not cleared before Podman test")
		o.Expect(fmt.Sprintf("%s", podmanVerify)).To(o.BeEmpty(), "Podman cache not cleared before Podman test")

		g.By("Step 9: Pull image by digest via Podman and verify fake mirror was attempted")
		podmanOutput, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, testNode, "sh", "-c",
			fmt.Sprintf("podman pull --log-level=debug %s 2>&1", pullImageByDigest))
		o.Expect(err).NotTo(o.HaveOccurred(), "Podman pull by digest failed")

		o.Expect(podmanOutput).To(o.ContainSubstring("does-not-exist-idms-test"),
			"Podman did not attempt fake mirror - IDMS configuration was not applied by Podman")
		e2e.Logf("Verified Podman attempted fake mirror and fell back to real source")
	})
})

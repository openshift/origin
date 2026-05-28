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
	ctx       context.Context
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

	patchData := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:""}}}`, nodeLabel))
	_, err = oc.AdminKubeClient().CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, patchData, metav1.PatchOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to label node %s for custom MCP", nodeName)

	pool := &mirrorTestPool{
		oc:        oc,
		ctx:       ctx,
		mcClient:  mcClient,
		PoolName:  poolName,
		NodeName:  nodeName,
		nodeLabel: nodeLabel,
	}

	err = waitForMirrorTestMCPReady(ctx, mcClient, poolName, 10*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred(), "custom MachineConfigPool %s did not become ready", poolName)
	e2e.Logf("Custom mirror test pool %s ready on node %s", poolName, nodeName)

	return pool
}

func (p *mirrorTestPool) currentSpec() string {
	return imagepolicy.GetMCPCurrentSpecConfigName(p.oc, p.PoolName)
}

func (p *mirrorTestPool) waitForRollout(initialSpec string) {
	imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(p.oc, p.PoolName, initialSpec)
}

func (p *mirrorTestPool) readRegistriesConf() string {
	registriesConf, err := nodeutils.ExecOnNodeWithChroot(p.oc, p.NodeName, "cat", "/etc/containers/registries.conf")
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to read registries.conf from node %s", p.NodeName)
	return registriesConf
}

func (p *mirrorTestPool) Teardown() {
	e2e.Logf("Teardown: removing label %s from node %s", p.nodeLabel, p.NodeName)
	removePatch := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:null}}}`, p.nodeLabel))
	_, patchErr := p.oc.AdminKubeClient().CoreV1().Nodes().Patch(p.ctx, p.NodeName, types.MergePatchType, removePatch, metav1.PatchOptions{})
	if patchErr != nil && !apierrors.IsNotFound(patchErr) {
		e2e.Logf("Warning: failed to remove label from node %s: %v", p.NodeName, patchErr)
	}

	e2e.Logf("Teardown: waiting for node %s to transition back to worker pool", p.NodeName)
	o.Eventually(func() bool {
		currentNode, getErr := p.oc.AdminKubeClient().CoreV1().Nodes().Get(p.ctx, p.NodeName, metav1.GetOptions{})
		if getErr != nil {
			return false
		}
		currentConfig := currentNode.Annotations["machineconfiguration.openshift.io/currentConfig"]
		desiredConfig := currentNode.Annotations["machineconfiguration.openshift.io/desiredConfig"]
		return currentConfig != "" && !strings.Contains(currentConfig, p.PoolName) && currentConfig == desiredConfig
	}, 15*time.Minute, 15*time.Second).Should(o.BeTrue(),
		"node %s should transition back to worker pool", p.NodeName)

	deleteErr := p.mcClient.MachineconfigurationV1().MachineConfigPools().Delete(p.ctx, p.PoolName, metav1.DeleteOptions{})
	if deleteErr != nil && !apierrors.IsNotFound(deleteErr) {
		e2e.Logf("Warning: failed to delete MachineConfigPool %s: %v", p.PoolName, deleteErr)
	}
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
		oc  = exutil.NewCLIWithoutNamespace("image-mirror-set")
		ctx = context.Background()
	)

	g.BeforeEach(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Skipping test on MicroShift cluster - MachineConfig resources are not available")
		}
	})

	g.It("[OTP] Create ImageDigestMirrorSet and ImageTagMirrorSet and verify registries.conf [OCP-57401]", func() {
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

		initialSpec := pool.currentSpec()

		createdIDMS, err := configClient.ImageDigestMirrorSets().Create(ctx, idms, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create ImageDigestMirrorSet")
		e2e.Logf("ImageDigestMirrorSet %q created successfully", createdIDMS.Name)

		g.DeferCleanup(func() {
			g.By("Cleanup: Delete IDMS and ITMS resources")
			cleanupSpec := pool.currentSpec()
			if delErr := configClient.ImageTagMirrorSets().Delete(ctx, itmsName, metav1.DeleteOptions{}); delErr != nil {
				e2e.Logf("Warning: failed to delete ImageTagMirrorSet: %v", delErr)
			}
			if delErr := configClient.ImageDigestMirrorSets().Delete(ctx, idmsName, metav1.DeleteOptions{}); delErr != nil {
				e2e.Logf("Warning: failed to delete ImageDigestMirrorSet: %v", delErr)
			}
			pool.waitForRollout(cleanupSpec)
		})

		pool.waitForRollout(initialSpec)
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

		itmsInitialSpec := pool.currentSpec()

		createdITMS, err := configClient.ImageTagMirrorSets().Create(ctx, itms, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create ImageTagMirrorSet")
		e2e.Logf("ImageTagMirrorSet %q created successfully", createdITMS.Name)

		g.By("Step 3: Wait for custom MCP to finish rolling out")
		pool.waitForRollout(itmsInitialSpec)
		e2e.Logf("Custom MCP %s finished rolling out after ITMS creation", pool.PoolName)

		g.By("Step 4: Verify /etc/containers/registries.conf on the custom pool node")
		registriesConf := pool.readRegistriesConf()
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
	g.It("[OTP] ICSP and IDMS/ITMS can coexist in cluster [OCP-70203]", ote.Informing(), func() {
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

		initialSpec := pool.currentSpec()

		_, err := operatorClient.ImageContentSourcePolicies().Create(ctx, icsp1, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create ICSP %s", icspName1)
		e2e.Logf("ICSP %s created successfully", icspName1)

		g.DeferCleanup(func() {
			g.By("Cleanup: Delete any remaining test resources and wait for custom MCP to settle")
			cleanupSpec := pool.currentSpec()
			toDelete := false

			for _, name := range []string{icspName1, icspName2} {
				if _, getErr := operatorClient.ImageContentSourcePolicies().Get(ctx, name, metav1.GetOptions{}); getErr == nil {
					if delErr := operatorClient.ImageContentSourcePolicies().Delete(ctx, name, metav1.DeleteOptions{}); delErr == nil {
						e2e.Logf("Cleanup: deleted ICSP %s", name)
						toDelete = true
					} else {
						e2e.Logf("Cleanup: warning - failed to delete ICSP %s: %v", name, delErr)
					}
				}
			}
			if _, getErr := configClient.ImageTagMirrorSets().Get(ctx, itmsName, metav1.GetOptions{}); getErr == nil {
				if delErr := configClient.ImageTagMirrorSets().Delete(ctx, itmsName, metav1.DeleteOptions{}); delErr == nil {
					e2e.Logf("Cleanup: deleted ITMS %s", itmsName)
					toDelete = true
				} else {
					e2e.Logf("Cleanup: warning - failed to delete ITMS %s: %v", itmsName, delErr)
				}
			}
			if _, getErr := configClient.ImageDigestMirrorSets().Get(ctx, idmsName, metav1.GetOptions{}); getErr == nil {
				if delErr := configClient.ImageDigestMirrorSets().Delete(ctx, idmsName, metav1.DeleteOptions{}); delErr == nil {
					e2e.Logf("Cleanup: deleted IDMS %s", idmsName)
					toDelete = true
				} else {
					e2e.Logf("Cleanup: warning - failed to delete IDMS %s: %v", idmsName, delErr)
				}
			}
			if toDelete {
				pool.waitForRollout(cleanupSpec)
			}
		})

		g.By("Step 2: Wait for custom MCP rollout after ICSP creation and verify registries.conf")
		pool.waitForRollout(initialSpec)
		e2e.Logf("Custom MCP %s rollout complete after ICSP creation", pool.PoolName)

		registriesConf := pool.readRegistriesConf()
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
		registriesConfAfterICSPDelete := pool.readRegistriesConf()
		o.Expect(registriesConfAfterICSPDelete).To(o.Equal(registriesConf),
			"registries.conf should be unchanged after ICSP deletion when IDMS covers the same mirror config")
		e2e.Logf("Confirmed: registries.conf unchanged after ICSP deletion")

		g.By("Step 6: Create ITMS with tag mirrors for ubi9/ubi-minimal (different source from IDMS)")
		itmsInitialSpec := pool.currentSpec()
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

		pool.waitForRollout(itmsInitialSpec)
		e2e.Logf("Custom MCP %s rollout complete after ITMS creation", pool.PoolName)

		g.By("Step 7: Verify registries.conf updated with ITMS tag-only entries alongside IDMS digest entries")
		registriesConfAfterITMS := pool.readRegistriesConf()
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
		icsp2InitialSpec := pool.currentSpec()
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

		pool.waitForRollout(icsp2InitialSpec)
		e2e.Logf("Custom MCP %s rollout complete after second ICSP creation", pool.PoolName)

		g.By("Step 9: Verify registries.conf updated with ICSP2 entries alongside IDMS and ITMS entries")
		registriesConfAfterICSP2 := pool.readRegistriesConf()
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
		idmsDeleteInitialSpec := pool.currentSpec()
		err = configClient.ImageDigestMirrorSets().Delete(ctx, idmsName, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to delete IDMS %s", idmsName)
		e2e.Logf("IDMS %s deleted successfully", idmsName)

		pool.waitForRollout(idmsDeleteInitialSpec)
		e2e.Logf("Custom MCP %s rollout complete after IDMS deletion", pool.PoolName)

		g.By("Step 11: Verify registries.conf - IDMS entries removed, ITMS and ICSP2 entries remain")
		registriesConfAfterIDMSDelete := pool.readRegistriesConf()
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
})

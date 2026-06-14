package node

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"
	configv1 "github.com/openshift/api/config/v1"
	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	"github.com/openshift/origin/test/extended/imagepolicy"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	nodeutils "github.com/openshift/origin/test/extended/node"
	exutil "github.com/openshift/origin/test/extended/util"
)

// pollMCPSpecUnchanged polls both worker and master MCP spec names every 15 seconds for the
// given duration using wait.PollImmediate. It returns an error if either spec deviates from
// the provided baseline values, or nil if the spec remained stable for the full duration.
func pollMCPSpecUnchanged(oc *exutil.CLI, workerSpec, masterSpec string, duration time.Duration) error {
	err := wait.PollImmediate(15*time.Second, duration, func() (bool, error) {
		if w := imagepolicy.GetMCPCurrentSpecConfigName(oc, "worker"); w != workerSpec {
			return false, fmt.Errorf("worker MCP spec changed from %s to %s", workerSpec, w)
		}
		if m := imagepolicy.GetMCPCurrentSpecConfigName(oc, "master"); m != masterSpec {
			return false, fmt.Errorf("master MCP spec changed from %s to %s", masterSpec, m)
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

		mcClient, err := machineconfigclient.NewForConfig(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create MachineConfig client")
		for _, pool := range []string{"worker", "master"} {
			mcp, err := mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, pool, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get MCP %s", pool)
			isUpdated := false
			for _, condition := range mcp.Status.Conditions {
				if condition.Type == "Updated" && condition.Status == corev1.ConditionTrue {
					isUpdated = true
					break
				}
			}
			o.Expect(isUpdated).To(o.BeTrue(),
				"MCP %s must be in Updated=True state before running this test (spec=%s, status=%s)",
				pool, mcp.Spec.Configuration.Name, mcp.Status.Configuration.Name)
			o.Expect(mcp.Spec.Configuration.Name).To(o.Equal(mcp.Status.Configuration.Name),
				"MCP %s spec and status configuration names must match before running this test", pool)
		}
	})

	g.It("[OTP] Create ImageDigestMirrorSet and ImageTagMirrorSet and verify registries.conf [OCP-57401]", func() {
		configClient := oc.AdminConfigClient().ConfigV1()
		suffix := utilrand.String(5)
		idmsName := fmt.Sprintf("digest-mirror-%s", suffix)
		itmsName := fmt.Sprintf("tag-mirror-%s", suffix)

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

		initialWorkerSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "worker")
		initialMasterSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "master")

		createdIDMS, err := configClient.ImageDigestMirrorSets().Create(ctx, idms, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create ImageDigestMirrorSet")
		e2e.Logf("ImageDigestMirrorSet %q created successfully", createdIDMS.Name)

		g.DeferCleanup(func() {
			g.By("Cleanup: Delete IDMS and ITMS resources")
			cleanupWorkerSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "worker")
			cleanupMasterSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "master")
			if delErr := configClient.ImageTagMirrorSets().Delete(ctx, itmsName, metav1.DeleteOptions{}); delErr != nil {
				e2e.Logf("Warning: failed to delete ImageTagMirrorSet: %v", delErr)
			}
			if delErr := configClient.ImageDigestMirrorSets().Delete(ctx, idmsName, metav1.DeleteOptions{}); delErr != nil {
				e2e.Logf("Warning: failed to delete ImageDigestMirrorSet: %v", delErr)
			}
			imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "worker", cleanupWorkerSpec)
			imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "master", cleanupMasterSpec)
		})

		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "worker", initialWorkerSpec)
		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "master", initialMasterSpec)
		e2e.Logf("IDMS MCP rollout complete")

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

		itmsWorkerSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "worker")
		itmsMasterSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "master")

		createdITMS, err := configClient.ImageTagMirrorSets().Create(ctx, itms, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create ImageTagMirrorSet")
		e2e.Logf("ImageTagMirrorSet %q created successfully", createdITMS.Name)

		g.By("Step 3: Wait for all nodes to finish rolling out")
		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "worker", itmsWorkerSpec)
		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "master", itmsMasterSpec)
		e2e.Logf("All MCPs have finished rolling out")

		g.By("Step 4: Verify /etc/containers/registries.conf on a worker node")
		workerNodeName := nodeutils.GetFirstReadyWorkerNode(oc)
		o.Expect(workerNodeName).NotTo(o.BeEmpty(), "no ready worker node found")

		registriesConf, err := nodeutils.ExecOnNodeWithChroot(oc, workerNodeName, "cat", "/etc/containers/registries.conf")
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to read registries.conf from node %s", workerNodeName)
		e2e.Logf("registries.conf content:\n%s", registriesConf)

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
	//
	// Test 1 of 3 for OCP-70203: verifies that creating an IDMS with the same mirror config as an
	// existing ICSP does not trigger a new MCP rollout, and that deleting the ICSP while the IDMS
	// is present also does not trigger a rollout .
	g.It("[OTP] ICSP and IDMS coexistence does not trigger redundant MCP rollout [OCP-70203]", ote.Informing(), func() {
		configClient := oc.AdminConfigClient().ConfigV1()
		operatorClient := oc.AdminOperatorClient().OperatorV1alpha1()
		suffix := utilrand.String(5)
		icspName := fmt.Sprintf("ubi8repo-%s", suffix)
		idmsName := fmt.Sprintf("digest-mirror-%s", suffix)

		g.By("Step 1: Create ICSP with digest mirrors for ubi8/ubi-minimal and openshift5")
		icsp := &operatorv1alpha1.ImageContentSourcePolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:   icspName,
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

		initialWorkerSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "worker")
		initialMasterSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "master")

		_, err := operatorClient.ImageContentSourcePolicies().Create(ctx, icsp, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create ICSP %s", icspName)
		e2e.Logf("ICSP %s created successfully", icspName)

		g.DeferCleanup(func() {
			g.By("Cleanup: Delete remaining ICSP and IDMS, wait for MCP to settle")
			cleanupWorkerSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "worker")
			cleanupMasterSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "master")
			toDelete := false

			if _, getErr := operatorClient.ImageContentSourcePolicies().Get(ctx, icspName, metav1.GetOptions{}); getErr == nil {
				if delErr := operatorClient.ImageContentSourcePolicies().Delete(ctx, icspName, metav1.DeleteOptions{}); delErr == nil {
					e2e.Logf("Cleanup: deleted ICSP %s", icspName)
					toDelete = true
				} else {
					e2e.Logf("Cleanup: warning - failed to delete ICSP %s: %v", icspName, delErr)
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
				imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "worker", cleanupWorkerSpec)
				imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "master", cleanupMasterSpec)
			}
		})

		g.By("Step 2: Wait for MCP rollout after ICSP creation and verify registries.conf")
		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "worker", initialWorkerSpec)
		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "master", initialMasterSpec)
		e2e.Logf("MCP rollout complete after ICSP creation")

		workerNode := nodeutils.GetFirstReadyWorkerNode(oc)
		o.Expect(workerNode).NotTo(o.BeEmpty(), "no ready worker node found")

		registriesConf, err := nodeutils.ExecOnNodeWithChroot(oc, workerNode, "cat", "/etc/containers/registries.conf")
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to read registries.conf from node %s", workerNode)
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
		specBeforeIDMS := imagepolicy.GetMCPCurrentSpecConfigName(oc, "worker")
		masterSpecBeforeIDMS := imagepolicy.GetMCPCurrentSpecConfigName(oc, "master")
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
		o.Expect(pollMCPSpecUnchanged(oc, specBeforeIDMS, masterSpecBeforeIDMS, 2*time.Minute)).
			NotTo(o.HaveOccurred(), "unexpected MCP rollout after IDMS creation with same config as ICSP")
		e2e.Logf("Confirmed: no new MC generated after IDMS creation (worker and master stable for 2 minutes)")

		g.By("Step 4.1: Delete ICSP - IDMS covers the same config so no new MC should be triggered")
		specBeforeICSPDelete := imagepolicy.GetMCPCurrentSpecConfigName(oc, "worker")
		masterSpecBeforeICSPDelete := imagepolicy.GetMCPCurrentSpecConfigName(oc, "master")
		err = operatorClient.ImageContentSourcePolicies().Delete(ctx, icspName, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to delete ICSP %s", icspName)
		e2e.Logf("ICSP %s deleted successfully", icspName)

		g.By("Step 4.2: Confirm no new MC was generated after ICSP deletion (IDMS still covers the same config)")
		o.Expect(pollMCPSpecUnchanged(oc, specBeforeICSPDelete, masterSpecBeforeICSPDelete, 2*time.Minute)).
			NotTo(o.HaveOccurred(), "unexpected MCP rollout after ICSP deletion when IDMS covers the same config")
		e2e.Logf("Confirmed: no new MC generated after ICSP deletion (worker and master stable for 2 minutes)")

		g.By("Step 5: Verify registries.conf is unchanged after ICSP deletion (IDMS maintains same mirror config)")
		registriesConfAfterICSPDelete, err := nodeutils.ExecOnNodeWithChroot(oc, workerNode, "cat", "/etc/containers/registries.conf")
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to read registries.conf from node %s", workerNode)
		o.Expect(registriesConfAfterICSPDelete).To(o.Equal(registriesConf),
			"registries.conf should be unchanged after ICSP deletion when IDMS covers the same mirror config")
		e2e.Logf("Confirmed: registries.conf unchanged after ICSP deletion")
	})

	// author: asahay@redhat.com
	//
	// Test 2 of 3 for OCP-70203: verifies that creating an ITMS with a new (non-overlapping) source
	// triggers an MCP rollout and that registries.conf is updated with both tag-only entries from the
	// ITMS and the existing digest-only entries from the IDMS.
	g.It("[OTP] ITMS tag mirrors appear in registries.conf alongside IDMS digest mirrors [OCP-70203]", ote.Informing(), func() {
		configClient := oc.AdminConfigClient().ConfigV1()
		suffix := utilrand.String(5)
		idmsName := fmt.Sprintf("digest-mirror-%s", suffix)
		itmsName := fmt.Sprintf("tag-mirror-%s", suffix)

		g.By("Step 1: Create IDMS with digest mirrors for ubi8/ubi-minimal")
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
				},
			},
		}

		idmsWorkerSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "worker")
		idmsMasterSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "master")

		_, err := configClient.ImageDigestMirrorSets().Create(ctx, idms, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create IDMS %s", idmsName)
		e2e.Logf("IDMS %s created successfully", idmsName)

		g.DeferCleanup(func() {
			g.By("Cleanup: Delete ITMS and IDMS, wait for MCP to settle")
			cleanupWorkerSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "worker")
			cleanupMasterSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "master")
			toDelete := false

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
				imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "worker", cleanupWorkerSpec)
				imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "master", cleanupMasterSpec)
			}
		})

		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "worker", idmsWorkerSpec)
		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "master", idmsMasterSpec)
		e2e.Logf("MCP rollout complete after IDMS creation")

		g.By("Step 2: Create ITMS with tag mirrors for ubi9/ubi-minimal (different source from IDMS)")
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

		itmsWorkerSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "worker")
		itmsMasterSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "master")

		_, err = configClient.ImageTagMirrorSets().Create(ctx, itms, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create ITMS %s", itmsName)
		e2e.Logf("ITMS %s created successfully", itmsName)

		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "worker", itmsWorkerSpec)
		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "master", itmsMasterSpec)
		e2e.Logf("MCP rollout complete after ITMS creation")

		g.By("Step 3: Verify registries.conf contains both tag-only ITMS entries and digest-only IDMS entries")
		workerNode := nodeutils.GetFirstReadyWorkerNode(oc)
		o.Expect(workerNode).NotTo(o.BeEmpty(), "no ready worker node found")

		registriesConf, err := nodeutils.ExecOnNodeWithChroot(oc, workerNode, "cat", "/etc/containers/registries.conf")
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to read registries.conf from node %s", workerNode)
		e2e.Logf("registries.conf after ITMS creation: read %d bytes, asserting expected entries", len(registriesConf))

		o.Expect(registriesConf).To(o.ContainSubstring(`location = "registry.access.redhat.com/ubi9/ubi-minimal"`),
			"registries.conf should contain the ITMS source ubi9/ubi-minimal")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "example.io/example/ubi-minimal-1"`),
			"registries.conf should contain ITMS mirror example.io/example/ubi-minimal-1")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "example.com/example/ubi-minimal-1"`),
			"registries.conf should contain ITMS mirror example.com/example/ubi-minimal-1")
		o.Expect(registriesConf).To(o.ContainSubstring(`pull-from-mirror = "tag-only"`),
			"registries.conf should have pull-from-mirror = tag-only for ITMS entries")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "registry.access.redhat.com/ubi8/ubi-minimal"`),
			"registries.conf should still contain IDMS entries for ubi8/ubi-minimal")
		o.Expect(registriesConf).To(o.ContainSubstring(`pull-from-mirror = "digest-only"`),
			"registries.conf should still have digest-only entries from IDMS")
	})

	// author: asahay@redhat.com
	//
	// Test 3 of 3 for OCP-70203: verifies that IDMS, ITMS, and a second ICSP can all coexist, and
	// that deleting the IDMS removes only its entries from registries.conf while the ITMS and ICSP
	// entries remain intact.
	g.It("[OTP] Deleting IDMS removes its entries while ITMS and ICSP entries remain [OCP-70203]", ote.Informing(), func() {
		configClient := oc.AdminConfigClient().ConfigV1()
		operatorClient := oc.AdminOperatorClient().OperatorV1alpha1()
		suffix := utilrand.String(5)
		idmsName := fmt.Sprintf("digest-mirror-%s", suffix)
		itmsName := fmt.Sprintf("tag-mirror-%s", suffix)
		icspName := fmt.Sprintf("ubi9repo-%s", suffix)

		g.By("Step 1: Create IDMS with digest mirrors for ubi8/ubi-minimal")
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
				},
			},
		}

		idmsWorkerSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "worker")
		idmsMasterSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "master")

		_, err := configClient.ImageDigestMirrorSets().Create(ctx, idms, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create IDMS %s", idmsName)
		e2e.Logf("IDMS %s created successfully", idmsName)

		g.DeferCleanup(func() {
			g.By("Cleanup: Delete remaining ICSP, ITMS, IDMS and wait for MCP to settle")
			cleanupWorkerSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "worker")
			cleanupMasterSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "master")
			toDelete := false

			if _, getErr := operatorClient.ImageContentSourcePolicies().Get(ctx, icspName, metav1.GetOptions{}); getErr == nil {
				if delErr := operatorClient.ImageContentSourcePolicies().Delete(ctx, icspName, metav1.DeleteOptions{}); delErr == nil {
					e2e.Logf("Cleanup: deleted ICSP %s", icspName)
					toDelete = true
				} else {
					e2e.Logf("Cleanup: warning - failed to delete ICSP %s: %v", icspName, delErr)
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
				imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "worker", cleanupWorkerSpec)
				imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "master", cleanupMasterSpec)
			}
		})

		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "worker", idmsWorkerSpec)
		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "master", idmsMasterSpec)
		e2e.Logf("MCP rollout complete after IDMS creation")

		g.By("Step 2: Create ITMS with tag mirrors for ubi9/ubi-minimal")
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

		itmsWorkerSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "worker")
		itmsMasterSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "master")

		_, err = configClient.ImageTagMirrorSets().Create(ctx, itms, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create ITMS %s", itmsName)
		e2e.Logf("ITMS %s created successfully", itmsName)

		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "worker", itmsWorkerSpec)
		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "master", itmsMasterSpec)
		e2e.Logf("MCP rollout complete after ITMS creation")

		g.By("Step 3: Create ICSP with digest mirrors for registry.example.com/example/myimage")
		icsp := &operatorv1alpha1.ImageContentSourcePolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:   icspName,
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

		icspWorkerSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "worker")
		icspMasterSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "master")

		_, err = operatorClient.ImageContentSourcePolicies().Create(ctx, icsp, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create ICSP %s", icspName)
		e2e.Logf("ICSP %s created successfully", icspName)

		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "worker", icspWorkerSpec)
		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "master", icspMasterSpec)
		e2e.Logf("MCP rollout complete after ICSP creation")

		g.By("Step 4: Verify registries.conf contains entries from all three resource types")
		workerNode := nodeutils.GetFirstReadyWorkerNode(oc)
		o.Expect(workerNode).NotTo(o.BeEmpty(), "no ready worker node found")

		registriesConf, err := nodeutils.ExecOnNodeWithChroot(oc, workerNode, "cat", "/etc/containers/registries.conf")
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to read registries.conf from node %s", workerNode)
		e2e.Logf("registries.conf after all resources created: read %d bytes, asserting expected entries", len(registriesConf))

		o.Expect(registriesConf).To(o.ContainSubstring(`location = "registry.access.redhat.com/ubi8/ubi-minimal"`),
			"registries.conf should contain IDMS entries for ubi8/ubi-minimal")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "registry.access.redhat.com/ubi9/ubi-minimal"`),
			"registries.conf should contain ITMS entries for ubi9/ubi-minimal")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "registry.example.com/example/myimage"`),
			"registries.conf should contain ICSP entries for registry.example.com/example/myimage")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "mirror.example.net/image"`),
			"registries.conf should contain ICSP mirror mirror.example.net/image")

		g.By("Step 5: Delete IDMS and wait for MCP rollout")
		idmsDeleteWorkerSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "worker")
		idmsDeleteMasterSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "master")

		err = configClient.ImageDigestMirrorSets().Delete(ctx, idmsName, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to delete IDMS %s", idmsName)
		e2e.Logf("IDMS %s deleted successfully", idmsName)

		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "worker", idmsDeleteWorkerSpec)
		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "master", idmsDeleteMasterSpec)
		e2e.Logf("MCP rollout complete after IDMS deletion")

		g.By("Step 6: Verify registries.conf - IDMS entries removed, ITMS and ICSP entries remain")
		registriesConfAfterIDMSDelete, err := nodeutils.ExecOnNodeWithChroot(oc, workerNode, "cat", "/etc/containers/registries.conf")
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to read registries.conf from node %s", workerNode)
		e2e.Logf("registries.conf after IDMS deletion: read %d bytes, asserting expected entries", len(registriesConfAfterIDMSDelete))

		o.Expect(registriesConfAfterIDMSDelete).NotTo(o.ContainSubstring(`location = "registry.access.redhat.com/ubi8/ubi-minimal"`),
			"registries.conf should not contain IDMS source registry.access.redhat.com/ubi8/ubi-minimal after IDMS deletion")
		o.Expect(registriesConfAfterIDMSDelete).To(o.ContainSubstring(`location = "registry.access.redhat.com/ubi9/ubi-minimal"`),
			"registries.conf should still contain ITMS entries for ubi9/ubi-minimal")
		o.Expect(registriesConfAfterIDMSDelete).To(o.ContainSubstring(`location = "registry.example.com/example/myimage"`),
			"registries.conf should still contain ICSP entries for registry.example.com/example/myimage")
	})
})

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
	"github.com/openshift/origin/test/extended/imagepolicy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	nodeutils "github.com/openshift/origin/test/extended/node"
	exutil "github.com/openshift/origin/test/extended/util"
)

func getWorkerMCPSpec(oc *exutil.CLI) string {
	return imagepolicy.GetMCPCurrentSpecConfigName(oc, "worker")
}

func getMCPSpecs(oc *exutil.CLI) (workerSpec, masterSpec string) {
	return imagepolicy.GetMCPCurrentSpecConfigName(oc, "worker"),
		imagepolicy.GetMCPCurrentSpecConfigName(oc, "master")
}

// waitForWorkerRollout waits for the worker pool only. Used during test steps
// since master pool is paused and we only verify registries.conf on worker nodes.
func waitForWorkerRollout(oc *exutil.CLI, workerSpec string) {
	imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "worker", workerSpec)
}

// pauseMasterPool pauses the master MCP so it does not drain during test steps.
// Only worker nodes roll out, keeping the test fast.
func pauseMasterPool(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
		"machineconfigpool/master", "--type=merge", "-p", `{"spec":{"paused":true}}`).Execute()
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to pause master pool")
	e2e.Logf("Master pool paused")
}

// unpauseMasterPool unpauses the master MCP. After all test resources are deleted,
// MCO sees the master config matches current state, so no drain is needed.
func unpauseMasterPool(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
		"machineconfigpool/master", "--type=merge", "-p", `{"spec":{"paused":false}}`).Execute()
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to unpause master pool")
	e2e.Logf("Master pool unpaused")
}

func readRegistriesConfOnWorker(ctx context.Context, oc *exutil.CLI) string {
	nodeName := nodeutils.GetFirstReadyWorkerNode(oc)
	o.Expect(nodeName).NotTo(o.BeEmpty(), "no ready worker node found")
	registriesConf, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, nodeName, "cat", "/etc/containers/registries.conf")
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to read registries.conf from node %s", nodeName)
	return registriesConf
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

		g.By("Pause master pool to avoid master drains during test")
		pauseMasterPool(oc)
		g.DeferCleanup(func() { unpauseMasterPool(oc) })

		workerSpec := getWorkerMCPSpec(oc)

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
		_, err := configClient.ImageDigestMirrorSets().Create(ctx, idms, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create ImageDigestMirrorSet")
		e2e.Logf("ImageDigestMirrorSet %q created successfully", idmsName)

		g.DeferCleanup(func() {
			g.By("Cleanup: Delete IDMS and ITMS resources")
			wSpec := getWorkerMCPSpec(oc)
			deleted := false
			if delErr := configClient.ImageTagMirrorSets().Delete(context.Background(), itmsName, metav1.DeleteOptions{}); delErr == nil {
				deleted = true
			} else {
				e2e.Logf("Warning: failed to delete ITMS %s: %v", itmsName, delErr)
			}
			if delErr := configClient.ImageDigestMirrorSets().Delete(context.Background(), idmsName, metav1.DeleteOptions{}); delErr == nil {
				deleted = true
			} else {
				e2e.Logf("Warning: failed to delete IDMS %s: %v", idmsName, delErr)
			}
			if deleted {
				waitForWorkerRollout(oc, wSpec)
			}
		})

		g.By("Step 2: Wait for worker MCP rollout and verify IDMS entries in registries.conf")
		waitForWorkerRollout(oc, workerSpec)
		e2e.Logf("Worker MCP rollout complete after IDMS creation")

		registriesConf := readRegistriesConfOnWorker(ctx, oc)
		e2e.Logf("registries.conf after IDMS: read %d bytes", len(registriesConf))

		o.Expect(registriesConf).To(o.ContainSubstring(`location = "registry.redhat.io/openshift4"`),
			"registries.conf should contain the IDMS source for openshift4")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "mirror.example.com/redhat"`),
			"registries.conf should contain the IDMS mirror for openshift4")
		o.Expect(registriesConf).To(o.ContainSubstring(`pull-from-mirror = "digest-only"`),
			"registries.conf should have pull-from-mirror set to digest-only for IDMS mirrors")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "registry.redhat.io/rhel8"`),
			"registries.conf should contain the IDMS source for rhel8")
		o.Expect(registriesConf).To(o.ContainSubstring("location = \"registry.redhat.io/rhel8\"\n  blocked = true"),
			"registry.redhat.io/rhel8 should be blocked (NeverContactSource)")

		g.By("Step 3: Create an ImageTagMirrorSet")
		workerSpec = getWorkerMCPSpec(oc)
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
		_, err = configClient.ImageTagMirrorSets().Create(ctx, itms, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create ImageTagMirrorSet")
		e2e.Logf("ImageTagMirrorSet %q created successfully", itmsName)

		g.By("Step 4: Wait for worker MCP rollout and verify ITMS entries alongside IDMS")
		waitForWorkerRollout(oc, workerSpec)
		e2e.Logf("Worker MCP rollout complete after ITMS creation")

		registriesConf = readRegistriesConfOnWorker(ctx, oc)
		e2e.Logf("registries.conf after ITMS: read %d bytes", len(registriesConf))

		o.Expect(registriesConf).To(o.ContainSubstring(`location = "registry.access.redhat.com/ubi8/ubi-minimal"`),
			"registries.conf should contain the ITMS source for ubi-minimal")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "example.io/example/ubi-minimal"`),
			"registries.conf should contain the ITMS mirror location")
		o.Expect(registriesConf).To(o.ContainSubstring(`pull-from-mirror = "tag-only"`),
			"registries.conf should have pull-from-mirror set to tag-only for ITMS mirrors")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "registry.access.redhat.com/ubi8/ubi-minimal-1"`),
			"registries.conf should contain the ITMS source for ubi-minimal-1")
		o.Expect(registriesConf).To(o.ContainSubstring("location = \"registry.access.redhat.com/ubi8/ubi-minimal-1\"\n  blocked = true"),
			"registry.access.redhat.com/ubi8/ubi-minimal-1 should be blocked (NeverContactSource)")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "registry.redhat.io/openshift4"`),
			"registries.conf should still contain IDMS entries after ITMS creation")
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

		g.By("Pause master pool to avoid master drains during test")
		pauseMasterPool(oc)
		g.DeferCleanup(func() { unpauseMasterPool(oc) })

		workerSpec := getWorkerMCPSpec(oc)

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

		_, err := operatorClient.ImageContentSourcePolicies().Create(ctx, icsp1, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create ICSP %s", icspName1)
		e2e.Logf("ICSP %s created successfully", icspName1)

		g.DeferCleanup(func() {
			g.By("Cleanup: Delete any remaining test resources")
			wSpec := getWorkerMCPSpec(oc)
			toDelete := false

			for _, name := range []string{icspName1, icspName2} {
				if _, getErr := operatorClient.ImageContentSourcePolicies().Get(context.Background(), name, metav1.GetOptions{}); getErr == nil {
					if delErr := operatorClient.ImageContentSourcePolicies().Delete(context.Background(), name, metav1.DeleteOptions{}); delErr == nil {
						e2e.Logf("Cleanup: deleted ICSP %s", name)
						toDelete = true
					} else {
						e2e.Logf("Warning: failed to delete ICSP %s: %v", name, delErr)
					}
				}
			}
			if _, getErr := configClient.ImageTagMirrorSets().Get(context.Background(), itmsName, metav1.GetOptions{}); getErr == nil {
				if delErr := configClient.ImageTagMirrorSets().Delete(context.Background(), itmsName, metav1.DeleteOptions{}); delErr == nil {
					e2e.Logf("Cleanup: deleted ITMS %s", itmsName)
					toDelete = true
				} else {
					e2e.Logf("Warning: failed to delete ITMS %s: %v", itmsName, delErr)
				}
			}
			if _, getErr := configClient.ImageDigestMirrorSets().Get(context.Background(), idmsName, metav1.GetOptions{}); getErr == nil {
				if delErr := configClient.ImageDigestMirrorSets().Delete(context.Background(), idmsName, metav1.DeleteOptions{}); delErr == nil {
					e2e.Logf("Cleanup: deleted IDMS %s", idmsName)
					toDelete = true
				} else {
					e2e.Logf("Warning: failed to delete IDMS %s: %v", idmsName, delErr)
				}
			}
			if toDelete {
				waitForWorkerRollout(oc, wSpec)
			}
		})

		g.By("Step 2: Wait for worker MCP rollout after ICSP creation and verify registries.conf")
		waitForWorkerRollout(oc, workerSpec)
		e2e.Logf("Worker MCP rollout complete after ICSP creation")

		registriesConf := readRegistriesConfOnWorker(ctx, oc)
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
		workerSpecBeforeIDMS, masterSpecBeforeIDMS := getMCPSpecs(oc)
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
		o.Expect(pollMCPSpecUnchanged(oc, "worker", workerSpecBeforeIDMS, 2*time.Minute)).
			NotTo(o.HaveOccurred(), "unexpected worker MCP rollout after IDMS creation with same config as ICSP")
		o.Expect(pollMCPSpecUnchanged(oc, "master", masterSpecBeforeIDMS, 2*time.Minute)).
			NotTo(o.HaveOccurred(), "unexpected master MCP rollout after IDMS creation with same config as ICSP")
		e2e.Logf("Confirmed: both MCPs stable for 2 minutes after IDMS creation")

		g.By("Step 4.1: Delete ICSP - IDMS covers the same config so no new MC should be triggered")
		workerSpecBeforeICSPDel, masterSpecBeforeICSPDel := getMCPSpecs(oc)
		err = operatorClient.ImageContentSourcePolicies().Delete(ctx, icspName1, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to delete ICSP %s", icspName1)
		e2e.Logf("ICSP %s deleted successfully", icspName1)

		g.By("Step 4.2: Confirm no new MC was generated after ICSP deletion (IDMS still covers the same config)")
		o.Expect(pollMCPSpecUnchanged(oc, "worker", workerSpecBeforeICSPDel, 2*time.Minute)).
			NotTo(o.HaveOccurred(), "unexpected worker MCP rollout after ICSP deletion when IDMS covers the same config")
		o.Expect(pollMCPSpecUnchanged(oc, "master", masterSpecBeforeICSPDel, 2*time.Minute)).
			NotTo(o.HaveOccurred(), "unexpected master MCP rollout after ICSP deletion when IDMS covers the same config")
		e2e.Logf("Confirmed: both MCPs stable for 2 minutes after ICSP deletion")

		g.By("Step 5: Verify registries.conf is unchanged after ICSP deletion (IDMS maintains same mirror config)")
		registriesConfAfterICSPDelete := readRegistriesConfOnWorker(ctx, oc)
		o.Expect(registriesConfAfterICSPDelete).To(o.Equal(registriesConf),
			"registries.conf should be unchanged after ICSP deletion when IDMS covers the same mirror config")
		e2e.Logf("Confirmed: registries.conf unchanged after ICSP deletion")

		g.By("Step 6: Create ITMS with tag mirrors for ubi9/ubi-minimal (different source from IDMS)")
		workerSpec = getWorkerMCPSpec(oc)
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

		waitForWorkerRollout(oc, workerSpec)
		e2e.Logf("Worker MCP rollout complete after ITMS creation")

		g.By("Step 7: Verify registries.conf updated with ITMS tag-only entries alongside IDMS digest entries")
		registriesConfAfterITMS := readRegistriesConfOnWorker(ctx, oc)
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
		workerSpec = getWorkerMCPSpec(oc)
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

		waitForWorkerRollout(oc, workerSpec)
		e2e.Logf("Worker MCP rollout complete after second ICSP creation")

		g.By("Step 9: Verify registries.conf updated with ICSP2 entries alongside IDMS and ITMS entries")
		registriesConfAfterICSP2 := readRegistriesConfOnWorker(ctx, oc)
		e2e.Logf("registries.conf after second ICSP creation: read %d bytes, asserting expected entries", len(registriesConfAfterICSP2))

		o.Expect(registriesConfAfterICSP2).To(o.ContainSubstring(`location = "registry.example.com/example/myimage"`),
			"registries.conf should contain the ICSP2 source")
		o.Expect(registriesConfAfterICSP2).To(o.ContainSubstring(`location = "mirror.example.net/image"`),
			"registries.conf should contain the ICSP2 mirror")
		o.Expect(registriesConfAfterICSP2).To(o.ContainSubstring(`location = "registry.access.redhat.com/ubi8/ubi-minimal"`),
			"registries.conf should still contain IDMS entries for ubi8/ubi-minimal")
		o.Expect(registriesConfAfterICSP2).To(o.ContainSubstring(`location = "registry.access.redhat.com/ubi9/ubi-minimal"`),
			"registries.conf should still contain ITMS entries for ubi9/ubi-minimal")

		g.By("Step 10: Delete IDMS and wait for worker MCP rollout")
		workerSpec = getWorkerMCPSpec(oc)
		err = configClient.ImageDigestMirrorSets().Delete(ctx, idmsName, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to delete IDMS %s", idmsName)
		e2e.Logf("IDMS %s deleted successfully", idmsName)

		waitForWorkerRollout(oc, workerSpec)
		e2e.Logf("Worker MCP rollout complete after IDMS deletion")

		g.By("Step 11: Verify registries.conf - IDMS entries removed, ITMS and ICSP2 entries remain")
		registriesConfAfterIDMSDelete := readRegistriesConfOnWorker(ctx, oc)
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

package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"github.com/openshift/origin/test/extended/imagepolicy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	nodeutils "github.com/openshift/origin/test/extended/node"
	exutil "github.com/openshift/origin/test/extended/util"
	operator "github.com/openshift/origin/test/extended/util/operator"
)

var _ = g.Describe("[Suite:openshift/disruptive-longrunning][sig-node][Disruptive] Image registry config", func() {
	var (
		oc = exutil.NewCLIWithoutNamespace("imgcfg")
	)

	g.BeforeEach(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to detect cluster type")
		if isMicroShift {
			g.Skip("Skipping test on MicroShift cluster - MachineConfig resources are not available")
		}
	})

	// Verifies that updating image.config.openshift.io/cluster with a new search
	// registry triggers an MCO rollout and the change lands on nodes.
	//author: cmaurya@redhat.com
	g.It("[OTP] change container registry config [OCP-44820]", func() {
		ctx := context.Background()
		searchRegistry := "qe.quay.io"

		g.By("Save the original image.config for later restore")
		originalImageConfig, err := oc.AdminConfigClient().ConfigV1().Images().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get image.config.openshift.io/cluster")

		initialWorkerSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "worker")
		initialMasterSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "master")

		g.DeferCleanup(func() {
			e2e.Logf("Cleanup: restoring original image.config")
			restoreErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				current, getErr := oc.AdminConfigClient().ConfigV1().Images().Get(ctx, "cluster", metav1.GetOptions{})
				if getErr != nil {
					return getErr
				}
				current.Spec.RegistrySources = originalImageConfig.Spec.RegistrySources
				_, updateErr := oc.AdminConfigClient().ConfigV1().Images().Update(ctx, current, metav1.UpdateOptions{})
				return updateErr
			})
			o.Expect(restoreErr).NotTo(o.HaveOccurred(),
				"cleanup failed: could not restore original image.config")

			cleanupWorkerSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "worker")
			cleanupMasterSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "master")
			imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "worker", cleanupWorkerSpec)
			imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "master", cleanupMasterSpec)

			e2e.Logf("Cleanup: waiting for all cluster operators to settle")
			waitErr := operator.WaitForOperatorsToSettle(ctx, oc.AdminConfigClient(), 10)
			o.Expect(waitErr).NotTo(o.HaveOccurred(),
				"cluster operators did not settle after restore")
		})

		g.By("Update image.config to add search registry and allowed registries")
		err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			imageConfig, getErr := oc.AdminConfigClient().ConfigV1().Images().Get(ctx, "cluster", metav1.GetOptions{})
			if getErr != nil {
				return getErr
			}
			imageConfig.Spec.RegistrySources.AllowedRegistries = []string{
				"registry.access.redhat.com", "docker.io", "quay.io", searchRegistry,
				"image-registry.openshift-image-registry.svc:5000", "quay-proxy.ci.openshift.org",
			}
			imageConfig.Spec.RegistrySources.ContainerRuntimeSearchRegistries = []string{
				"registry.access.redhat.com", "docker.io", "quay.io", searchRegistry,
			}
			_, updateErr := oc.AdminConfigClient().ConfigV1().Images().Update(ctx, imageConfig, metav1.UpdateOptions{})
			return updateErr
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to update image.config.openshift.io/cluster")

		g.By("Wait for worker and master MCP rollout to complete")
		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "worker", initialWorkerSpec)
		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "master", initialMasterSpec)

		g.By("Verify search registries config on a worker node")
		workers, err := exutil.GetReadySchedulableWorkerNodes(ctx, oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get ready schedulable worker nodes")
		o.Expect(workers).NotTo(o.BeEmpty(), "no ready worker nodes found")

		var registriesConf string
		o.Eventually(func() error {
			var execErr error
			registriesConf, execErr = nodeutils.ExecOnNodeWithChroot(oc, workers[0].Name,
				"cat", "/etc/containers/registries.conf.d/01-image-searchRegistries.conf")
			if execErr != nil {
				return execErr
			}
			if !strings.Contains(registriesConf, searchRegistry) {
				return fmt.Errorf("search registry %s not yet in config", searchRegistry)
			}
			return nil
		}, 30*time.Second, 5*time.Second).Should(o.Succeed(),
			"search registry %s not found in registries config on node %s", searchRegistry, workers[0].Name)
		e2e.Logf("Registries config on %s:\n%s", workers[0].Name, registriesConf)

		g.By("Verify policy.json is updated with allowed registries")
		policyJSON, err := nodeutils.ExecOnNodeWithChroot(oc, workers[0].Name,
			"cat", "/etc/containers/policy.json")
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to read policy.json on node %s", workers[0].Name)
		e2e.Logf("policy.json on %s:\n%s", workers[0].Name, policyJSON)
		o.Expect(policyJSON).To(o.ContainSubstring(searchRegistry),
			"policy.json should contain allowed registry %s", searchRegistry)
	})
})

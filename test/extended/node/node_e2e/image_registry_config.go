package node

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	nodeutils "github.com/openshift/origin/test/extended/node"
	exutil "github.com/openshift/origin/test/extended/util"
	operator "github.com/openshift/origin/test/extended/util/operator"
)

const (
	imageRegistryConfigLabel           = "image_registry_config"
	nodeResourceRegistryRolloutTimeout = 30 * time.Minute
)

var _ = g.Describe("[Suite:openshift/disruptive-longrunning][sig-node][Disruptive][NodeResource:numNodes=all,label=image_registry_config] Image registry config", func() {
	var (
		oc = exutil.NewCLIWithoutNamespace("imgcfg")
	)

	g.BeforeEach(func(ctx context.Context) {
		nodeutils.SkipOnMicroShift(oc)
		nodeutils.EnsureNodeResourceNodesReady(ctx, oc, imageRegistryConfigLabel)
	})

	// Verifies that updating image.config.openshift.io/cluster with a new search
	// registry triggers an MCO rollout and the change lands on nodes.
	//author: cmaurya@redhat.com
	g.It("[OTP] change container registry config [OCP-44820]", func() {
		ctx := context.Background()
		searchRegistry := "qe.quay.io"

		g.By("Pause master pool to avoid master drains during test")
		pauseMasterPool(oc)
		g.DeferCleanup(func() { unpauseMasterPool(oc) })

		g.By("Save the original image.config for later restore")
		originalImageConfig, err := oc.AdminConfigClient().ConfigV1().Images().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get image.config.openshift.io/cluster")

		g.DeferCleanup(func() {
			cleanupCtx := context.Background()
			e2e.Logf("Cleanup: restoring original image.config")
			restoreErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				current, getErr := oc.AdminConfigClient().ConfigV1().Images().Get(cleanupCtx, "cluster", metav1.GetOptions{})
				if getErr != nil {
					return getErr
				}
				current.Spec.RegistrySources = originalImageConfig.Spec.RegistrySources
				_, updateErr := oc.AdminConfigClient().ConfigV1().Images().Update(cleanupCtx, current, metav1.UpdateOptions{})
				return updateErr
			})
			o.Expect(restoreErr).NotTo(o.HaveOccurred(),
				"cleanup failed: could not restore original image.config")

			nodeutils.WaitForNodeResourceFileNotContains(cleanupCtx, oc, imageRegistryConfigLabel,
				"/etc/containers/registries.conf.d/01-image-searchRegistries.conf", searchRegistry, nodeResourceRegistryRolloutTimeout)
			nodeutils.WaitForNodeResourceFileNotContains(cleanupCtx, oc, imageRegistryConfigLabel,
				"/etc/containers/policy.json", searchRegistry, nodeResourceRegistryRolloutTimeout)

			e2e.Logf("Cleanup: waiting for all cluster operators to settle")
			waitErr := operator.WaitForOperatorsToSettle(cleanupCtx, oc.AdminConfigClient(), 10)
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
				"image-registry.openshift-image-registry.svc:5000", "quay-proxy.ci.openshift.org", "registry.redhat.io",
			}
			imageConfig.Spec.RegistrySources.ContainerRuntimeSearchRegistries = []string{
				"registry.access.redhat.com", "docker.io", "quay.io", searchRegistry,
			}
			_, updateErr := oc.AdminConfigClient().ConfigV1().Images().Update(ctx, imageConfig, metav1.UpdateOptions{})
			return updateErr
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to update image.config.openshift.io/cluster")

		g.By("Wait for search registry config on NodeResource pool nodes")
		nodeutils.WaitForNodeResourceFileContains(ctx, oc, imageRegistryConfigLabel,
			"/etc/containers/registries.conf.d/01-image-searchRegistries.conf", []string{searchRegistry}, nodeResourceRegistryRolloutTimeout)
		nodeutils.WaitForNodeResourceFileContains(ctx, oc, imageRegistryConfigLabel,
			"/etc/containers/policy.json", []string{searchRegistry}, nodeResourceRegistryRolloutTimeout)

		g.By("Verify search registries config on a worker node")
		workerNodeName, nodeErr := nodeutils.GetNodeResource(ctx, oc, imageRegistryConfigLabel)
		o.Expect(nodeErr).NotTo(o.HaveOccurred(), "no ready worker node found")

		registriesConf, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, workerNodeName,
			"cat", "/etc/containers/registries.conf.d/01-image-searchRegistries.conf")
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to read search registries config on node %s", workerNodeName)
		e2e.Logf("Registries config on %s:\n%s", workerNodeName, registriesConf)
		o.Expect(registriesConf).To(o.ContainSubstring(searchRegistry),
			"search registry %s not found in registries config on node %s", searchRegistry, workerNodeName)

		g.By("Verify policy.json is updated with allowed registries")
		policyJSON, err := nodeutils.ExecOnNodeWithChroot(ctx, oc, workerNodeName,
			"cat", "/etc/containers/policy.json")
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to read policy.json on node %s", workerNodeName)
		e2e.Logf("policy.json on %s:\n%s", workerNodeName, policyJSON)
		o.Expect(policyJSON).To(o.ContainSubstring(searchRegistry),
			"policy.json should contain allowed registry %s", searchRegistry)
	})
})

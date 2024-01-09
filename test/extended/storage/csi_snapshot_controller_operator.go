package storage

import (
	"reflect"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	clusterCSISnapshotOperatorNs = "openshift-cluster-storage-operator"
	snapshotWebhookSecretName    = "csi-snapshot-webhook-secret"
	snapshotWebhookDeployName    = "csi-snapshot-webhook"
)

// This is [Serial] because it deletes the csi-snapshot-webhook-secret
var _ = g.Describe("[sig-storage][Feature:Cluster-CSI-Snapshot-Controller-Operator][Serial][apigroup:operator.openshift.io]", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLIWithoutNamespace("storage-csi-snapshot-operator")

	g.BeforeEach(func() {
		// Skip if CSISnapshot CO is not enabled
		if CSISnapshotEnabled, _ := exutil.IsCapabilityEnabled(oc, configv1.ClusterVersionCapabilityCSISnapshot); !CSISnapshotEnabled {
			g.Skip("Skip for CSISnapshot capability is not enabled on the test cluster!")
		}
	})

	g.AfterEach(func() {
		WaitForCSOHealthy(oc)
	})

	g.It("should restart webhook Pods if csi-snapshot-webhook-secret expiry annotation is changed", func() {

		g.By("# Get the csiSnapshotWebhook annotations")
		csiSnapshotWebhookAnnotationsOri := exutil.GetDeploymentTemplateAnnotations(oc, snapshotWebhookDeployName, clusterCSISnapshotOperatorNs)

		g.By("# Modify the csi-snapshot-webhook-secret expiry annotation")
		defer func() {
			if exutil.WaitForDeploymentReady(oc, snapshotWebhookDeployName, clusterCSISnapshotOperatorNs) != nil {
				e2e.Failf("The csiSnapshotWebhook was not recovered ready")
			}
		}()
		o.Expect(oc.AsAdmin().Run("annotate").Args("-n", clusterCSISnapshotOperatorNs, "secret", snapshotWebhookSecretName,
			"service.alpha.openshift.io/expiry-", "service.beta.openshift.io/expiry-").Execute()).NotTo(o.HaveOccurred())

		g.By("# Check the webhook Pods were restarted")
		o.Eventually(func() bool {
			csiSnapshotWebhookAnnotationsCurrent := exutil.GetDeploymentTemplateAnnotations(oc, snapshotWebhookDeployName, clusterCSISnapshotOperatorNs)
			return reflect.DeepEqual(csiSnapshotWebhookAnnotationsOri, csiSnapshotWebhookAnnotationsCurrent)
		}).WithTimeout(defaultMaxWaitingTime).WithPolling(defaultPollingTime).Should(o.BeFalse(), "The csiSnapshotWebhook was not updated")
	})

	g.It("should restart webhook Pods if csi-snapshot-webhook-secret is deleted", func() {

		g.By("# Get the csiSnapshotWebhook annotations")
		csiSnapshotWebhookAnnotationsOri := exutil.GetDeploymentTemplateAnnotations(oc, snapshotWebhookDeployName, clusterCSISnapshotOperatorNs)

		g.By("# Delete the csi-snapshot-webhook-secret")
		defer func() {
			if exutil.WaitForDeploymentReady(oc, snapshotWebhookDeployName, clusterCSISnapshotOperatorNs) != nil {
				e2e.Failf("The csiSnapshotWebhook was not recovered ready")
			}
		}()
		o.Expect(oc.AsAdmin().Run("delete").Args("-n", clusterCSISnapshotOperatorNs, "secret", snapshotWebhookSecretName).Execute()).NotTo(o.HaveOccurred())

		g.By("# Check the webhook Pods were restarted")
		o.Eventually(func() bool {
			csiSnapshotWebhookAnnotationsCurrent := exutil.GetDeploymentTemplateAnnotations(oc, snapshotWebhookDeployName, clusterCSISnapshotOperatorNs)
			return reflect.DeepEqual(csiSnapshotWebhookAnnotationsOri, csiSnapshotWebhookAnnotationsCurrent)
		}).WithTimeout(defaultMaxWaitingTime).WithPolling(defaultPollingTime).Should(o.BeFalse(), "The csiSnapshotWebhook was not updated")
	})
})

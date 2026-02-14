package storage

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe(`[sig-storage][Feature:BYOK][Jira:"Storage"]`, func() {
	defer g.GinkgoRecover()

	var (
		oc                                 = exutil.NewCLIWithPodSecurityLevel("csi-byok", admissionapi.LevelPrivileged)
		cloudProvider                      = ""
		featureProviderSupportProvisioners = []string{}
	)

	g.BeforeEach(func() {
		// Detect cloud provider and set supported provisioners
		cloudProvider = e2e.TestContext.Provider
		featureProviderSupportProvisioners = GetBYOKProvisionerNames(cloudProvider)

		if len(featureProviderSupportProvisioners) == 0 {
			g.Skip("Skip for scenario non-supported provisioner!!!")
		}

	})

	g.It("managed storage classes should be set with the specified encryption key", func() {

		for _, provisioner := range featureProviderSupportProvisioners {

			// Get provisioner info from const
			provisionerInfo := GetProvisionerByName(provisioner)
			if provisionerInfo == nil {
				g.Skip("Provisioner not found in configuration: " + provisioner)
			}

			// Skipped for test clusters not installed with the BYOK
			byokKeyID := getByokKeyIDFromClusterCSIDriver(oc, provisioner)
			if len(byokKeyID) == 0 {
				g.Skip("Skipped: the cluster is not byok cluster, no key settings in clustercsidriver/" + provisioner)
			}

			g.By("Get preset storageClass names from configuration")
			presetStorageClassNames := getPresetStorageClassNamesByProvisioner(provisioner)
			if len(presetStorageClassNames) == 0 {
				g.Skip(fmt.Sprintf("No preset storage classes configured for provisioner %s. Expected one of: %v",
					provisioner, provisionerInfo.ManagedStorageClassNames))
			}

			g.By("Verify all preset storage classes have encryption key configured")
			for _, scName := range presetStorageClassNames {
				g.By(fmt.Sprintf("Verifying storage class: %s", scName))

				sc, err := oc.AdminKubeClient().StorageV1().StorageClasses().Get(context.Background(), scName, metav1.GetOptions{})
				if err != nil {
					g.Skip(fmt.Sprintf("Storage class %s not found in cluster: %v", scName, err))
				}

				// Double check the storage class is properly configured
				o.Expect(sc.Provisioner).Should(o.Equal(provisioner),
					fmt.Sprintf("Storage class %s provisioner mismatch", sc.Name))

				// Verify BYOK key parameter exists
				storedKeyID, exists := sc.Parameters[provisionerInfo.EncryptionKeyName]
				if !exists {
					g.Fail(fmt.Sprintf("Storage class %s does not have BYOK key parameter %s configured",
						sc.Name, provisionerInfo.EncryptionKeyName))
				}
				o.Expect(storedKeyID).Should(o.Equal(byokKeyID),
					fmt.Sprintf("Storage class %s has different BYOK key than ClusterCSIDriver", sc.Name))
			}
		}
	})
})

func getByokKeyIDFromClusterCSIDriver(oc *exutil.CLI, provisioner string) string {
	clusterCSIDriver, err := oc.AdminOperatorClient().OperatorV1().ClusterCSIDrivers().Get(context.Background(), provisioner, metav1.GetOptions{})
	if err != nil {
		e2e.Logf("Failed to get ClusterCSIDriver %s: %v", provisioner, err)
		return ""
	}

	driverConfig := clusterCSIDriver.Spec.DriverConfig
	if driverConfig.DriverType == "" {
		return ""
	}

	// Extract key ID based on driver type using struct fields
	switch driverConfig.DriverType {
	case "AWS":
		if driverConfig.AWS != nil {
			return driverConfig.AWS.KMSKeyARN
		}
	case "Azure":
		if driverConfig.Azure != nil && driverConfig.Azure.DiskEncryptionSet != nil {
			// Build the full disk encryption set ID
			des := driverConfig.Azure.DiskEncryptionSet
			return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/diskEncryptionSets/%s",
				des.SubscriptionID, des.ResourceGroup, des.Name)
		}
	case "GCP":
		if driverConfig.GCP != nil && driverConfig.GCP.KMSKey != nil {
			// For GCP, return the full KMS key reference or just the key ring based on what's needed
			kmsKey := driverConfig.GCP.KMSKey
			location := kmsKey.Location
			if location == "" {
				location = "global"
			}
			return fmt.Sprintf("projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s",
				kmsKey.ProjectID, location, kmsKey.KeyRing, kmsKey.Name)
		}
	case "IBMCloud":
		if driverConfig.IBMCloud != nil {
			return driverConfig.IBMCloud.EncryptionKeyCRN
		}
	}

	return ""
}

func getPresetStorageClassNamesByProvisioner(provisioner string) []string {
	// Get provisioner info to get managed storage class names from static config
	provisionerInfo := GetProvisionerByName(provisioner)
	if provisionerInfo == nil {
		e2e.Logf("Provisioner not found in configuration: %s", provisioner)
		return []string{}
	}
	return provisionerInfo.ManagedStorageClassNames
}

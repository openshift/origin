package storage

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	imageregistry "github.com/openshift/client-go/imageregistry/clientset/versioned"
	clusteroperatorhelpers "github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	exutil "github.com/openshift/origin/test/extended/util"
)

// Define test waiting time const
const (
	defaultMaxWaitingTime = 200 * time.Second
	defaultPollingTime    = 2 * time.Second
)

// IsCSOHealthy checks whether the Cluster Storage Operator is healthy
func IsCSOHealthy(oc *exutil.CLI) (bool, error) {
	// CSO healthyStatus:[degradedStatus:False, progressingStatus:False, availableStatus:True, upgradeableStatus:True]
	clusterStorageOperator, getOperatorErr := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(context.Background(), "storage", metav1.GetOptions{})
	if getOperatorErr != nil {
		e2e.Logf("Error getting storage operator: %v", getOperatorErr)
		return false, getOperatorErr
	}
	return clusteroperatorhelpers.IsStatusConditionTrue(clusterStorageOperator.Status.Conditions, configv1.OperatorAvailable) &&
		clusteroperatorhelpers.IsStatusConditionTrue(clusterStorageOperator.Status.Conditions, configv1.OperatorUpgradeable) &&
		clusteroperatorhelpers.IsStatusConditionFalse(clusterStorageOperator.Status.Conditions, configv1.OperatorDegraded) &&
		clusteroperatorhelpers.IsStatusConditionFalse(clusterStorageOperator.Status.Conditions, configv1.OperatorProgressing), nil
}

// WaitForCSOHealthy waits for Cluster Storage Operator become healthy
func WaitForCSOHealthy(oc *exutil.CLI) {
	o.Eventually(func() bool {
		IsCSOHealthy, getCSOHealthyErr := IsCSOHealthy(oc)
		if getCSOHealthyErr != nil {
			e2e.Logf(`Get CSO status failed of: "%v", try again`, getCSOHealthyErr)
		}
		return IsCSOHealthy
	}).WithTimeout(defaultMaxWaitingTime).WithPolling(defaultPollingTime).Should(o.BeTrue(), "Waiting for CSO become healthy timeout")
}

func skipIfNotS3Storage(oc *exutil.CLI) {
	g.By("checking storage type")

	imageRegistryConfigClient, err := imageregistry.NewForConfig(oc.AdminConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	imageRegistryConfig, err := imageRegistryConfigClient.ImageregistryV1().Configs().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	if imageRegistryConfig.Spec.Storage.S3 == nil {
		e2eskipper.Skipf("No S3 storage detected")
	}
}

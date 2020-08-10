package operators

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo"
	configv1 "github.com/openshift/api/config/v1"
	configclientv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

func NewClusterOperatorUpgradeTests() []upgrades.Test {
	ret := []upgrades.Test{}

	for name, owner := range clusterOperators {
		ret = append(ret, &ClusterOperatorUpgradeTest{ClusterOperatorName: name, Owner: owner, tornDown: make(chan struct{})})
	}

	return ret
}

// clusterOperators to their owners if they need to be overridden. Not scientific, just dumped `oc get co -oname`.
var clusterOperators = map[string]string{
	"authentication":                     "",
	"cloud-credential":                   "",
	"cluster-autoscaler":                 "",
	"config-operator":                    "",
	"console":                            "",
	"csi-snapshot-controller":            "",
	"dns":                                "",
	"etcd":                               "",
	"image-registry":                     "",
	"ingress":                            "",
	"insights":                           "",
	"kube-apiserver":                     "",
	"kube-controller-manager":            "",
	"kube-scheduler":                     "",
	"kube-storage-version-migrator":      "",
	"machine-api":                        "",
	"machine-approver":                   "",
	"machine-config":                     "",
	"marketplace":                        "",
	"monitoring":                         "",
	"network":                            "",
	"node-tuning":                        "",
	"openshift-apiserver":                "",
	"openshift-controller-manager":       "",
	"openshift-samples":                  "",
	"operator-lifecycle-manager":         "",
	"operator-lifecycle-manager-catalog": "",
	"operator-lifecycle-manager-packageserver": "",
	"service-ca": "",
	"storage":    "",
}

type ClusterOperatorUpgradeTest struct {
	ClusterOperatorName string
	Owner               string
	tornDown            chan struct{}
}

// Name returns the tracking name of the test.
func (t *ClusterOperatorUpgradeTest) Name() string {
	if len(t.Owner) > 0 {
		return "[" + t.Owner + "] upgrade"
	}
	return "[sig-" + t.ClusterOperatorName + "] upgrade"
}

func (t *ClusterOperatorUpgradeTest) Setup(f *framework.Framework) {
}

func (t *ClusterOperatorUpgradeTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	ginkgo.By(fmt.Sprintf("Waiting for upgrade to finish for clusteroperator/%s", t.ClusterOperatorName))

	configClient, err := configclientv1.NewForConfig(f.ClientConfig())
	framework.ExpectNoError(err)

	operatorUpgraded := false
	wait.PollImmediateUntil(10*time.Second, func() (bool, error) {
		ctx := context.TODO()
		clusterOperator, err := configClient.ClusterOperators().Get(ctx, t.ClusterOperatorName, metav1.GetOptions{})
		if err != nil {
			// log
			return false, nil
		}
		if !isClusterOperatorStatusConditionTrue(clusterOperator.Status.Conditions, string(configv1.OperatorAvailable)) {
			return false, nil
		}
		if !isClusterOperatorStatusConditionFalse(clusterOperator.Status.Conditions, string(configv1.OperatorDegraded)) {
			return false, nil
		}
		if !isClusterOperatorStatusConditionFalse(clusterOperator.Status.Conditions, string(configv1.OperatorProgressing)) {
			return false, nil
		}

		operatorUpgraded = true
		return true, nil
	}, t.tornDown)

	if !operatorUpgraded {
		framework.ExpectNoError(fmt.Errorf("clusteroperator/%s did not upgrade", t.ClusterOperatorName))
	}
}

// Teardown cleans up any remaining resources.
func (t *ClusterOperatorUpgradeTest) Teardown(f *framework.Framework) {
	close(t.tornDown)
}

func isClusterOperatorStatusConditionTrue(conditions []configv1.ClusterOperatorStatusCondition, conditionType string) bool {
	return isClusterOperatorStatusConditionPresentAndEqual(conditions, conditionType, configv1.ConditionTrue)
}

func isClusterOperatorStatusConditionFalse(conditions []configv1.ClusterOperatorStatusCondition, conditionType string) bool {
	return isClusterOperatorStatusConditionPresentAndEqual(conditions, conditionType, configv1.ConditionFalse)
}

func isClusterOperatorStatusConditionPresentAndEqual(conditions []configv1.ClusterOperatorStatusCondition, conditionType string, status configv1.ConditionStatus) bool {
	for _, condition := range conditions {
		if string(condition.Type) == conditionType {
			return condition.Status == status
		}
	}
	return false
}

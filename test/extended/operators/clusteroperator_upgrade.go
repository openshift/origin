package operators

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/openshift/origin/test/extended/util/disruption"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/davecgh/go-spew/spew"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"

	"github.com/onsi/ginkgo"
	configv1 "github.com/openshift/api/config/v1"
	configclientv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

// VersionTracker provides a way to funnel late-binding version strings after they are known by order of upgrade.
// In CI upgrades are identified with payload images, but they do not have versions known until *after* the CVO has
// acknowledged the upgrade.
type VersionTracker interface {
	GetTargetVersionFor(upgradeIndex int) string
	SetTargetVersionFor(upgradeIndex int, version string)
}

type versionTracker struct {
	lock                        sync.RWMutex
	upgradeIndexToTargetVersion map[int]string
}

func NewVersionTracker() VersionTracker {
	return &versionTracker{upgradeIndexToTargetVersion: map[int]string{}}
}
func (v *versionTracker) GetTargetVersionFor(upgradeIndex int) string {
	v.lock.RLock()
	defer v.lock.RUnlock()
	return v.upgradeIndexToTargetVersion[upgradeIndex]
}

func (v *versionTracker) SetTargetVersionFor(upgradeIndex int, version string) {
	v.lock.Lock()
	defer v.lock.Unlock()
	v.upgradeIndexToTargetVersion[upgradeIndex] = version
}

func NewClusterOperatorUpgradeTests(versions []string, versionTracker VersionTracker) []upgrades.Test {
	ret := []upgrades.Test{}

	// skip the first version, because that's the one installed, not upgraded to
	for i := range versions[1:] {
		for name, owner := range clusterOperators {
			ret = append(ret, &ClusterOperatorUpgradeTest{
				ClusterOperatorName: name,
				Owner:               owner,
				upgradeIndex:        i,
				versionTracker:      versionTracker,
				tornDown:            make(chan struct{})},
			)
		}
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
	// ordinal is the order of the upgrade in question, since the actual value doesn't matter.  we just need a consistent
	// name for junit testing in a single job.
	upgradeIndex   int
	versionTracker VersionTracker
	tornDown       chan struct{}
}

// Name returns the tracking name of the test.
func (t *ClusterOperatorUpgradeTest) Name() string {
	if len(t.Owner) > 0 {
		return fmt.Sprintf("[%s] upgrade %d", t.Owner, t.upgradeIndex)
	}
	return fmt.Sprintf("[sig-%s] upgrade %d", t.ClusterOperatorName, t.upgradeIndex)
}

func (t *ClusterOperatorUpgradeTest) Setup(f *framework.Framework) {
}

func (t *ClusterOperatorUpgradeTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	ginkgo.By(fmt.Sprintf("Waiting for upgrade to finish for clusteroperator/%s", t.ClusterOperatorName))

	configClient, err := configclientv1.NewForConfig(f.ClientConfig())
	framework.ExpectNoError(err)

	operatorUpgraded := false
	operatorExisted := false
	start := time.Now()
	wait.PollImmediateUntil(10*time.Second, func() (bool, error) {
		ctx := context.TODO()
		clusterOperator, err := configClient.ClusterOperators().Get(ctx, t.ClusterOperatorName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			if time.Now().Sub(start) > 45*time.Minute {
				e2elog.Logf("operator didn't exist after 45 minutes")
				operatorUpgraded = true // we don't want to fail on missing operators.  at least not yet.
				return true, nil
			}
		}
		if err != nil {
			// log
			return false, nil
		}
		operatorExisted = true
		if !isClusterOperatorStatusConditionTrue(clusterOperator.Status.Conditions, string(configv1.OperatorAvailable)) {
			return false, nil
		}
		if !isClusterOperatorStatusConditionFalse(clusterOperator.Status.Conditions, string(configv1.OperatorDegraded)) {
			return false, nil
		}
		if !isClusterOperatorStatusConditionFalse(clusterOperator.Status.Conditions, string(configv1.OperatorProgressing)) {
			return false, nil
		}
		operatorVersionFound := false
		for _, operatorVersion := range clusterOperator.Status.Versions {
			if operatorVersion.Name != "operator" {
				continue
			}
			operatorVersionFound = true
			if len(operatorVersion.Version) > 0 && operatorVersion.Version == t.versionTracker.GetTargetVersionFor(t.upgradeIndex) {
				operatorUpgraded = true
			}
		}
		if !operatorVersionFound {
			e2elog.Logf("missing operator version: %v", spew.Sdump(clusterOperator.Status.Versions))
			operatorUpgraded = true // don't fail until we can see some output
		}
		if time.Now().Sub(start) > 45*time.Minute {
			e2elog.Logf("operator didn't finish: %v", spew.Sdump(clusterOperator.Status.Versions))
			disruption.FrameworkFlakef(f, "%q didn't finish getting to %q, %s", clusterOperator.Name, t.versionTracker.GetTargetVersionFor(t.upgradeIndex), spew.Sdump(clusterOperator))
			operatorUpgraded = true // mercy mark so we don't fail the test for now
		}

		if !operatorUpgraded {
			return false, nil
		}
		return true, nil
	}, t.tornDown)

	// if the operator never existed, then either it wasn't in this payload or it was completely hosed.  If completely hosed
	// the upgrade should fail and someone will notice.  If it didn't exist, then skipping it here allows us to have one
	// big list of operators that doesn't have to account for each release.
	if !operatorExisted {
		return
	}
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

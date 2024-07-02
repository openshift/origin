// Package clusterversionoperator contains utilities for exercising the cluster-version operator.
package clusterversionoperator

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	restclient "k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

// AdminAckTest contains artifacts used during test
type AdminAckTest struct {
	Oc     *exutil.CLI
	Config *restclient.Config
	Poll   time.Duration
}

const adminAckGateFmt string = "^ack-[4-5][.]([0-9]{1,})-[^-]"

var adminAckGateRegexp = regexp.MustCompile(adminAckGateFmt)

// Test simply returns successfully if admin ack functionality is not part of the baseline being tested. Otherwise,
// for each configured admin ack gate, test verifies the gate name format and that it contains a description. If
// valid and the gate is applicable to the OCP version under test, test checks the value of the admin ack gate.
// If the gate has been ack'ed the test verifies that the Upgradeable condition does not complain about the ack. Test
// then clears the ack and verifies that the Upgradeable condition complains about the ack. Test then sets the ack
// and verifies that the Upgradeable condition no longer complains about the ack.
func (t *AdminAckTest) Test(ctx context.Context) {
	if t.Poll == 0 {
		if err := t.test(ctx, nil, nil); err != nil {
			framework.Fail(err.Error())
		}
		return
	}

	exercisedGates := sets.NewString()
	exercisedVersions := sets.NewString()
	success := false
	var lastError error
	wait.UntilWithContext(ctx, func(ctx context.Context) {
		if err := t.test(ctx, exercisedGates, exercisedVersions); err != nil {
			framework.Logf("Retriable failure to evaluate admin acks: %v", err)
			lastError = err
		} else {
			success = true
		}
	}, t.Poll)

	if !success {
		framework.Failf("Never able to evaluate admin acks.  Most recent failure: %v", lastError)
	}

	// Perform one guaranteed check after the upgrade is complete. The polled check above is cancelled
	// on done signal (so we never know whether the poll was lucky to run at least once since
	// the version was bumped).
	postUpdateCtx, postUpdateCtxCancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer postUpdateCtxCancel()
	if current, err := getCurrentVersion(postUpdateCtx, t.Config); err != nil {
		framework.Fail(err.Error())
	} else if current != "" && !exercisedVersions.Has(current) {
		// We never saw the current version while polling, so lets check it now
		if err := t.test(postUpdateCtx, exercisedGates, exercisedVersions); err != nil {
			framework.Fail(err.Error())
		}
	}
}

func (t *AdminAckTest) test(ctx context.Context, exercisedGates, exercisedVersions sets.String) error {
	gateCm, err := getAdminGatesConfigMap(ctx, t.Oc)
	if err != nil {
		return err
	}
	// Check if this release has admin ack functionality.
	if gateCm == nil || (gateCm != nil && len(gateCm.Data) == 0) {
		framework.Logf("Skipping admin ack test. Admin ack is not in this baseline or contains no gates.")
		return nil
	}
	ackCm, err := getAdminAcksConfigMap(ctx, t.Oc)
	if err != nil {
		return err
	}
	currentVersion, err := getCurrentVersion(ctx, t.Config)
	if err != nil {
		return err
	}

	if exercisedVersions != nil {
		exercisedVersions.Insert(currentVersion)
	}
	for k, v := range gateCm.Data {
		if exercisedGates != nil && exercisedGates.Has(k) {
			continue
		}
		ackVersion := adminAckGateRegexp.FindString(k)
		if ackVersion == "" {
			return fmt.Errorf("configmap openshift-config-managed/admin-gates gate %s has invalid format; must comply with %q.", k, adminAckGateFmt)
		}
		if v == "" {
			return fmt.Errorf("Configmap openshift-config-managed/admin-gates gate %s does not contain description.", k)
		}
		if !gateApplicableToCurrentVersion(ackVersion, currentVersion) {
			framework.Logf("Gate %s not applicable to current version %s", ackVersion, currentVersion)
			continue
		}
		if ackCm.Data[k] == "true" {
			if setFalse, err := upgradeableExplicitlyFalse(ctx, t.Config); err != nil {
				return fmt.Errorf("unable to check Upgradeable condition: %w", err)
			} else if setFalse {
				upgradeableMessage, match, err := adminAckRequiredWithMessage(ctx, t.Config, v)
				if err != nil {
					return fmt.Errorf("gate %s has been ack'ed but unable to determine Upgradeable message: %w", k, err)
				} else if match {
					return fmt.Errorf("gate %s has been ack'ed but Upgradeable is false with reason AdminAckRequired and message %q", k, v)
				}
				framework.Logf("Gate %s has been ack'ed. Upgradeable is "+
					"false but not due to this gate which would set reason AdminAckRequired or MultipleReasons with message %s. %s", k, v, upgradeableMessage)
			}
			// Clear admin ack configmap gate ack
			if err := setAdminGate(ctx, k, "", t.Oc); err != nil {
				return err
			}
		}
		if err := waitForAdminAckRequired(ctx, t.Config, v); err != nil {
			return err
		}
		// Update admin ack configmap with ack
		if err := setAdminGate(ctx, k, "true", t.Oc); err != nil {
			return err
		}
		if err = waitForAdminAckNotRequired(ctx, t.Config, v); err != nil {
			return err
		}
		if exercisedGates != nil {
			exercisedGates.Insert(k)
		}
	}
	framework.Logf("Admin Ack verified")
	return nil
}

// getClusterVersion returns the ClusterVersion object.
func getClusterVersion(ctx context.Context, config *restclient.Config) (*configv1.ClusterVersion, error) {
	c, err := configv1client.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return c.ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
}

// getCurrentVersion determines and returns the cluster's current version by iterating through the
// provided update history until it finds the first version with update State of Completed. If a
// Completed version is not found the version of the oldest history entry, which is the originally
// installed version, is returned. If history is empty the empty string is returned.
func getCurrentVersion(ctx context.Context, config *restclient.Config) (string, error) {
	cv, err := getClusterVersion(ctx, config)
	if err != nil {
		return "", err
	}

	for _, h := range cv.Status.History {
		if h.State == configv1.CompletedUpdate {
			return h.Version, nil
		}
	}
	// Empty history should only occur if method is called early in startup before history is populated.
	if len(cv.Status.History) != 0 {
		return cv.Status.History[len(cv.Status.History)-1].Version, nil
	}
	return "", nil
}

// getEffectiveMinor attempts to do a simple parse of the version provided.  If it does not parse, the value is considered
// an empty string, which works for a comparison for equivalence.
func getEffectiveMinor(version string) string {
	splits := strings.Split(version, ".")
	if len(splits) < 2 {
		return ""
	}
	return splits[1]
}

func gateApplicableToCurrentVersion(gateAckVersion string, currentVersion string) bool {
	parts := strings.Split(gateAckVersion, "-")
	ackMinor := getEffectiveMinor(parts[1])
	cvMinor := getEffectiveMinor(currentVersion)
	if ackMinor == cvMinor {
		return true
	}
	return false
}

func getAdminGatesConfigMap(ctx context.Context, oc *exutil.CLI) (*corev1.ConfigMap, error) {
	cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps("openshift-config-managed").Get(ctx, "admin-gates", metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("Error accessing configmap openshift-config-managed/admin-gates: %w", err)
		} else {
			return nil, nil
		}
	}
	return cm, nil
}

func getAdminAcksConfigMap(ctx context.Context, oc *exutil.CLI) (*corev1.ConfigMap, error) {
	cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps("openshift-config").Get(ctx, "admin-acks", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("Error accessing configmap openshift-config/admin-acks: %w", err)
	}
	return cm, nil
}

// adminAckRequiredWithMessage returns true if Upgradeable condition reason is AdminAckRequired
// or MultipleReasons and message contains given message.
func adminAckRequiredWithMessage(ctx context.Context, config *restclient.Config, message string) (string, bool, error) {
	clusterVersion, err := getClusterVersion(ctx, config)
	if err != nil {
		return "", false, err
	}

	cond := getUpgradeableStatusCondition(clusterVersion.Status.Conditions)
	if cond == nil {
		return "", false, nil
	}
	if (cond.Reason == "AdminAckRequired" || cond.Reason == "MultipleReasons") && strings.Contains(cond.Message, message) {
		return cond.Message, true, nil
	}
	return cond.Message, false, nil
}

// upgradeableExplicitlyFalse returns true if the Upgradeable condition status is set to false.
func upgradeableExplicitlyFalse(ctx context.Context, config *restclient.Config) (bool, error) {
	clusterVersion, err := getClusterVersion(ctx, config)
	if err != nil {
		return false, err
	}

	cond := getUpgradeableStatusCondition(clusterVersion.Status.Conditions)
	if cond != nil && cond.Status == configv1.ConditionFalse {
		return true, nil
	}
	return false, nil
}

// setAdminGate gets the admin ack configmap and then updates it with given gate name and given value.
func setAdminGate(ctx context.Context, gateName string, gateValue string, oc *exutil.CLI) error {
	ackCm, err := getAdminAcksConfigMap(ctx, oc)
	if err != nil {
		return err
	}
	if ackCm.Data == nil {
		ackCm.Data = make(map[string]string)
	}
	ackCm.Data[gateName] = gateValue
	if _, err := oc.AdminKubeClient().CoreV1().ConfigMaps("openshift-config").Update(ctx, ackCm, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("Unable to update configmap openshift-config/admin-acks: %w", err)
	}
	return nil
}

// adminAckDeadline is the upper bound of time for CVO to notice a new adminack
// gate. CVO sync loop duration is nondeterministic 2-4m interval so we set this
// slightly above the worst case.
const adminAckDeadline = 4*time.Minute + 5*time.Second

func waitForAdminAckRequired(ctx context.Context, config *restclient.Config, message string) error {
	framework.Logf("Waiting for Upgradeable to be AdminAckRequired for %q ...", message)
	var lastUpgradeableMessage string
	var lastError error
	if err := wait.PollImmediate(10*time.Second, adminAckDeadline, func() (bool, error) {
		upgradeableMessage, match, err := adminAckRequiredWithMessage(ctx, config, message)
		lastError = err
		if err != nil {
			return false, nil
		}
		lastUpgradeableMessage = upgradeableMessage
		return match, nil
	}); err != nil {
		return fmt.Errorf("Error while waiting for Upgradeable to complain about AdminAckRequired with message %q: %w (last error %w)\n%s", message, err, lastError, lastUpgradeableMessage)
	}
	return nil
}

func waitForAdminAckNotRequired(ctx context.Context, config *restclient.Config, message string) error {
	framework.Logf("Waiting for Upgradeable to not complain about AdminAckRequired for %q ...", message)
	var lastUpgradeableMessage string
	var lastError error
	if err := wait.PollImmediate(10*time.Second, adminAckDeadline, func() (bool, error) {
		upgradeableMessage, match, err := adminAckRequiredWithMessage(ctx, config, message)
		lastError = err
		if err != nil {
			return false, nil
		}
		lastUpgradeableMessage = upgradeableMessage
		return !match, nil
	}); err != nil {
		return fmt.Errorf("Error while waiting for Upgradeable to not complain about AdminAckRequired with message %q: %w (last error %w)\n%s", message, err, lastError, lastUpgradeableMessage)
	}
	return nil
}

func getUpgradeableStatusCondition(conditions []configv1.ClusterOperatorStatusCondition) *configv1.ClusterOperatorStatusCondition {
	for _, condition := range conditions {
		if condition.Type == configv1.OperatorUpgradeable {
			return &condition
		}
	}
	return nil
}

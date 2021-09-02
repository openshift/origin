package util

import (
	"context"
	"fmt"
	"strings"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	restclient "k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
)

// AdminAckTest contains artifacts used during test
type AdminAckTest struct {
	Oc     *CLI
	Config *restclient.Config
}

// Test simply returns successfully if admin ack functionality is not part the baseline being tested. Otherwise,
// test first checks the value of the admin ack gate. If the gate has been ack'ed the test verifies that Upgradeable
// condition is not true for reason AdminAckRequired and then clears the ack. Test then verifies Upgradeable
// condition is true and contains correct reason and correct message. It then modifies the admin-acks configmap to
// ack the necessary admin-ack gate and waits for the Upgradeable condition to change to true.
func (t *AdminAckTest) Test(ctx context.Context) {

	// Check if this release has admin ack functionality.
	gateCm, errMsg := getAdminGatesConfigMap(ctx, t.Oc)
	if len(errMsg) != 0 {
		framework.Failf(errMsg)
	}
	if gateCm == nil {
		framework.Logf("Skipping admin ack test. Admin ack is not in this baseline.")
		return
	}
	var msg string
	if msg = gateCm.Data["ack-4.8-kube-1.22-api-removals-in-4.9"]; msg == "" {
		framework.Failf("Configmap openshift-config-managed/admin-gates gate ack-4.8-kube-1.22-api-removals-in-4.9 does not contain description.")
	}
	ackCm, errMsg := getAdminAcksConfigMap(ctx, t.Oc)
	if len(errMsg) != 0 {
		framework.Failf(errMsg)
	}
	if ackCm.Data["ack-4.8-kube-1.22-api-removals-in-4.9"] == "true" {
		if !upgradeable(ctx, t.Config) {
			if adminAckRequiredWithMessage(ctx, t.Config, msg) {
				framework.Failf(fmt.Sprintf("Gate ack-4.8-kube-1.22-api-removals-in-4.9 has been ack'ed but Upgradeable is "+
					"false with reason AdminAckRequired and message %q.", msg))
			}
			framework.Logf("Gate ack-4.8-kube-1.22-api-removals-in-4.9 has been ack'ed. Upgradeable is " +
				"false but not for reason AdminAckRequired.")
		}
		// Clear admin ack configmap gate ack
		if errMsg = setAdminGate(ctx, "ack-4.8-kube-1.22-api-removals-in-4.9", "", t.Oc); len(errMsg) != 0 {
			framework.Failf(errMsg)
		}
		foo, msg := getAdminAcksConfigMap(ctx, t.Oc)
		if len(msg) != 0 {
			framework.Logf(msg)
		}
		framework.Logf("ack-4.8-kube-1.22-api-removals-in-4.9 value is %q", foo.Data["ack-4.8-kube-1.22-api-removals-in-4.9"])
	}
	logUpgradeable(ctx, t.Config)
	if errMsg = waitForAdminAckRequired(ctx, t.Config, msg); len(errMsg) != 0 {
		framework.Failf(errMsg)
	}
	logUpgradeable(ctx, t.Config)
	// Update admin ack configmap with ack
	if errMsg = setAdminGate(ctx, "ack-4.8-kube-1.22-api-removals-in-4.9", "true", t.Oc); len(errMsg) != 0 {
		framework.Failf(errMsg)
	}
	if errMsg = waitForUpgradeable(ctx, t.Config); len(errMsg) != 0 {
		framework.Failf(errMsg)
	}
	framework.Logf("Admin Ack verified")
}

func getAdminGatesConfigMap(ctx context.Context, oc *CLI) (*corev1.ConfigMap, string) {
	cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps("openshift-config-managed").Get(ctx, "admin-gates", metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Sprintf("Error accessing configmap openshift-config-managed/admin-gates, err=%v", err)
		} else {
			return nil, ""
		}
	}
	return cm, ""
}

func getAdminAcksConfigMap(ctx context.Context, oc *CLI) (*corev1.ConfigMap, string) {
	cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps("openshift-config").Get(ctx, "admin-acks", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Sprintf("Error accessing configmap openshift-config/admin-acks, err=%v", err)
	}
	return cm, ""
}

// getClusterVersion returns the ClusterVersion object.
func getClusterVersion(ctx context.Context, config *restclient.Config) *configv1.ClusterVersion {
	c, err := configv1client.NewForConfig(config)
	if err != nil {
		framework.Failf(fmt.Sprintf("Error getting config, err=%v", err))
	}
	cv, err := c.ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
	if err != nil {
		framework.Failf(fmt.Sprintf("Error getting custer version, err=%v", err))
	}
	return cv
}

// adminAckRequiredWithMessage returns true if Upgradeable condition reason contains AdminAckRequired
// and message contains given message.
func adminAckRequiredWithMessage(ctx context.Context, config *restclient.Config, message string) bool {
	clusterVersion := getClusterVersion(ctx, config)
	cond := getUpgradeableStatusCondition(clusterVersion.Status.Conditions)
	if cond != nil && strings.Contains(cond.Reason, "AdminAckRequired") && strings.Contains(cond.Message, message) {
		return true
	}
	return false
}

func logUpgradeable(ctx context.Context, config *restclient.Config) {
	clusterVersion := getClusterVersion(ctx, config)
	cond := getUpgradeableStatusCondition(clusterVersion.Status.Conditions)
	if cond != nil {
		framework.Logf(fmt.Sprintf("Upgradeable: Status=%s, Reason=%s, Message=%q.", cond.Status, cond.Reason, cond.Message))
	} else {
		framework.Logf("Upgradeable nil")
	}
}

// upgradeable returns true if the Upgradeable condition is nil or is set to true.
func upgradeable(ctx context.Context, config *restclient.Config) bool {
	clusterVersion := getClusterVersion(ctx, config)
	cond := getUpgradeableStatusCondition(clusterVersion.Status.Conditions)
	if cond == nil || (cond != nil && cond.Status == configv1.ConditionTrue) {
		return true
	}
	return false
}

// setAdminGate gets the admin ack configmap and then updates it with given gate name and given value.
func setAdminGate(ctx context.Context, gateName string, gateValue string, oc *CLI) string {
	ackCm, errMsg := getAdminAcksConfigMap(ctx, oc)
	if len(errMsg) != 0 {
		framework.Failf(errMsg)
	}
	ackCm.Data[gateName] = gateValue
	_, err := oc.AdminKubeClient().CoreV1().ConfigMaps("openshift-config").Update(ctx, ackCm, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Sprintf("Unable to update configmap openshift-config/admin-acks, err=%v.", err)
	}
	return ""
}

func waitForAdminAckRequired(ctx context.Context, config *restclient.Config, message string) string {
	framework.Logf("Waiting for Upgradeable to be AdminAckRequired...")
	if err := wait.PollImmediate(10*time.Second, 9*time.Minute, func() (bool, error) {
		if adminAckRequiredWithMessage(ctx, config, message) {
			return true, nil
		}
		return false, nil
	}); err != nil {
		return fmt.Sprintf("Error while waiting for Upgradeable to go AdminAckRequired with message %q, err=%v", message, err)
	}
	return ""
}

func waitForUpgradeable(ctx context.Context, config *restclient.Config) string {
	framework.Logf("Waiting for Upgradeable true...")
	if err := wait.PollImmediate(10*time.Second, 9*time.Minute, func() (bool, error) {
		if upgradeable(ctx, config) {
			return true, nil
		}
		return false, nil
	}); err != nil {
		return fmt.Sprintf("Error while waiting for Upgradeable to go true, err=%v", err)
	}
	return ""
}

func getUpgradeableStatusCondition(conditions []configv1.ClusterOperatorStatusCondition) *configv1.ClusterOperatorStatusCondition {
	for _, condition := range conditions {
		if condition.Type == configv1.OperatorUpgradeable {
			return &condition
		}
	}
	return nil
}

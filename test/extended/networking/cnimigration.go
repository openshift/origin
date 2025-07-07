package networking

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	networkopernames "github.com/openshift/cluster-network-operator/pkg/names"
	"github.com/openshift/origin/test/extended/util/operator"

	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubetypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	g "github.com/onsi/ginkgo/v2"
	"github.com/pborman/uuid"
)

const (
	optionsEnvKey        = "TEST_SDN_LIVE_MIGRATION_OPTIONS"
	featureGateCRName    = "cluster"
	networkCRName        = "cluster"
	maxMigrationDuration = 2 * time.Hour
	// length of time to wait in-order to confirm migration is in-progress after initiating migration
	networkOpAckTimeout = 10 * time.Minute
)

type testConfig struct {
	targetCNI       string
	rollbackEnabled bool
}

func getTestConfig() testConfig {
	o := testConfig{}
	allOptionsStr := os.Getenv(optionsEnvKey)
	if allOptionsStr == "" {
		return o
	}
	for _, opt := range strings.Split(allOptionsStr, ",") {
		parts := strings.SplitN(opt, "=", 2)
		if len(parts) != 2 {
			framework.Failf("expected option of the form KEY=VALUE instead of %q", opt)
		}
		switch parts[0] {
		case "target-cni":
			o.targetCNI = parts[1]
		case "rollback":
			if strings.ToLower(parts[1]) == "true" {
				o.rollbackEnabled = true
			}
		default:
			framework.Failf("unrecognized option: %s=%s", parts[0], parts[1])
		}
	}
	return o
}

func startLiveMigration(ctx context.Context, c configv1client.Interface, targetCNI string) error {
	patch := []byte(fmt.Sprintf(`{"metadata":{"annotations": {"%s":""}},"spec":{"networkType":"%s"}}`,
		networkopernames.NetworkTypeMigrationAnnotation, targetCNI))
	if err := retry.RetryOnConflict(wait.Backoff{Steps: 10, Duration: time.Second}, func() error {
		_, err := c.ConfigV1().Networks().Patch(ctx, networkCRName, kubetypes.MergePatchType, patch, metav1.PatchOptions{})
		return err
	}); err != nil {
		return err
	}
	return nil
}

func isMigrationInProgressTrue(ctx context.Context, c configv1client.Interface) (bool, error) {
	clusterConfig, err := c.ConfigV1().Networks().Get(ctx, networkCRName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	if meta.IsStatusConditionPresentAndEqual(clusterConfig.Status.Conditions, networkopernames.NetworkTypeMigrationInProgress, metav1.ConditionTrue) {
		return true, nil
	}
	return false, nil
}

func isMigrationComplete(ctx context.Context, c configv1client.Interface) (bool, error) {
	var clusterConfig *configv1.Network
	err := wait.PollUntilContextTimeout(ctx, 1*time.Second, 60*time.Second, true, func(ctx context.Context) (done bool, err error) {
		clusterConfig, err = c.ConfigV1().Networks().Get(ctx, networkCRName, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Failed to get Network CR %s - retrying: %v", networkCRName, err)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return false, err
	}

	if !meta.IsStatusConditionPresentAndEqual(clusterConfig.Status.Conditions, networkopernames.NetworkTypeMigrationInProgress, metav1.ConditionFalse) {
		return false, nil
	}
	if !meta.IsStatusConditionPresentAndEqual(clusterConfig.Status.Conditions, networkopernames.NetworkTypeMigrationMTUReady, metav1.ConditionUnknown) {
		return false, nil
	}
	if !meta.IsStatusConditionPresentAndEqual(clusterConfig.Status.Conditions, networkopernames.NetworkTypeMigrationOriginalCNIPurged, metav1.ConditionUnknown) {
		return false, nil
	}
	if !meta.IsStatusConditionPresentAndEqual(clusterConfig.Status.Conditions, networkopernames.NetworkTypeMigrationTargetCNIInUse, metav1.ConditionUnknown) {
		return false, nil
	}
	if !meta.IsStatusConditionPresentAndEqual(clusterConfig.Status.Conditions, networkopernames.NetworkTypeMigrationTargetCNIAvailable, metav1.ConditionUnknown) {
		return false, nil
	}
	return true, nil
}

func isCNIDeployed(ctx context.Context, c configv1client.Interface, cni string) (bool, error) {
	clusterConfig, err := c.ConfigV1().Networks().Get(ctx, networkCRName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	if clusterConfig.Status.NetworkType == cni {
		return true, nil
	}
	return false, nil
}

func getCurrentCNI(ctx context.Context, configClient configv1client.Interface) (string, error) {
	clusterConfig, err := configClient.ConfigV1().Networks().Get(ctx, networkCRName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return clusterConfig.Status.NetworkType, nil
}

func migrateCNI(ctx context.Context, c configv1client.Interface, config *restclient.Config, targetCNI string) error {
	if inProgress, err := isMigrationInProgressTrue(ctx, c); err != nil {
		return err
	} else if inProgress {
		return fmt.Errorf("migration is already in-progress")
	}
	if isDeployed, err := isCNIDeployed(ctx, c, targetCNI); err != nil {
		return err
	} else if isDeployed {
		return fmt.Errorf("CNI %q is already deployed", targetCNI)
	}
	initCNI, err := getCurrentCNI(ctx, c)
	if err != nil {
		return fmt.Errorf("failed to get current CNI: %v", err)
	}
	framework.Logf("Starting migration from CNI %s to %s", initCNI, targetCNI)
	framework.Logf("Max duration of migration is %0.2f", maxMigrationDuration.Minutes())
	kubeClient := kubernetes.NewForConfigOrDie(config)
	uid := uuid.NewRandom().String()
	recordEvent(kubeClient, uid, "Migration", "MigrationStarted", fmt.Sprintf("migration from CNI %s to %s", initCNI, targetCNI), false)
	// trigger the migration and record migration in-progress
	if err = startLiveMigration(ctx, c, targetCNI); err != nil {
		return fmt.Errorf("failed to start live migration: %v", err)
	}
	// sanity-check: wait until the network operator acknowledges that migration is in-progress.
	if err := wait.PollImmediate(5*time.Second, networkOpAckTimeout, func() (bool, error) {
		inProgress, err := isMigrationInProgressTrue(ctx, c)
		if err != nil {
			return false, err
		}
		return inProgress, nil
	}); err != nil {
		recordEvent(kubeClient, uid, "Migration", "MigrationFailed", fmt.Sprintf("failed to acknowledge migration request: %v", err), true)
		return fmt.Errorf("timed out waiting for network operator to acknowledge migration is in-progress: %v", err)
	}
	migrationStart := time.Now()
	if err := wait.PollUntilContextTimeout(ctx, 10*time.Second, maxMigrationDuration, true, func(ctx context.Context) (bool, error) {
		isMigrationCompleted, err := isMigrationComplete(ctx, c)
		if err != nil {
			return false, err
		}
		return isMigrationCompleted, nil
	}); err != nil {
		return fmt.Errorf("cluster did not complete CNI migration from CNI %s to %s: %v", initCNI, targetCNI, err)
	}
	migrationEnd := time.Now()
	migrationDuration := migrationEnd.Sub(migrationStart)
	migrationDurationMinutes := migrationDuration.Minutes()
	framework.Logf("Completed migration from CNI %s to %s. Completed in %0.2f minutes", initCNI, targetCNI, migrationDurationMinutes)
	recordEvent(kubeClient, uid, "Migration", "MigrationSucceeded",
		fmt.Sprintf("from CNI %s to %s. Completed in %0.2f minutes", initCNI, targetCNI, migrationDurationMinutes), false)
	return nil
}

// recordEvent attempts to record an event to the cluster to indicate actions taken during a live
// migration for timeline review.
func recordEvent(client kubernetes.Interface, uid, action, reason, note string, warning bool) {
	currentTime := metav1.MicroTime{Time: time.Now()}
	t := corev1.EventTypeNormal
	if warning {
		t = corev1.EventTypeWarning
	}
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()
	ns := "openshift-network-operator"
	_, err := client.EventsV1().Events(ns).Create(ctx, &eventsv1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%v.%x", "cni-migration", currentTime.UnixNano()),
		},
		Regarding:           corev1.ObjectReference{Kind: "Network", Name: networkCRName, Namespace: ns, APIVersion: configv1.GroupVersion.String()},
		Action:              action,
		Reason:              reason,
		Note:                note,
		Type:                t,
		EventTime:           currentTime,
		ReportingController: "openshift-tests.openshift.io/cni-migration",
		ReportingInstance:   uid,
	}, metav1.CreateOptions{})
	if err != nil {
		framework.Logf("Unable to record cluster event: %v", err)
	}
}

func isSupportedCNI(cni string) bool {
	return cni == OVNKubernetesPluginName || cni == OpenshiftSDNPluginName
}

func getRollBackCNI(cni string) string {
	if cni == OVNKubernetesPluginName {
		return OpenshiftSDNPluginName
	}
	if cni == OpenshiftSDNPluginName {
		return OVNKubernetesPluginName
	}
	panic(fmt.Sprintf("unsupported CNI %q specified. Unable to determine rollback CNI", cni))
}

var _ = g.Describe("[sig-network][Feature:CNIMigration]", g.Ordered, func() {
	var tc testConfig

	g.BeforeAll(func() {
		tc = getTestConfig()
	})

	g.BeforeEach(func() {
		if tc.targetCNI == "" {
			e2eskipper.Skipf("CNI migration tests are disabled because %s environment key does not have a value which contains 'target-key=$CNI'", optionsEnvKey)
		}
		if !isSupportedCNI(tc.targetCNI) {
			framework.Failf("target CNI %q is unsupported", tc.targetCNI)
		}
	})

	g.It("Cluster should not be live migrating before beginning migration [Early][Suite:openshift/network/live-migration]", func(ctx context.Context) {
		config, err := framework.LoadConfig()
		framework.ExpectNoError(err)
		client := configv1client.NewForConfigOrDie(config)
		inProgress, err := isMigrationInProgressTrue(ctx, client)
		framework.ExpectNoError(err)
		if inProgress {
			framework.Fail("migration is already in-progress")
		}
	})

	g.It("Target CNI should not be deployed [Early][Suite:openshift/network/live-migration]", func(ctx context.Context) {
		config, err := framework.LoadConfig()
		framework.ExpectNoError(err)
		client := configv1client.NewForConfigOrDie(config)
		isCNIDeployed, err := isCNIDeployed(ctx, client, tc.targetCNI)
		framework.ExpectNoError(err)
		if isCNIDeployed {
			framework.Failf("CNI %q is already deployed", tc.targetCNI)
		}
	})

	g.It("All nodes should be in ready state [Early][Suite:openshift/network/live-migration]", func(ctx context.Context) {
		config, err := framework.LoadConfig()
		framework.ExpectNoError(err)
		client := kubernetes.NewForConfigOrDie(config)
		nodeList, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		framework.ExpectNoError(err)
		var errMsgs []string
		for _, node := range nodeList.Items {
			for _, condition := range node.Status.Conditions {
				if condition.Type == corev1.NodeReady && condition.Status != corev1.ConditionTrue {
					err := fmt.Errorf("node/%s ready state is not true, status: %v, reason: %s", node.Name, condition.Status, condition.Reason)
					framework.Logf("Error: %v", err)
					errMsgs = append(errMsgs, err.Error())
				}
			}
		}
		if len(errMsgs) > 0 {
			combinedErr := fmt.Errorf("%s", strings.Join(errMsgs, "; "))
			framework.ExpectNoError(combinedErr)
		}
	})

	g.It("Should perform live migration [Disruptive][Suite:openshift/network/live-migration]", func(ctx context.Context) {
		config, err := framework.LoadConfig()
		framework.ExpectNoError(err)
		client := configv1client.NewForConfigOrDie(config)
		framework.ExpectNoError(err)
		framework.ExpectNoError(
			migrateCNI(ctx, client, config, tc.targetCNI),
			fmt.Sprintf("during to migrate to CNI %s", tc.targetCNI),
		)
		if tc.rollbackEnabled {
			rollbackCNI := getRollBackCNI(tc.targetCNI)
			framework.ExpectNoError(
				migrateCNI(ctx, client, config, rollbackCNI),
				fmt.Sprintf("during rollback to CNI %s from %s", rollbackCNI, tc.targetCNI),
			)
		}
	})

	g.It("Cluster operators should be stable [Late][Suite:openshift/network/live-migration]", func(ctx context.Context) {
		config, err := framework.LoadConfig()
		framework.ExpectNoError(err)
		client := configv1client.NewForConfigOrDie(config)
		// 5 min timeout for operators to "settle"
		framework.ExpectNoError(operator.WaitForOperatorsToSettle(ctx, client, 5))
	})
})

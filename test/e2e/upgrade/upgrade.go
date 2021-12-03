package upgrade

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
	"k8s.io/kubernetes/test/e2e/upgrades/apps"
	"k8s.io/kubernetes/test/e2e/upgrades/node"

	g "github.com/onsi/ginkgo"
	"github.com/pborman/uuid"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/test/e2e/upgrade/adminack"
	"github.com/openshift/origin/test/e2e/upgrade/alert"
	"github.com/openshift/origin/test/e2e/upgrade/service"
	"github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/disruption"
	"github.com/openshift/origin/test/extended/util/disruption/controlplane"
	"github.com/openshift/origin/test/extended/util/disruption/frontends"
	"github.com/openshift/origin/test/extended/util/disruption/imageregistry"
	"github.com/openshift/origin/test/extended/util/operator"
)

// NoTests is an empty list of tests
func NoTests() []upgrades.Test {
	return []upgrades.Test{}
}

// AllTests includes all tests (minimal + disruption)
func AllTests() []upgrades.Test {
	return []upgrades.Test{
		&adminack.UpgradeTest{},
		controlplane.NewKubeAvailableWithNewConnectionsTest(),
		controlplane.NewOpenShiftAvailableNewConnectionsTest(),
		controlplane.NewOAuthAvailableNewConnectionsTest(),
		controlplane.NewKubeAvailableWithConnectionReuseTest(),
		controlplane.NewOpenShiftAvailableWithConnectionReuseTest(),
		controlplane.NewOAuthAvailableWithConnectionReuseTest(),
		&alert.UpgradeTest{},
		&frontends.AvailableTest{},
		&service.UpgradeTest{},
		&node.SecretUpgradeTest{},
		&apps.ReplicaSetUpgradeTest{},
		&apps.StatefulSetUpgradeTest{},
		&apps.DeploymentUpgradeTest{},
		&apps.JobUpgradeTest{},
		&node.ConfigMapUpgradeTest{},
		&apps.DaemonSetUpgradeTest{},
		&imageregistry.AvailableTest{},
	}
}

var (
	upgradeToImage             string
	upgradeTests               = []upgrades.Test{}
	upgradeAbortAt             int
	upgradeDisruptRebootPolicy string
)

// upgradeAbortAtRandom is a special value indicating the abort should happen at a random percentage
// between (0,100].
const upgradeAbortAtRandom = -1

// SetTests controls the list of tests to run during an upgrade. See AllTests for the supported
// suite.
func SetTests(tests []upgrades.Test) {
	upgradeTests = tests
}

// SetToImage sets the image that will be upgraded to. This may be a comma delimited list
// of sequential upgrade attempts.
func SetToImage(image string) {
	upgradeToImage = image
}

func SetUpgradeDisruptReboot(policy string) error {
	switch policy {
	case "graceful", "force":
		upgradeDisruptRebootPolicy = policy
		return nil
	default:
		upgradeDisruptRebootPolicy = ""
		return fmt.Errorf("disrupt-reboot must be empty, 'graceful', or 'force'")
	}
}

// SetUpgradeAbortAt defines abort behavior during an upgrade. Allowed values are:
//
// * empty string - do not abort
// * integer between 0-100 - once this percentage of operators have updated, rollback to the previous version
//
func SetUpgradeAbortAt(policy string) error {
	if len(policy) == 0 {
		upgradeAbortAt = 0
	}
	if policy == "random" {
		upgradeAbortAt = upgradeAbortAtRandom
		return nil
	}
	if val, err := strconv.Atoi(policy); err == nil {
		if val < 0 || val > 100 {
			return fmt.Errorf("abort-at must be empty, set to 'random', or an integer in [0,100], inclusive")
		}
		if val == 0 {
			upgradeAbortAt = 1
		} else {
			upgradeAbortAt = val
		}
		return nil
	}
	return fmt.Errorf("abort-at must be empty, set to 'random', or an integer in [0,100], inclusive")
}

var _ = g.Describe("[sig-arch][Feature:ClusterUpgrade]", func() {
	f := framework.NewDefaultFramework("cluster-upgrade")
	f.SkipNamespaceCreation = true
	f.SkipPrivilegedPSPBinding = true

	g.It("Cluster should remain functional during upgrade [Disruptive]", func() {
		config, err := framework.LoadConfig()
		framework.ExpectNoError(err)
		client := configv1client.NewForConfigOrDie(config)
		dynamicClient := dynamic.NewForConfigOrDie(config)

		upgCtx, err := getUpgradeContext(client, upgradeToImage)
		framework.ExpectNoError(err, "determining what to upgrade to version=%s image=%s", "", upgradeToImage)

		disruption.Run(f, "Cluster upgrade", "upgrade",
			disruption.TestData{
				UpgradeType:    upgrades.ClusterUpgrade,
				UpgradeContext: *upgCtx,
			},
			upgradeTests,
			func() {
				for i := 1; i < len(upgCtx.Versions); i++ {
					framework.ExpectNoError(clusterUpgrade(f, client, dynamicClient, config, upgCtx.Versions[i]), fmt.Sprintf("during upgrade to %s", upgCtx.Versions[i].NodeImage))
				}
			},
		)
	})
})

func latestHistory(history []configv1.UpdateHistory) *configv1.UpdateHistory {
	if len(history) > 0 {
		return &history[0]
	}
	return nil
}

func latestCompleted(history []configv1.UpdateHistory) (*configv1.Update, bool) {
	for _, version := range history {
		if version.State == configv1.CompletedUpdate {
			return &configv1.Update{Version: version.Version, Image: version.Image}, true
		}
	}
	return nil, false
}

func getUpgradeContext(c configv1client.Interface, upgradeImage string) (*upgrades.UpgradeContext, error) {
	if upgradeImage == "[pause]" {
		return &upgrades.UpgradeContext{
			Versions: []upgrades.VersionContext{
				{Version: *version.MustParseSemantic("0.0.1"), NodeImage: "[pause]"},
				{Version: *version.MustParseSemantic("0.0.2"), NodeImage: "[pause]"},
			},
		}, nil
	}

	cv, err := c.ConfigV1().ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if cv.Spec.DesiredUpdate != nil {
		if cv.Status.ObservedGeneration != cv.Generation {
			return nil, fmt.Errorf("cluster may be in the process of upgrading, cannot start a test")
		}
		if len(cv.Status.History) > 0 && cv.Status.History[0].State != configv1.CompletedUpdate {
			return nil, fmt.Errorf("cluster is already being upgraded, cannot start a test: %s", versionString(*cv.Spec.DesiredUpdate))
		}
	}
	if c := findCondition(cv.Status.Conditions, configv1.OperatorDegraded); c != nil && c.Status == configv1.ConditionTrue {
		return nil, fmt.Errorf("cluster is reporting a degraded condition, cannot continue: %v", c.Message)
	}
	if c := findCondition(cv.Status.Conditions, configv1.ClusterStatusConditionType("Failing")); c != nil && c.Status == configv1.ConditionTrue {
		return nil, fmt.Errorf("cluster is reporting a failing condition, cannot continue: %v", c.Message)
	}
	if c := findCondition(cv.Status.Conditions, configv1.OperatorProgressing); c == nil || c.Status != configv1.ConditionFalse {
		return nil, fmt.Errorf("cluster must be reporting a progressing=false condition, cannot continue: %#v", c)
	}
	if c := findCondition(cv.Status.Conditions, configv1.OperatorAvailable); c == nil || c.Status != configv1.ConditionTrue {
		return nil, fmt.Errorf("cluster must be reporting an available=true condition, cannot continue: %#v", c)
	}

	current, ok := latestCompleted(cv.Status.History)
	if !ok {
		return nil, fmt.Errorf("cluster has not rolled out a version yet, must wait until that is complete")
	}

	curVer, err := version.ParseSemantic(current.Version)
	if err != nil {
		return nil, err
	}

	upgCtx := &upgrades.UpgradeContext{
		Versions: []upgrades.VersionContext{
			{
				Version:   *curVer,
				NodeImage: current.Image,
			},
		},
	}

	if len(upgradeImage) == 0 {
		return upgCtx, nil
	}

	upgradeImages := strings.Split(upgradeImage, ",")
	if (len(upgradeImages[0]) > 0 && upgradeImages[0] == current.Image) || (len(upgradeImages[0]) > 0 && upgradeImages[0] == current.Version) {
		framework.Logf("cluster is already at version %s", versionString(*current))
	}
	for _, upgradeImage := range upgradeImages {
		var next upgrades.VersionContext
		if nextVer, err := version.ParseSemantic(upgradeImage); err == nil {
			next.Version = *nextVer
		} else {
			next.NodeImage = upgradeImage
		}
		upgCtx.Versions = append(upgCtx.Versions, next)
	}

	return upgCtx, nil
}

var errControlledAbort = fmt.Errorf("beginning abort")

func clusterUpgrade(f *framework.Framework, c configv1client.Interface, dc dynamic.Interface, config *rest.Config, version upgrades.VersionContext) error {
	fmt.Fprintf(os.Stderr, "\n\n\n")
	defer func() { fmt.Fprintf(os.Stderr, "\n\n\n") }()

	if version.NodeImage == "[pause]" {
		framework.Logf("Running a dry-run upgrade test")
		wait.PollImmediate(10*time.Second, 5*time.Minute, func() (bool, error) {
			framework.Logf("Waiting ...")
			return false, nil
		})
		return nil
	}

	uid := uuid.NewRandom().String()

	kubeClient := kubernetes.NewForConfigOrDie(config)

	// this is very long.  We should update the clusteroperator junit to give us a duration.
	maximumDuration := 150 * time.Minute
	baseDurationToSoftFailure := 75 * time.Minute
	durationToSoftFailure := baseDurationToSoftFailure

	infra, err := c.ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	framework.ExpectNoError(err)
	if infra.Status.PlatformStatus.Type == configv1.AWSPlatformType {
		// due to https://bugzilla.redhat.com/show_bug.cgi?id=1943804 upgrades take ~12 extra minutes on AWS
		// and see commit d69db34a816f3ce8a9ab567621d145c5cd2d257f which notes that some AWS upgrades can
		// take close to 105 minutes total (75 is base duration, so adding 30 more if it's AWS)
		durationToSoftFailure = baseDurationToSoftFailure + (30 * time.Minute)
	} else {
		// if the cluster is on AWS we've already bumped the timeout enough, but if not we need to check if
		// the CNI is OVN and increase our timeout for that
		network, err := c.ConfigV1().Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
		framework.ExpectNoError(err)
		if network.Status.NetworkType == "OVNKubernetes" {
			// deploying with OVN is expected to take longer. on average, ~15m longer
			// some extra context to this increase which links to a jira showing which operators take longer:
			// compared to OpenShiftSDN:
			//   https://bugzilla.redhat.com/show_bug.cgi?id=1942164
			durationToSoftFailure = baseDurationToSoftFailure + (15 * time.Minute)
		}
	}

	if cv, err := c.ConfigV1().ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{}); err != nil {
		return err
	} else if util.GetMajorMinor(version.Version.String()) != util.GetMajorMinor(cv.Status.Desired.Version) {
		framework.Logf("Upgrade from %s to %s changes the major.minor, so allowing an additional 15 minutes", cv.Status.Desired.Version, version.Version.String())
		durationToSoftFailure += 15 * time.Minute
	}

	framework.Logf("Starting upgrade to version=%s image=%s attempt=%s", version.Version.String(), version.NodeImage, uid)
	recordClusterEvent(kubeClient, uid, "Upgrade", "UpgradeStarted", fmt.Sprintf("version/%s image/%s", version.Version.String(), version.NodeImage), false)

	// decide whether to abort at a percent
	abortAt := upgradeAbortAt
	switch abortAt {
	case 0:
		// no abort
	case upgradeAbortAtRandom:
		abortAt = int(rand.Int31n(100) + 1)
		maximumDuration *= 2
		durationToSoftFailure *= 2
		framework.Logf("Upgrade will be aborted and the cluster will roll back to the current version after %d%% of operators have upgraded (picked randomly)", abortAt)
	default:
		maximumDuration *= 2
		durationToSoftFailure *= 2
		framework.Logf("Upgrade will be aborted and the cluster will roll back to the current version after %d%% of operators have upgraded", upgradeAbortAt)
	}

	var (
		desired  configv1.Update
		original *configv1.ClusterVersion
		updated  *configv1.ClusterVersion
	)

	monitor := versionMonitor{
		client: c,
	}
	defer monitor.Describe(f)

	// trigger the update and record verification as an independent step
	if err := disruption.RecordJUnit(
		f,
		"[sig-cluster-lifecycle] Cluster version operator acknowledges upgrade",
		func() error {
			cv, err := c.ConfigV1().ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})
			if err != nil {
				return err
			}

			original = cv
			cv = cv.DeepCopy()
			desired = configv1.Update{
				Version: version.Version.String(),
				Image:   version.NodeImage,
				Force:   true,
			}
			monitor.oldVersion = original.Status.Desired.Version

			cv.Spec.DesiredUpdate = &desired
			cv, err = c.ConfigV1().ClusterVersions().Update(context.Background(), cv, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
			updated = cv
			var observedGeneration int64

			// wait until the cluster acknowledges the update
			if err := wait.PollImmediate(5*time.Second, 2*time.Minute, func() (bool, error) {
				cv, _, err := monitor.Check(updated.Generation, desired)
				if err != nil || cv == nil {
					return false, err
				}
				observedGeneration = cv.Status.ObservedGeneration
				return cv.Status.ObservedGeneration >= updated.Generation, nil

			}); err != nil {
				return fmt.Errorf(
					"Timed out waiting for cluster to acknowledge upgrade: %v; observedGeneration: %d; updated.Generation: %d",
					err, observedGeneration, updated.Generation)
			}
			return nil
		},
	); err != nil {
		recordClusterEvent(kubeClient, uid, "Upgrade", "UpgradeFailed", fmt.Sprintf("failed to acknowledge version: %v", err), true)
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go monitor.Disrupt(ctx, kubeClient, upgradeDisruptRebootPolicy)

	// observe the upgrade, taking action as necessary
	if err := disruption.RecordJUnit(
		f,
		"[sig-cluster-lifecycle] Cluster completes upgrade",
		func() error {
			framework.Logf("Cluster version operator acknowledged upgrade request")
			aborted := false
			action := "upgrade"
			var lastMessage string
			upgradeStarted := time.Now()

			if err := wait.PollImmediate(10*time.Second, maximumDuration, func() (bool, error) {
				cv, msg, err := monitor.Check(updated.Generation, desired)
				if msg != "" {
					lastMessage = msg
				}
				if err != nil || cv == nil {
					return false, err
				}

				if !aborted && monitor.ShouldUpgradeAbort(abortAt) {
					framework.Logf("Instructing the cluster to return to %s / %s", original.Status.Desired.Version, original.Status.Desired.Image)
					desired = configv1.Update{
						Image: original.Status.Desired.Image,
						Force: true,
					}
					if err := retry.RetryOnConflict(wait.Backoff{Steps: 10, Duration: time.Second}, func() error {
						cv, err := c.ConfigV1().ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})
						if err != nil {
							return err
						}
						cv.Spec.DesiredUpdate = &desired
						cv, err = c.ConfigV1().ClusterVersions().Update(context.Background(), cv, metav1.UpdateOptions{})
						if err == nil {
							updated = cv
						}
						return err
					}); err != nil {
						return false, err
					}
					recordClusterEvent(kubeClient, uid, "Upgrade", "UpgradeRollback", fmt.Sprintf("version/%s image/%s", original.Status.Desired.Version, original.Status.Desired.Version), false)
					aborted = true
					action = "aborted upgrade"
					return false, nil
				}

				return monitor.Reached(cv, desired)

			}); err != nil {
				if lastMessage != "" {
					return fmt.Errorf("Cluster did not complete %s: %v: %s", action, err, lastMessage)
				}
				return fmt.Errorf("Cluster did not complete %s: %v", action, err)
			}

			framework.Logf("Completed %s to %s", action, versionString(desired))
			recordClusterEvent(kubeClient, uid, "Upgrade", "UpgradeVersion", fmt.Sprintf("version/%s image/%s", updated.Status.Desired.Version, updated.Status.Desired.Version), false)

			// record whether the cluster was fast or slow upgrading.  Don't fail the test, we still want signal on the actual tests themselves.
			upgradeEnded := time.Now()
			upgradeDuration := upgradeEnded.Sub(upgradeStarted)
			testCaseName := fmt.Sprintf("[sig-cluster-lifecycle] cluster upgrade should complete in %0.2f minutes", durationToSoftFailure.Minutes())
			failure := ""
			if upgradeDuration > durationToSoftFailure {
				failure = fmt.Sprintf("%s to %s took too long: %0.2f minutes", action, versionString(desired), upgradeDuration.Minutes())
			}
			disruption.RecordJUnitResult(f, testCaseName, upgradeDuration, failure)

			return nil
		},
	); err != nil {
		recordClusterEvent(kubeClient, uid, "Upgrade", "UpgradeFailed", fmt.Sprintf("failed to reach cluster version: %v", err), true)
		return err
	}

	var errMasterUpdating error
	if err := disruption.RecordJUnit(
		f,
		"[sig-mco] Machine config pools complete upgrade",
		func() error {
			framework.Logf("Waiting on pools to be upgraded")
			if err := wait.PollImmediate(10*time.Second, 30*time.Minute, func() (bool, error) {
				mcps := dc.Resource(schema.GroupVersionResource{
					Group:    "machineconfiguration.openshift.io",
					Version:  "v1",
					Resource: "machineconfigpools",
				})
				pools, err := mcps.List(context.Background(), metav1.ListOptions{})
				if err != nil {
					framework.Logf("error getting pools %v", err)
					return false, nil
				}
				allUpdated := true
				for _, p := range pools.Items {
					updated, requiresUpdate := IsPoolUpdated(mcps, p.GetName())
					allUpdated = allUpdated && updated

					// Invariant: when CVO reaches level, MCO is required to have rolled out control plane updates
					if p.GetName() == "master" && requiresUpdate && errMasterUpdating == nil {
						errMasterUpdating = fmt.Errorf("the %q pool should be updated before the CVO reports available at the new version", p.GetName())
						framework.Logf("Invariant violation detected: %s", errMasterUpdating)
					}
				}
				return allUpdated, nil
			}); err != nil {
				return fmt.Errorf("Pools did not complete upgrade: %v", err)
			}
			framework.Logf("All pools completed upgrade")
			return nil
		},
	); err != nil {
		recordClusterEvent(kubeClient, uid, "Upgrade", "UpgradeFailed", fmt.Sprintf("failed to upgrade nodes: %v", err), true)
		return err
	}

	if errMasterUpdating != nil {
		recordClusterEvent(kubeClient, uid, "Upgrade", "UpgradeFailed", fmt.Sprintf("master was updating after cluster version reached level: %v", errMasterUpdating), true)
		return errMasterUpdating
	}

	if err := disruption.RecordJUnit(
		f,
		"[sig-cluster-lifecycle] ClusterOperators are available and not degraded after upgrade",
		func() error {
			if err := operator.WaitForOperatorsToSettle(context.TODO(), c); err != nil {
				return err
			}
			return nil
		},
	); err != nil {
		recordClusterEvent(kubeClient, uid, "Upgrade", "UpgradeFailed", fmt.Sprintf("failed to settle operators: %v", err), true)
		return err
	}

	recordClusterEvent(kubeClient, uid, "Upgrade", "UpgradeComplete", fmt.Sprintf("version/%s image/%s", updated.Status.Desired.Version, updated.Status.Desired.Image), false)
	return nil
}

// recordClusterEvent attempts to record an event to the cluster to indicate actions taken during an
// upgrade for timeline review.
func recordClusterEvent(client kubernetes.Interface, uid, action, reason, note string, warning bool) {
	currentTime := metav1.MicroTime{Time: time.Now()}
	t := v1.EventTypeNormal
	if warning {
		t = v1.EventTypeWarning
	}
	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancelFn()
	ns := "openshift-cluster-version"
	_, err := client.EventsV1().Events(ns).Create(ctx, &eventsv1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%v.%x", "cluster", currentTime.UnixNano()),
		},
		Regarding:           v1.ObjectReference{Kind: "ClusterVersion", Name: "cluster", Namespace: ns, APIVersion: configv1.GroupVersion.String()},
		Action:              action,
		Reason:              reason,
		Note:                note,
		Type:                t,
		EventTime:           currentTime,
		ReportingController: "openshift-tests.openshift.io/upgrade",
		ReportingInstance:   uid,
	}, metav1.CreateOptions{})
	if err != nil {
		framework.Logf("Unable to record cluster event: %v", err)
	}
}

// TODO(runcom): drop this when MCO types are in openshift/api and we can use the typed client directly
func IsPoolUpdated(dc dynamic.NamespaceableResourceInterface, name string) (poolUpToDate bool, poolIsUpdating bool) {
	pool, err := dc.Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		framework.Logf("error getting pool %s: %v", name, err)
		return false, false
	}

	paused, found, err := unstructured.NestedBool(pool.Object, "spec", "paused")
	if err != nil || !found {
		return false, false
	}

	conditions, found, err := unstructured.NestedFieldNoCopy(pool.Object, "status", "conditions")
	if err != nil || !found {
		return false, false
	}
	original, ok := conditions.([]interface{})
	if !ok {
		return false, false
	}
	var updated, updating, degraded bool
	for _, obj := range original {
		o, ok := obj.(map[string]interface{})
		if !ok {
			return false, false
		}
		t, found, err := unstructured.NestedString(o, "type")
		if err != nil || !found {
			return false, false
		}
		s, found, err := unstructured.NestedString(o, "status")
		if err != nil || !found {
			return false, false
		}
		if t == "Updated" && s == "True" {
			updated = true
		}
		if t == "Updating" && s == "True" {
			updating = true
		}
		if t == "Degraded" && s == "True" {
			degraded = true
		}
	}
	if paused {
		framework.Logf("Pool %s is paused, treating as up-to-date (Updated: %v, Updating: %v, Degraded: %v)", name, updated, updating, degraded)
		return true, updating
	}
	if updated && !updating && !degraded {
		return true, updating
	}
	framework.Logf("Pool %s is still reporting (Updated: %v, Updating: %v, Degraded: %v)", name, updated, updating, degraded)
	return false, updating
}

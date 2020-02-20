package upgrade

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

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
	apps "k8s.io/kubernetes/test/e2e/upgrades/apps"

	g "github.com/onsi/ginkgo"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/test/e2e/upgrade/service"
	"github.com/openshift/origin/test/extended/util/disruption"
	"github.com/openshift/origin/test/extended/util/disruption/controlplane"
	"github.com/openshift/origin/test/extended/util/disruption/frontends"
)

func AllTests() []upgrades.Test {
	return []upgrades.Test{
		&controlplane.AvailableTest{},
		&frontends.AvailableTest{},
		&service.UpgradeTest{},
		&upgrades.SecretUpgradeTest{},
		&apps.ReplicaSetUpgradeTest{},
		&apps.StatefulSetUpgradeTest{},
		&apps.DeploymentUpgradeTest{},
		&apps.JobUpgradeTest{},
		&upgrades.ConfigMapUpgradeTest{},
		&apps.DaemonSetUpgradeTest{},
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

var _ = g.Describe("[Disruptive]", func() {
	f := framework.NewDefaultFramework("cluster-upgrade")
	f.SkipNamespaceCreation = true
	f.SkipPrivilegedPSPBinding = true

	g.Describe("Cluster upgrade", func() {
		g.It("should maintain a functioning cluster [Feature:ClusterUpgrade]", func() {
			config, err := framework.LoadConfig()
			framework.ExpectNoError(err)
			client := configv1client.NewForConfigOrDie(config)
			dynamicClient := dynamic.NewForConfigOrDie(config)

			upgCtx, err := getUpgradeContext(client, upgradeToImage)
			framework.ExpectNoError(err, "determining what to upgrade to version=%s image=%s", "", upgradeToImage)

			disruption.Run(
				"Cluster upgrade",
				"upgrade",
				disruption.TestData{
					UpgradeType:    upgrades.ClusterUpgrade,
					UpgradeContext: *upgCtx,
				},
				upgradeTests,
				func() {
					for i := 1; i < len(upgCtx.Versions); i++ {
						framework.ExpectNoError(clusterUpgrade(client, dynamicClient, config, upgCtx.Versions[i]), fmt.Sprintf("during upgrade to %s", upgCtx.Versions[i].NodeImage))
					}
				},
			)
		})
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

	cv, err := c.ConfigV1().ClusterVersions().Get("version", metav1.GetOptions{})
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
		return nil, fmt.Errorf("cluster is already at version %s", versionString(*current))
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

func clusterUpgrade(c configv1client.Interface, dc dynamic.Interface, config *rest.Config, version upgrades.VersionContext) error {
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

	kubeClient := kubernetes.NewForConfigOrDie(config)

	maximumDuration := 75 * time.Minute

	framework.Logf("Starting upgrade to version=%s image=%s", version.Version.String(), version.NodeImage)

	// decide whether to abort at a percent
	abortAt := upgradeAbortAt
	switch abortAt {
	case 0:
		// no abort
	case upgradeAbortAtRandom:
		abortAt = int(rand.Int31n(100) + 1)
		maximumDuration *= 2
		framework.Logf("Upgrade will be aborted and the cluster will roll back to the current version after %d%% of operators have upgraded (picked randomly)", abortAt)
	default:
		maximumDuration *= 2
		framework.Logf("Upgrade will be aborted and the cluster will roll back to the current version after %d%% of operators have upgraded", upgradeAbortAt)
	}

	// trigger the update
	cv, err := c.ConfigV1().ClusterVersions().Get("version", metav1.GetOptions{})
	if err != nil {
		return err
	}
	oldImage := cv.Status.Desired.Image
	oldVersion := cv.Status.Desired.Version
	desired := configv1.Update{
		Version: version.Version.String(),
		Image:   version.NodeImage,
		Force:   true,
	}
	cv.Spec.DesiredUpdate = &desired
	updated, err := c.ConfigV1().ClusterVersions().Update(cv)
	if err != nil {
		return err
	}

	monitor := versionMonitor{
		client:     c,
		oldVersion: oldVersion,
	}

	// wait until the cluster acknowledges the update
	if err := wait.PollImmediate(5*time.Second, 2*time.Minute, func() (bool, error) {
		cv, _, err := monitor.Check(updated.Generation, desired)
		if err != nil || cv == nil {
			return false, err
		}
		return cv.Status.ObservedGeneration >= updated.Generation, nil

	}); err != nil {
		monitor.Output()
		return fmt.Errorf("Cluster did not acknowledge request to upgrade in a reasonable time: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go monitor.Disrupt(ctx, kubeClient, upgradeDisruptRebootPolicy)

	// observe the upgrade, taking action as necessary
	framework.Logf("Cluster version operator acknowledged upgrade request")
	aborted := false
	var lastMessage string
	if err := wait.PollImmediate(10*time.Second, maximumDuration, func() (bool, error) {
		cv, msg, err := monitor.Check(updated.Generation, desired)
		if msg != "" {
			lastMessage = msg
		}
		if err != nil || cv == nil {
			return false, err
		}

		if !aborted && monitor.ShouldUpgradeAbort(abortAt) {
			framework.Logf("Instructing the cluster to return to %s / %s", oldVersion, oldImage)
			desired = configv1.Update{
				Image: oldImage,
				Force: true,
			}
			if err := retry.RetryOnConflict(wait.Backoff{Steps: 10, Duration: time.Second}, func() error {
				cv, err := c.ConfigV1().ClusterVersions().Get("version", metav1.GetOptions{})
				if err != nil {
					return err
				}
				cv.Spec.DesiredUpdate = &desired
				cv, err = c.ConfigV1().ClusterVersions().Update(cv)
				if err == nil {
					updated = cv
				}
				return err
			}); err != nil {
				return false, err
			}
			aborted = true
			return false, nil
		}

		return monitor.Reached(cv, desired)

	}); err != nil {
		monitor.Output()
		if lastMessage != "" {
			return fmt.Errorf("Cluster did not complete upgrade: %v: %s", err, lastMessage)
		}
		return fmt.Errorf("Cluster did not complete upgrade: %v", err)
	}

	framework.Logf("Completed upgrade to %s", versionString(desired))

	framework.Logf("Waiting on pools to be upgraded")
	if err := wait.PollImmediate(10*time.Second, 30*time.Minute, func() (bool, error) {
		mcps := dc.Resource(schema.GroupVersionResource{
			Group:    "machineconfiguration.openshift.io",
			Version:  "v1",
			Resource: "machineconfigpools",
		})
		pools, err := mcps.List(metav1.ListOptions{})
		if err != nil {
			framework.Logf("error getting pools %v", err)
			return false, nil
		}
		allUpdated := true
		for _, p := range pools.Items {
			updated, err := IsPoolUpdated(mcps, p.GetName())
			if err != nil {
				framework.Logf("error checking pool %s: %v", p.GetName(), err)
				return false, nil
			}
			allUpdated = allUpdated && updated
		}
		return allUpdated, nil
	}); err != nil {
		return fmt.Errorf("Pools did not complete upgrade: %v", err)
	}
	framework.Logf("All pools completed upgrade")

	return nil
}

// TODO(runcom): drop this when MCO types are in openshift/api and we can use the typed client directly
func IsPoolUpdated(dc dynamic.NamespaceableResourceInterface, name string) (bool, error) {
	pool, err := dc.Get(name, metav1.GetOptions{})
	if err != nil {
		framework.Logf("error getting pool %s: %v", name, err)
		return false, nil
	}
	conditions, found, err := unstructured.NestedFieldNoCopy(pool.Object, "status", "conditions")
	if err != nil || !found {
		return false, nil
	}
	original, ok := conditions.([]interface{})
	if !ok {
		return false, nil
	}
	var updated, updating, degraded bool
	for _, obj := range original {
		o, ok := obj.(map[string]interface{})
		if !ok {
			return false, nil
		}
		t, found, err := unstructured.NestedString(o, "type")
		if err != nil || !found {
			return false, nil
		}
		s, found, err := unstructured.NestedString(o, "status")
		if err != nil || !found {
			return false, nil
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
	if updated && !updating && !degraded {
		return true, nil
	}
	framework.Logf("Pool %s is still reporting (Updated: %v, Updating: %v, Degraded: %v)", name, updated, updating, degraded)
	return false, nil
}

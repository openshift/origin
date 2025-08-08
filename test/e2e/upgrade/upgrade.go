package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortests/network/disruptioningress"

	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/pborman/uuid"
	v1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
	"k8s.io/kubernetes/test/e2e/upgrades/apps"
	"k8s.io/kubernetes/test/e2e/upgrades/node"

	"github.com/openshift/origin/test/e2e/upgrade/adminack"
	"github.com/openshift/origin/test/e2e/upgrade/dns"
	"github.com/openshift/origin/test/e2e/upgrade/manifestdelete"
	"github.com/openshift/origin/test/extended/prometheus"
	"github.com/openshift/origin/test/extended/util/disruption"
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
		&manifestdelete.UpgradeTest{},

		&node.SecretUpgradeTest{},
		&apps.ReplicaSetUpgradeTest{},
		&apps.StatefulSetUpgradeTest{},
		&apps.DeploymentUpgradeTest{},
		&apps.JobUpgradeTest{},
		&node.ConfigMapUpgradeTest{},
		&apps.DaemonSetUpgradeTest{},
		&prometheus.ImagePullsAreFast{},
		&prometheus.MetricsAvailableAfterUpgradeTest{},
		&dns.UpgradeTest{},
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
const defaultCVOUpdateAckTimeout = 2 * time.Minute

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

	g.It("Cluster should be upgradeable before beginning upgrade [Early][Suite:upgrade]", func() {
		config, err := framework.LoadConfig()
		framework.ExpectNoError(err)
		client := configv1client.NewForConfigOrDie(config)
		err = checkUpgradeability(client)
		framework.ExpectNoError(err)
	})

	g.It("All nodes should be in ready state [Early][Suite:upgrade]", func() {
		config, err := framework.LoadConfig()
		framework.ExpectNoError(err)
		client := kubernetes.NewForConfigOrDie(config)
		nodeList, err := client.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		var errMsgs []string
		for _, node := range nodeList.Items {
			for _, condition := range node.Status.Conditions {
				if condition.Type == v1.NodeReady && condition.Status != v1.ConditionTrue {
					err := fmt.Errorf("node/%s ready state is not true, status: %v, reason: %s", node.Name, condition.Status, condition.Reason)
					framework.Logf("Error: %v", err)
					errMsgs = append(errMsgs, err.Error())
				}
			}
		}

		if len(errMsgs) > 0 {
			combinedErr := fmt.Errorf("%s", strings.Join(errMsgs, "; "))
			o.Expect(combinedErr).NotTo(o.HaveOccurred())
		}
	})

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
					framework.ExpectNoError(
						clusterUpgrade(f, client, dynamicClient, config, upgCtx.Versions[i]),
						fmt.Sprintf("during upgrade to %s", upgCtx.Versions[i].NodeImage))
				}
			},
		)
	})

	g.It("Cluster should be upgradeable after finishing upgrade [Late][Suite:upgrade]", func() {
		config, err := framework.LoadConfig()
		framework.ExpectNoError(err)
		client := configv1client.NewForConfigOrDie(config)
		var lastErr error
		err = wait.PollImmediate(1*time.Second, 30*time.Second, func() (bool, error) {
			if err := checkUpgradeability(client); err != nil {
				lastErr = err
				framework.Logf("Upgradeability check failed, retrying: %v", err)
				return false, nil // retry on error
			}
			return true, nil
		})
		if err != nil && lastErr != nil {
			err = lastErr
		}
		framework.ExpectNoError(err)
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

func checkUpgradeability(configClient configv1client.Interface) error {
	ctx := context.TODO()
	cv, err := configClient.ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
	if err != nil {
		return err
	}

	if cv.Spec.DesiredUpdate != nil {
		if cv.Status.ObservedGeneration != cv.Generation {
			return fmt.Errorf("cluster may be in the process of upgrading")
		}
		if len(cv.Status.History) > 0 && cv.Status.History[0].State != configv1.CompletedUpdate {
			return fmt.Errorf("cluster is still being upgraded: %s", versionString(*cv.Spec.DesiredUpdate))
		}
	}
	if c := findCondition(cv.Status.Conditions, configv1.OperatorDegraded); c != nil && c.Status == configv1.ConditionTrue {
		framework.Logf("A clusteroperator is degraded %v:\n%v\n", c.Message, clusterOperatorsForRendering(ctx, configClient))
		return fmt.Errorf("cluster is reporting a degraded condition: %v", c.Message)
	}
	if c := findCondition(cv.Status.Conditions, configv1.ClusterStatusConditionType("Failing")); c != nil && c.Status == configv1.ConditionTrue {
		framework.Logf("A clusteroperator is failing %v:\n%v\n", c.Message, clusterOperatorsForRendering(ctx, configClient))
		return fmt.Errorf("cluster is reporting a failing condition: %v", c.Message)
	}
	if c := findCondition(cv.Status.Conditions, configv1.OperatorProgressing); c == nil || c.Status != configv1.ConditionFalse {
		framework.Logf("A clusteroperator is progressing %v:\n%v\n", c.Message, clusterOperatorsForRendering(ctx, configClient))
		return fmt.Errorf("cluster must be reporting a progressing=false condition: %#v", c)
	}
	if c := findCondition(cv.Status.Conditions, configv1.OperatorAvailable); c == nil || c.Status != configv1.ConditionTrue {
		framework.Logf("A clusteroperator is not available %v:\n%v\n", c.Message, clusterOperatorsForRendering(ctx, configClient))
		return fmt.Errorf("cluster must be reporting an available=true condition: %#v", c)
	}

	_, ok := latestCompleted(cv.Status.History)
	if !ok {
		return fmt.Errorf("cluster has not rolled out a version yet")
	}

	return nil
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

	if err := checkUpgradeability(c); err != nil {
		return nil, err
	}

	cv, err := c.ConfigV1().ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	current, _ := latestCompleted(cv.Status.History)
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

	// ignore the failure here, we don't want this to fail the upgrade, we want it to fail this particular test.
	_ = disruption.RecordJUnit(
		f,
		"[bz-Routing] console is not available via ingress",
		func() (error, bool) {
			pollErr := wait.PollImmediateWithContext(context.TODO(), 1*time.Second, 10*time.Minute, func(ctx context.Context) (bool, error) {
				consoleSampler := disruptioningress.CreateConsoleRouteAvailableWithNewConnections(config)
				_, err := consoleSampler.CheckConnection(ctx)
				if err == nil {
					return true, nil
				}
				klog.Errorf("ingress is down: %v", err)
				return false, nil
			})
			return pollErr, false
		},
	)

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

	const (
		OVN = "OVNKubernetes"
		SDN = "OpenShiftSDN"
	)

	// Maximum duration is our maximum polling interval for upgrades
	var maximumDuration = 150 * time.Minute

	// defaultUpgradeLimit is the default for platforms not listed below.
	var defaultUpgradeLimit = 100.0 * time.Minute

	// The durations below were calculated from the P99 gathered from sippy's record of upgrade
	// durations[1]. We are only considering the tuple of platform and network type, not upgrade
	// type.  Micro upgrades are volatile because you may only get 1 or 2 components, or you may
	// get them all. The worst case of a micro upgrade is where all components get updated including
	// OS, which would be the same as a minor upgrade, so we use the minor threshold for all values.
	//
	// [1] https://raw.githubusercontent.com/openshift/sippy/f546164c18db9a4a930cc64f43bcdcd35509e767/scripts/sql/test-duration-stats.sql
	type limitLocator struct {
		Platform configv1.PlatformType
		Network  string
	}
	var upgradeDurationLimits = map[limitLocator]float64{
		{configv1.AWSPlatformType, OVN}:       85,
		{configv1.AWSPlatformType, SDN}:       95,
		{configv1.AzurePlatformType, OVN}:     100,
		{configv1.AzurePlatformType, SDN}:     100,
		{configv1.GCPPlatformType, OVN}:       90,
		{configv1.GCPPlatformType, SDN}:       75,
		{configv1.BareMetalPlatformType, OVN}: 80,
		{configv1.BareMetalPlatformType, SDN}: 70,
		{configv1.VSpherePlatformType, OVN}:   95,
		{configv1.VSpherePlatformType, SDN}:   70,
	}

	infra, err := c.ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	framework.ExpectNoError(err)
	network, err := c.ConfigV1().Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
	framework.ExpectNoError(err)
	platformType, err := platformidentification.GetJobType(context.TODO(), config)
	framework.ExpectNoError(err)

	// Default upgrade duration limit is our limit for configurations other than those
	// listed below.
	upgradeDurationLimit := defaultUpgradeLimit
	switch {
	case platformType.Architecture == platformidentification.ArchitectureS390:
		// s390 historically takes slightly under 100 minutes to upgrade. In 4.14, this regressed
		// to be 30 minutes longer. https://issues.redhat.com/browse/OCPBUGS-13059
		upgradeDurationLimit = 130 * time.Minute
	case platformType.Architecture == platformidentification.ArchitecturePPC64le:
		// ppc appears to take just over 75 minutes, let's lock it into that value, so we don't
		// get worse.
		upgradeDurationLimit = 80 * time.Minute
	case infra.Status.InfrastructureTopology == configv1.SingleReplicaTopologyMode:
		// single node takes a lot less since there's one node
		upgradeDurationLimit = 65 * time.Minute
	default:
		locator := limitLocator{infra.Status.PlatformStatus.Type, network.Status.NetworkType}

		if limit, ok := upgradeDurationLimits[locator]; ok {
			upgradeDurationLimit = time.Duration(limit) * time.Minute
		}
	}

	// If we are running single node on an AWS metal instance, we need to use a higher timeout because metal instances take significantly longer
	if infra.Status.InfrastructureTopology == configv1.SingleReplicaTopologyMode && infra.Status.PlatformStatus.Type == configv1.AWSPlatformType {
		nodes, err := kubeClient.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		framework.ExpectNoError(err)

		// If any nodes are AWS metal instances, override the SNO time limit with the AWS Metal limit
		for _, node := range nodes.Items {
			if strings.Contains(node.Labels["node.kubernetes.io/instance-type"], "metal") {
				upgradeDurationLimit = 95 * time.Minute
				break
			}
		}
	}

	framework.Logf("Upgrade time limit set as %0.2f", upgradeDurationLimit.Minutes())

	framework.Logf("Starting upgrade to version=%s image=%s attempt=%s", version.Version.String(), version.NodeImage, uid)
	recordClusterEvent(kubeClient, uid, "Upgrade", monitorapi.UpgradeStartedReason, fmt.Sprintf("version/%s image/%s", version.Version.String(), version.NodeImage), false)

	// decide whether to abort at a percent
	abortAt := upgradeAbortAt
	switch abortAt {
	case 0:
		// no abort
	case upgradeAbortAtRandom:
		abortAt = int(rand.Int31n(100) + 1)
		maximumDuration *= 2
		upgradeDurationLimit *= 2
		framework.Logf("Upgrade will be aborted and the cluster will roll back to the current version after %d%% of operators have upgraded (picked randomly)", abortAt)
	default:
		maximumDuration *= 2
		upgradeDurationLimit *= 2
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

	//used below in separate paths
	clusterCompletesUpgradeTestName := "[sig-cluster-lifecycle] Cluster completes upgrade"

	// trigger the update and record verification as an independent step
	if err := disruption.RecordJUnit(
		f,
		"[sig-cluster-lifecycle] Cluster version operator acknowledges upgrade",
		func() (error, bool) {
			cv, err := c.ConfigV1().ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})
			if err != nil {
				return err, false
			}
			original = cv
			monitor.oldVersion = original.Status.Desired.Version

			desired = configv1.Update{
				Version: version.Version.String(),
				Image:   version.NodeImage,
				Force:   true,
			}
			updateJSON, err := json.Marshal(desired)
			if err != nil {
				return fmt.Errorf("marshal ClusterVersion patch: %v", err), false
			}
			patch := []byte(fmt.Sprintf(`{"spec":{"desiredUpdate": %s}}`, updateJSON))
			cv, err = c.ConfigV1().ClusterVersions().Patch(context.Background(), original.ObjectMeta.Name, types.MergePatchType, patch, metav1.PatchOptions{})
			if err != nil {
				return err, false
			}
			updated = cv
			var observedGeneration int64

			var cvoAckTimeout time.Duration
			switch infra.Status.PlatformStatus.Type {
			// Timeout was previously 2min, bumped for metal/openstack while work underway on https://bugzilla.redhat.com/show_bug.cgi?id=2071998
			case configv1.BareMetalPlatformType:
				cvoAckTimeout = 10 * time.Minute
			case configv1.OpenStackPlatformType:
				cvoAckTimeout = 4 * time.Minute
			default:
				cvoAckTimeout = defaultCVOUpdateAckTimeout
			}

			start := time.Now()
			// wait until the cluster acknowledges the update
			if err := wait.PollImmediate(5*time.Second, cvoAckTimeout, func() (bool, error) {
				cv, _, err := monitor.Check(updated.Generation, desired)
				if err != nil || cv == nil {
					return false, err
				}
				observedGeneration = cv.Status.ObservedGeneration
				return cv.Status.ObservedGeneration >= updated.Generation, nil

			}); err != nil {
				return fmt.Errorf(
					"Timed out waiting for cluster to acknowledge upgrade: %v; observedGeneration: %d; updated.Generation: %d",
					err, observedGeneration, updated.Generation), false
			}
			// We allow extra time on a couple platforms above, if we're over the default we'll flake this test
			// to allow insight into how often we're hitting this problem and when the issue is fixed.
			timeToAck := time.Now().Sub(start)
			if timeToAck > defaultCVOUpdateAckTimeout {
				return fmt.Errorf("CVO took %s to acknowledge upgrade (> %s), flaking test", timeToAck, defaultCVOUpdateAckTimeout), true
			}
			return nil, false
		},
	); err != nil {
		//before returning the err force a failure for completes upgrade which follows this test
		disruption.RecordJUnit(f, clusterCompletesUpgradeTestName, func() (error, bool) {
			framework.Logf("Cluster version operator failed to acknowledge upgrade request")
			return fmt.Errorf("Cluster did not complete upgrade: operator failed to acknowledge upgrade request"), false
		})
		recordClusterEvent(kubeClient, uid, "Upgrade", monitorapi.UpgradeFailedReason, fmt.Sprintf("failed to acknowledge version: %v", err), true)
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go monitor.Disrupt(ctx, kubeClient, upgradeDisruptRebootPolicy)

	// observe the upgrade, taking action as necessary
	if err := disruption.RecordJUnit(
		f,
		clusterCompletesUpgradeTestName,
		func() (error, bool) {
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

				if !aborted && monitor.ShouldUpgradeAbort(abortAt, desired) {
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
					recordClusterEvent(kubeClient, uid, "Upgrade", monitorapi.UpgradeRollbackReason, fmt.Sprintf("version/%s image/%s", original.Status.Desired.Version, original.Status.Desired.Version), false)
					aborted = true
					action = "aborted upgrade"
					return false, nil
				}

				return monitor.Reached(cv, desired)

			}); err != nil {
				if lastMessage != "" {
					return fmt.Errorf("Cluster did not complete %s: %v: %s", action, err, lastMessage), false
				}
				return fmt.Errorf("Cluster did not complete %s: %v", action, err), false
			}

			framework.Logf("Completed %s to %s", action, versionString(desired))
			recordClusterEvent(kubeClient, uid, "Upgrade", monitorapi.UpgradeVersionReason, fmt.Sprintf("version/%s image/%s", updated.Status.Desired.Version, updated.Status.Desired.Version), false)

			// record whether the cluster was fast or slow upgrading.  Don't fail the test, we still want signal on the actual tests themselves.
			upgradeEnded := time.Now()
			upgradeDuration := upgradeEnded.Sub(upgradeStarted)
			testCaseName := fmt.Sprintf("[sig-cluster-lifecycle] cluster upgrade should complete in a reasonable time")
			failure := ""
			if upgradeDuration > upgradeDurationLimit {
				failure = fmt.Sprintf("%s to %s took too long: %0.2f minutes (for this platform/network, it should be less than %0.2f minutes)", action, versionString(desired), upgradeDuration.Minutes(), upgradeDurationLimit.Minutes())
			}
			disruption.RecordJUnitResult(f, testCaseName, upgradeDuration, failure)

			return nil, false
		},
	); err != nil {
		recordClusterEvent(kubeClient, uid, "Upgrade", monitorapi.UpgradeFailedReason, fmt.Sprintf("failed to reach cluster version: %v", err), true)
		return err
	}

	var errMasterUpdating error
	if err := disruption.RecordJUnit(
		f,
		"[sig-mco] Machine config pools complete upgrade",
		func() (error, bool) {
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
				return fmt.Errorf("Pools did not complete upgrade: %v", err), false
			}
			framework.Logf("All pools completed upgrade")
			return nil, false
		},
	); err != nil {
		recordClusterEvent(kubeClient, uid, "Upgrade", monitorapi.UpgradeFailedReason, fmt.Sprintf("failed to upgrade nodes: %v", err), true)
		return err
	}

	if errMasterUpdating != nil {
		recordClusterEvent(kubeClient, uid, "Upgrade", monitorapi.UpgradeFailedReason, fmt.Sprintf("master was updating after cluster version reached level: %v", errMasterUpdating), true)
		return errMasterUpdating
	}

	if err := disruption.RecordJUnit(
		f,
		"[sig-cluster-lifecycle] ClusterOperators are available and not degraded after upgrade",
		func() (error, bool) {
			if err := operator.WaitForOperatorsToSettle(context.TODO(), c, 5); err != nil {
				return err, false
			}
			return nil, false
		},
	); err != nil {
		recordClusterEvent(kubeClient, uid, "Upgrade", monitorapi.UpgradeFailedReason, fmt.Sprintf("failed to settle operators: %v", err), true)
		return err
	}

	recordClusterEvent(kubeClient, uid, "Upgrade", monitorapi.UpgradeCompleteReason, fmt.Sprintf("version/%s image/%s", updated.Status.Desired.Version, updated.Status.Desired.Image), false)
	return nil
}

// recordClusterEvent attempts to record an event to the cluster to indicate actions taken during an
// upgrade for timeline review.
func recordClusterEvent(client kubernetes.Interface, uid, action string, reason monitorapi.IntervalReason, note string, warning bool) {
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
		Reason:              reason.String(),
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

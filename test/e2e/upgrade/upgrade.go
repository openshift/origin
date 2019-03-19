package upgrade

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/chaosmonkey"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/ginkgowrapper"
	"k8s.io/kubernetes/test/e2e/upgrades"
	apps "k8s.io/kubernetes/test/e2e/upgrades/apps"
	"k8s.io/kubernetes/test/utils/junit"

	g "github.com/onsi/ginkgo"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
)

func AllTests() []upgrades.Test {
	return []upgrades.Test{
		&upgrades.ServiceUpgradeTest{},
		&upgrades.SecretUpgradeTest{},
		&apps.ReplicaSetUpgradeTest{},
		&apps.StatefulSetUpgradeTest{},
		&apps.DeploymentUpgradeTest{},
		&apps.JobUpgradeTest{},
		&upgrades.ConfigMapUpgradeTest{},
		// &upgrades.HPAUpgradeTest{},
		//&storage.PersistentVolumeUpgradeTest{},
		&apps.DaemonSetUpgradeTest{},
		// &upgrades.IngressUpgradeTest{},
		// &upgrades.AppArmorUpgradeTest{},
		// &upgrades.MySqlUpgradeTest{},
		// &upgrades.EtcdUpgradeTest{},
		// &upgrades.CassandraUpgradeTest{},
	}
}

var upgradeTests = []upgrades.Test{}

func SetTests(tests []upgrades.Test) {
	upgradeTests = tests
}

var _ = g.Describe("[Disruptive]", func() {
	f := framework.NewDefaultFramework("cluster-upgrade")
	f.SkipNamespaceCreation = true
	f.SkipPrivilegedPSPBinding = true

	g.Describe("Cluster upgrade", func() {
		g.It("should maintain a functioning cluster [Feature:ClusterUpgrade]", func() {
			// Create the frameworks here because we can only create them
			// in a "Describe".
			testFrameworks := createUpgradeFrameworks(upgradeTests)

			config, err := framework.LoadConfig()
			framework.ExpectNoError(err)
			client := configv1client.NewForConfigOrDie(config)

			upgCtx, err := getUpgradeContext(client, framework.TestContext.UpgradeTarget, framework.TestContext.UpgradeImage)
			framework.ExpectNoError(err, "determining what to upgrade to version=%s image=%s", framework.TestContext.UpgradeTarget, framework.TestContext.UpgradeImage)

			testSuite := &junit.TestSuite{Name: "Cluster upgrade"}
			clusterUpgradeTest := &junit.TestCase{Name: "cluster-upgrade", Classname: "upgrade_tests"}
			testSuite.TestCases = append(testSuite.TestCases, clusterUpgradeTest)
			upgradeFunc := func() {
				start := time.Now()
				defer finalizeUpgradeTest(start, clusterUpgradeTest)
				framework.ExpectNoError(clusterUpgrade(client, upgCtx.Versions[1]), "during upgrade")
			}
			runUpgradeSuite(f, upgradeTests, testFrameworks, testSuite, upgCtx, upgrades.ClusterUpgrade, upgradeFunc)
		})
	})
})

type chaosMonkeyAdapter struct {
	test        upgrades.Test
	testReport  *junit.TestCase
	framework   *framework.Framework
	upgradeType upgrades.UpgradeType
	upgCtx      upgrades.UpgradeContext
}

func (cma *chaosMonkeyAdapter) Test(sem *chaosmonkey.Semaphore) {
	start := time.Now()
	var once sync.Once
	ready := func() {
		once.Do(func() {
			sem.Ready()
		})
	}
	defer finalizeUpgradeTest(start, cma.testReport)
	defer ready()
	if skippable, ok := cma.test.(upgrades.Skippable); ok && skippable.Skip(cma.upgCtx) {
		g.By("skipping test " + cma.test.Name())
		cma.testReport.Skipped = "skipping test " + cma.test.Name()
		return
	}

	cma.framework.BeforeEach()
	cma.test.Setup(cma.framework)
	defer cma.test.Teardown(cma.framework)
	ready()
	cma.test.Test(cma.framework, sem.StopCh, cma.upgradeType)
}

func finalizeUpgradeTest(start time.Time, tc *junit.TestCase) {
	tc.Time = time.Since(start).Seconds()
	r := recover()
	if r == nil {
		return
	}

	switch r := r.(type) {
	case ginkgowrapper.FailurePanic:
		tc.Failures = []*junit.Failure{
			{
				Message: r.Message,
				Type:    "Failure",
				Value:   fmt.Sprintf("%s\n\n%s", r.Message, r.FullStackTrace),
			},
		}
	case ginkgowrapper.SkipPanic:
		tc.Skipped = fmt.Sprintf("%s:%d %q", r.Filename, r.Line, r.Message)
	default:
		tc.Errors = []*junit.Error{
			{
				Message: fmt.Sprintf("%v", r),
				Type:    "Panic",
				Value:   fmt.Sprintf("%v\n\n%s", r, debug.Stack()),
			},
		}
	}
}

func createUpgradeFrameworks(tests []upgrades.Test) map[string]*framework.Framework {
	nsFilter := regexp.MustCompile("[^[:word:]-]+") // match anything that's not a word character or hyphen
	testFrameworks := map[string]*framework.Framework{}
	for _, t := range tests {
		ns := nsFilter.ReplaceAllString(t.Name(), "-") // and replace with a single hyphen
		ns = strings.Trim(ns, "-")
		testFrameworks[t.Name()] = &framework.Framework{
			BaseName:                 ns,
			AddonResourceConstraints: make(map[string]framework.ResourceConstraint),
			Options: framework.FrameworkOptions{
				ClientQPS:   20,
				ClientBurst: 50,
			},
		}
	}
	return testFrameworks
}

func runUpgradeSuite(
	f *framework.Framework,
	tests []upgrades.Test,
	testFrameworks map[string]*framework.Framework,
	testSuite *junit.TestSuite,
	upgCtx *upgrades.UpgradeContext,
	upgradeType upgrades.UpgradeType,
	upgradeFunc func(),
) {
	cm := chaosmonkey.New(upgradeFunc)
	for _, t := range tests {
		f, ok := testFrameworks[t.Name()]
		if !ok {
			panic(fmt.Sprintf("can't find test framework for %q", t.Name()))
		}
		testCase := &junit.TestCase{
			Name:      t.Name(),
			Classname: "upgrade_tests",
		}
		testSuite.TestCases = append(testSuite.TestCases, testCase)
		cma := chaosMonkeyAdapter{
			test:        t,
			testReport:  testCase,
			framework:   f,
			upgradeType: upgradeType,
			upgCtx:      *upgCtx,
		}
		cm.Register(cma.Test)
	}

	start := time.Now()
	defer func() {
		testSuite.Update()
		testSuite.Time = time.Since(start).Seconds()
		if framework.TestContext.ReportDir != "" {
			fname := filepath.Join(framework.TestContext.ReportDir, fmt.Sprintf("junit_%supgrades.xml", framework.TestContext.ReportPrefix))
			f, err := os.Create(fname)
			if err != nil {
				return
			}
			defer f.Close()
			xml.NewEncoder(f).Encode(testSuite)
		}
	}()
	cm.Do()
}

func latestCompleted(history []configv1.UpdateHistory) (*configv1.Update, bool) {
	for _, version := range history {
		if version.State == configv1.CompletedUpdate {
			return &configv1.Update{Version: version.Version, Image: version.Image}, true
		}
	}
	return nil, false
}

func getUpgradeContext(c configv1client.Interface, upgradeTarget, upgradeImage string) (*upgrades.UpgradeContext, error) {
	if upgradeTarget == "[pause]" {
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
	if c := findCondition(cv.Status.Conditions, configv1.OperatorFailing); c != nil && c.Status == configv1.ConditionTrue {
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

	if len(upgradeTarget) == 0 && len(upgradeImage) == 0 {
		return upgCtx, nil
	}

	if (len(upgradeImage) > 0 && upgradeImage == current.Image) || (len(upgradeTarget) > 0 && upgradeTarget == current.Version) {
		return nil, fmt.Errorf("cluster is already at version %s", versionString(*current))
	}

	var next upgrades.VersionContext
	next.NodeImage = upgradeImage
	if len(upgradeTarget) > 0 {
		nextVer, err := version.ParseSemantic(upgradeTarget)
		if err != nil {
			return nil, err
		}
		next.Version = *nextVer
	}
	upgCtx.Versions = append(upgCtx.Versions, next)

	return upgCtx, nil
}

func clusterUpgrade(c configv1client.Interface, version upgrades.VersionContext) error {
	fmt.Fprintf(os.Stderr, "\n\n\n")
	defer func() { fmt.Fprintf(os.Stderr, "\n\n\n") }()

	if version.NodeImage == "[pause]" {
		framework.Logf("Running a dry-run upgrade test")
		time.Sleep(2 * time.Minute)
		return nil
	}

	framework.Logf("Starting upgrade to version=%s image=%s", version.Version.String(), version.NodeImage)
	cv, err := c.ConfigV1().ClusterVersions().Get("version", metav1.GetOptions{})
	if err != nil {
		return err
	}
	desired := configv1.Update{
		Version: version.Version.String(),
		Image:   version.NodeImage,
	}
	cv.Spec.DesiredUpdate = &desired
	updated, err := c.ConfigV1().ClusterVersions().Update(cv)
	if err != nil {
		return err
	}

	var lastCV *configv1.ClusterVersion
	if err := wait.PollImmediate(5*time.Second, 2*time.Minute, func() (bool, error) {
		cv, err := c.ConfigV1().ClusterVersions().Get("version", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		lastCV = cv
		if cv.Status.ObservedGeneration > updated.Generation {
			if cv.Spec.DesiredUpdate == nil || desired != *cv.Spec.DesiredUpdate {
				return false, fmt.Errorf("desired cluster version was changed by someone else: %v", cv.Spec.DesiredUpdate)
			}
		}
		return cv.Status.ObservedGeneration == updated.Generation, nil
	}); err != nil {
		if lastCV != nil {
			data, _ := json.MarshalIndent(lastCV, "", "  ")
			framework.Logf("Current cluster version:\n%s", data)
		}
		return fmt.Errorf("Cluster did not acknowledge request to upgrade in a reasonable time: %v", err)
	}

	framework.Logf("Cluster version operator acknowledged upgrade request")

	if err := wait.PollImmediate(5*time.Second, 45*time.Minute, func() (bool, error) {
		cv, err := c.ConfigV1().ClusterVersions().Get("version", metav1.GetOptions{})
		if err != nil {
			framework.Logf("unable to retrieve cluster version during upgrade: %v", err)
			return false, nil
		}
		lastCV = cv
		if cv.Status.ObservedGeneration > updated.Generation {
			if cv.Spec.DesiredUpdate == nil || desired != *cv.Spec.DesiredUpdate {
				return false, fmt.Errorf("desired cluster version was changed by someone else: %v", cv.Spec.DesiredUpdate)
			}
		}

		if c := findCondition(cv.Status.Conditions, configv1.OperatorFailing); c != nil {
			if c.Status == configv1.ConditionTrue {
				framework.Logf("cluster upgrade is failing: %v", c.Message)
			}
		}

		if target, ok := latestCompleted(cv.Status.History); !ok || !equivalentUpdates(*target, cv.Status.Desired) {
			return false, nil
		}

		if c := findCondition(cv.Status.Conditions, configv1.OperatorAvailable); c != nil {
			if c.Status != configv1.ConditionTrue {
				return false, fmt.Errorf("cluster version was Available=false after completion: %v", cv.Status.Conditions)
			}
		}
		if c := findCondition(cv.Status.Conditions, configv1.OperatorProgressing); c != nil {
			if c.Status == configv1.ConditionTrue {
				return false, fmt.Errorf("cluster version was Progressing=true after completion: %v", cv.Status.Conditions)
			}
		}
		if c := findCondition(cv.Status.Conditions, configv1.OperatorFailing); c != nil {
			if c.Status == configv1.ConditionTrue {
				return false, fmt.Errorf("cluster version was Failing=true after completion: %v", cv.Status.Conditions)
			}
		}

		return true, nil
	}); err != nil {
		if lastCV != nil {
			data, _ := json.MarshalIndent(lastCV, "", "  ")
			framework.Logf("Cluster version:\n%s", data)
		}
		if coList, err := c.ConfigV1().ClusterOperators().List(metav1.ListOptions{}); err == nil {
			buf := &bytes.Buffer{}
			tw := tabwriter.NewWriter(buf, 0, 2, 1, ' ', 0)
			fmt.Fprintf(tw, "NAME\tA F P\tMESSAGE\n")
			for _, item := range coList.Items {
				fmt.Fprintf(tw,
					"%s\t%s %s %s\t%s\n",
					item.Name,
					findConditionShortStatus(item.Status.Conditions, configv1.OperatorAvailable),
					findConditionShortStatus(item.Status.Conditions, configv1.OperatorFailing),
					findConditionShortStatus(item.Status.Conditions, configv1.OperatorProgressing),
					findConditionMessage(item.Status.Conditions, configv1.OperatorProgressing),
				)
			}
			tw.Flush()
			framework.Logf("Cluster operators:\n%s", buf.String())
		}

		return fmt.Errorf("Cluster did not complete upgrade: %v", err)
	}

	framework.Logf("Completed upgrade to %s", versionString(desired))
	return nil
}

func findConditionShortStatus(conditions []configv1.ClusterOperatorStatusCondition, name configv1.ClusterStatusConditionType) string {
	if c := findCondition(conditions, name); c != nil {
		switch c.Status {
		case configv1.ConditionTrue:
			return "T"
		case configv1.ConditionFalse:
			return "F"
		default:
			return "U"
		}
	}
	return " "
}

func findConditionMessage(conditions []configv1.ClusterOperatorStatusCondition, name configv1.ClusterStatusConditionType) string {
	if c := findCondition(conditions, name); c != nil {
		return c.Message
	}
	return ""
}

func findCondition(conditions []configv1.ClusterOperatorStatusCondition, name configv1.ClusterStatusConditionType) *configv1.ClusterOperatorStatusCondition {
	for i := range conditions {
		if name == conditions[i].Type {
			return &conditions[i]
		}
	}
	return nil
}

func equivalentUpdates(a, b configv1.Update) bool {
	if len(a.Image) > 0 && len(b.Image) > 0 {
		return a.Image == b.Image
	}
	if len(a.Version) > 0 && len(b.Version) > 0 {
		return a.Version == b.Version
	}
	return false
}

func versionString(update configv1.Update) string {
	switch {
	case len(update.Version) > 0 && len(update.Image) > 0:
		return fmt.Sprintf("%s (%s)", update.Version, update.Image)
	case len(update.Image) > 0:
		return update.Image
	case len(update.Version) > 0:
		return update.Version
	default:
		return "<empty>"
	}
}

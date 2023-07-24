package main

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/synthetictests"
	"github.com/openshift/origin/pkg/test/ginkgo"
	"github.com/openshift/origin/test/e2e/upgrade"
	"k8s.io/kubectl/pkg/util/templates"
)

// upgradeSuites are all known upgrade test suites this binary should run
var upgradeSuites = testSuites{
	{
		TestSuite: ginkgo.TestSuite{
			Name: "all",
			Description: templates.LongDesc(`
		Run all tests.
		`),
			Matches: func(name string) bool {
				if isStandardEarlyTest(name) {
					return true
				}
				return strings.Contains(name, "[Feature:ClusterUpgrade]") && !strings.Contains(name, "[Suite:k8s]")
			},
			TestTimeout:         240 * time.Minute,
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(synthetictests.SystemUpgradeEventInvariants),
		},
		PreSuite: upgradeTestPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "platform",
			Description: templates.LongDesc(`
		Run only the tests that verify the platform remains available.
		`),
			Matches: func(name string) bool {
				if isStandardEarlyTest(name) {
					return true
				}
				return strings.Contains(name, "[Feature:ClusterUpgrade]") && !strings.Contains(name, "[Suite:k8s]")
			},
			TestTimeout:         240 * time.Minute,
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(synthetictests.SystemUpgradeEventInvariants),
		},
		PreSuite: upgradeTestPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "none",
			Description: templates.LongDesc(`
	Don't run disruption tests.
		`),
			Matches: func(name string) bool {
				if isStandardEarlyTest(name) {
					return true
				}
				return strings.Contains(name, "[Feature:ClusterUpgrade]") && !strings.Contains(name, "[Suite:k8s]")
			},
			TestTimeout:         240 * time.Minute,
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(synthetictests.SystemUpgradeEventInvariants),
		},
		PreSuite: upgradeTestPreSuite,
	},
}

// upgradeTestPreSuite validates the test options and gathers data useful prior to launching the upgrade and it's
// related tests.
func upgradeTestPreSuite(o *RunSuiteOptions) error {
	return o.UpgradeTestPreSuite()
}

// upgradeTestPreTest uses variables set at suite execution time to prepare the upgrade
// test environment in process (setting constants in the upgrade packages).
func upgradeTestPreTest() error {
	value := os.Getenv("TEST_UPGRADE_OPTIONS")
	if len(value) == 0 {
		return nil
	}

	var opt UpgradeOptions
	if err := json.Unmarshal([]byte(value), &opt); err != nil {
		return err
	}
	SetUpgradeGlobalsFromTestOptions(opt.TestOptions)
	upgrade.SetToImage(opt.ToImage)
	switch opt.Suite {
	case "none":
		return filterUpgrade(upgrade.NoTests(), func(string) bool { return true })
	case "platform":
		return filterUpgrade(upgrade.AllTests(), func(name string) bool {
			return false
		})
	default:
		return filterUpgrade(upgrade.AllTests(), func(string) bool { return true })
	}
}

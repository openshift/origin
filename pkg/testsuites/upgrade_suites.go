package testsuites

import (
	"strings"
	"time"

	"github.com/openshift/origin/pkg/test/ginkgo"
	"k8s.io/kubectl/pkg/util/templates"
)

func UpgradeTestSuites() []*ginkgo.TestSuite {
	copied := make([]*ginkgo.TestSuite, 0, len(upgradeSuites))
	for i := range upgradeSuites {
		curr := upgradeSuites[i]
		copied = append(copied, &curr)
	}
	return copied
}

// upgradeSuites are all known upgrade test suites this binary should run
var upgradeSuites = []ginkgo.TestSuite{
	{
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
		TestTimeout: 240 * time.Minute,
	},
	{
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
		TestTimeout: 240 * time.Minute,
	},
	{
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
		TestTimeout: 240 * time.Minute,
	},
}

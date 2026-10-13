package testsuites

import (
	"time"

	"k8s.io/kubectl/pkg/util/templates"

	"github.com/openshift/origin/pkg/test/ginkgo"
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
		Qualifiers: []string{
			withStandardEarlyTests(`name.contains("[Feature:ClusterUpgrade]") && !name.contains("[Suite:k8s]")`),
		},
		TestTimeout: 240 * time.Minute,
	},
	{
		Name: "platform",
		Description: templates.LongDesc(`
		Run only the tests that verify the platform remains available.
		`),
		Qualifiers: []string{
			withStandardEarlyTests(`name.contains("[Feature:ClusterUpgrade]") && !name.contains("[Suite:k8s]")`),
		},
		TestTimeout: 240 * time.Minute,
	},
	{
		Name: "none",
		Description: templates.LongDesc(`
	Don't run disruption tests.
		`),
		Qualifiers: []string{
			withStandardEarlyTests(`name.contains("[Feature:ClusterUpgrade]") && !name.contains("[Suite:k8s]")`),
		},
		TestTimeout: 240 * time.Minute,
	},
}

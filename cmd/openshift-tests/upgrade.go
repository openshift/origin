package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/pflag"

	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	"k8s.io/kubernetes/test/e2e/upgrades"

	"github.com/openshift/origin/pkg/test/ginkgo"
	"github.com/openshift/origin/test/e2e/upgrade"
	exutil "github.com/openshift/origin/test/extended/util"
)

// upgradeSuites are all known upgade test suites this binary should run
var upgradeSuites = []*ginkgo.TestSuite{
	{
		Name: "all",
		Description: templates.LongDesc(`
		Run all tests.
		`),
		Matches: func(name string) bool { return strings.Contains(name, "[Feature:ClusterUpgrade]") },

		Init: func() error { return filterUpgrade(upgrade.AllTests(), func(name string) bool { return true }) },
	},
}

type UpgradeOptions struct {
	Suite    string
	ToImage  string
	JUnitDir string
}

func (o *UpgradeOptions) ToEnv() string {
	out, err := json.Marshal(o)
	if err != nil {
		panic(err)
	}
	return string(out)
}

func initUpgrade(value string) error {
	if len(value) == 0 {
		return nil
	}
	var opt UpgradeOptions
	if err := json.Unmarshal([]byte(value), &opt); err != nil {
		return err
	}
	for _, suite := range upgradeSuites {
		if suite.Name == opt.Suite {
			exutil.TestContext.UpgradeTarget = ""
			exutil.TestContext.UpgradeImage = opt.ToImage
			exutil.TestContext.ReportDir = opt.JUnitDir
			if suite.Init != nil {
				return suite.Init()
			}
			return nil
		}
	}
	return fmt.Errorf("unrecognized upgrade info")
}

func filterUpgrade(tests []upgrades.Test, match func(string) bool) error {
	var scope []upgrades.Test
	for _, test := range tests {
		if match(test.Name()) {
			scope = append(scope, test)
		}
	}
	upgrade.SetTests(scope)
	return nil
}

func bindUpgradeOptions(opt *UpgradeOptions, flags *pflag.FlagSet) {
	flags.StringVar(&opt.ToImage, "to-image", opt.ToImage, "Specify the image to test an upgrade to.")
}

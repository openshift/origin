package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	"github.com/openshift/origin/pkg/test/ginkgo"
	_ "github.com/openshift/origin/test/e2e/dr"
)

// disasterRecoverySuites are all known disaster recovery test suites this binary should run
var disasterRecoverySuites = []*ginkgo.TestSuite{
	{
		Name: "all",
		Description: templates.LongDesc(`
		Run all tests.
		`),
		Matches: func(name string) bool {
			return strings.Contains(name, "[Feature:DisasterRecovery]")
		},
		TestTimeout: 120 * time.Minute,
	},
}

// DisasterRecoveryOptions lists all options for disaster recovery tests
type DisasterRecoveryOptions struct {
	Suite       string
	TestOptions []string
}

func (o *DisasterRecoveryOptions) OptionsMap() (map[string]string, error) {
	options := make(map[string]string)
	for _, option := range o.TestOptions {
		parts := strings.SplitN(option, "=", 2)
		switch {
		case len(parts) != 2, len(parts[0]) == 0:
			return nil, fmt.Errorf("test option %q is not valid, must be KEY=VALUE", option)
		}
		_, exists := options[parts[0]]
		if exists {
			return nil, fmt.Errorf("option %q declared twice", parts[0])
		}
		options[parts[0]] = parts[1]
	}
	return options, nil
}

func (o *DisasterRecoveryOptions) ToEnv() string {
	out, err := json.Marshal(o)
	if err != nil {
		panic(err)
	}
	return string(out)
}

func initDRSnapshotRestore(value string) error {
	if len(value) == 0 {
		return nil
	}
	var opt DisasterRecoveryOptions
	if err := json.Unmarshal([]byte(value), &opt); err != nil {
		return err
	}
	for _, suite := range disasterRecoverySuites {
		if suite.Name == opt.Suite {
			o, err := opt.OptionsMap()
			if err != nil {
				return err
			}
			if suite.Init != nil {
				return suite.Init(o)
			}
			return nil
		}
	}
	return fmt.Errorf("unrecognized test info")
}

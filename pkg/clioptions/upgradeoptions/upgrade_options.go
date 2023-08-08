package upgradeoptions

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openshift/origin/test/e2e/upgrade"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

type UpgradeOptions struct {
	Suite       string
	ToImage     string
	TestOptions []string
}

func NewUpgradeOptionsFromYAML(yaml string) (*UpgradeOptions, error) {
	if len(yaml) == 0 {
		return nil, nil
	}

	var opt UpgradeOptions
	if err := json.Unmarshal([]byte(yaml), &opt); err != nil {
		return nil, fmt.Errorf("failed to parse upgrade options from yaml: %w", err)
	}
	return &opt, nil
}

func (o *UpgradeOptions) ToEnv() string {
	if o == nil {
		return ""
	}

	out, err := json.Marshal(o)
	if err != nil {
		panic(err)
	}
	return string(out)
}

// SetUpgradeGlobals uses variables set at suite execution time to prepare the upgrade
// test environment in process (setting constants in the upgrade packages).
func (o *UpgradeOptions) SetUpgradeGlobals() error {
	// nothing to do
	if o == nil {
		return nil
	}

	for _, opt := range o.TestOptions {
		parts := strings.SplitN(opt, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("expected option of the form KEY=VALUE instead of %q", opt)
		}
		switch parts[0] {
		case "abort-at":
			if err := upgrade.SetUpgradeAbortAt(parts[1]); err != nil {
				return err
			}
		case "disrupt-reboot":
			if err := upgrade.SetUpgradeDisruptReboot(parts[1]); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unrecognized upgrade option: %s", parts[0])
		}
	}

	upgrade.SetToImage(o.ToImage)
	switch o.Suite {
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

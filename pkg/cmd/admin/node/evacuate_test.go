package node

import (
	"github.com/spf13/cobra"
	"strconv"
	"testing"
)

func TestEvacuateFlags(t *testing.T) {
	defaults := NewEvacuateOptions(nil)

	tests := map[string]struct {
		flagName   string
		defaultVal string
	}{
		"dry run": {
			flagName:   flagDryRun,
			defaultVal: strconv.FormatBool(defaults.DryRun),
		},
		"force": {
			flagName:   flagForce,
			defaultVal: strconv.FormatBool(defaults.Force),
		},
		"grace period": {
			flagName:   flagGracePeriod,
			defaultVal: strconv.FormatInt(defaults.GracePeriod, 10),
		},
	}

	cmd := NewCommandManageNode(nil, ManageNodeCommandName, ManageNodeCommandName, nil)
	for _, v := range tests {
		testFlag(cmd, v.flagName, v.defaultVal, t)
	}
}

func testFlag(cmd *cobra.Command, flagName string, defaultVal string, t *testing.T) {
	f := cmd.Flag(flagName)
	if f == nil {
		t.Fatalf("expected flag %s to be registered but found none", flagName)
	}

	if f.DefValue != defaultVal {
		t.Errorf("expected default value of %s for %s but found %s", defaultVal, flagName, f.DefValue)
	}
}

func TestEvacOptionsGracePeriod(t *testing.T) {
	opts := &EvacuateOptions{
		GracePeriod: 999,
	}

	// ensure delete options are created correctly
	deleteOptions := opts.makeDeleteOptions()
	if deleteOptions == nil {
		t.Fatalf("nil delete options were created")
	}
	if deleteOptions.GracePeriodSeconds == nil {
		t.Fatalf("delete options did not contain grace period %v", deleteOptions)
	}
	if *deleteOptions.GracePeriodSeconds != opts.GracePeriod {
		t.Errorf("expected %d grace period but found %d", opts.GracePeriod, *deleteOptions.GracePeriodSeconds)
	}
}

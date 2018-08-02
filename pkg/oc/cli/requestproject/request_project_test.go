package requestproject

import (
	"strconv"
	"testing"

	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
)

// TestRequestProjectDefaultFlags ensures that flags default values are set.
func TestRequestProjectDefaultFlags(t *testing.T) {

	tests := map[string]struct {
		flagName   string
		defaultVal string
	}{
		"display name": {
			flagName:   "display-name",
			defaultVal: "",
		},
		"description": {
			flagName:   "description",
			defaultVal: "",
		},
		"skip config write": {
			flagName:   "skip-config-write",
			defaultVal: strconv.FormatBool(false),
		},
	}

	cmd := NewCmdRequestProject("oc", nil, genericclioptions.NewTestIOStreamsDiscard())

	for _, v := range tests {
		f := cmd.Flag(v.flagName)
		if f == nil {
			t.Fatalf("expected flag %s to be registered but found none", v.flagName)
		}

		if f.DefValue != v.defaultVal {
			t.Errorf("expected default value of %s for %s but found %s", v.defaultVal, v.flagName, f.DefValue)
		}
	}
}

package set

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

func TestLocalAndDryRunFlags(t *testing.T) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	errout := &bytes.Buffer{}
	f := clientcmd.NewFactory(nil)
	setCmd := NewCmdSet("", f, in, out, errout)
	ensureLocalAndDryRunFlagsOnChildren(t, setCmd, "")
}

func ensureLocalAndDryRunFlagsOnChildren(t *testing.T, c *cobra.Command, prefix string) {
	for _, cmd := range c.Commands() {
		name := prefix + cmd.Name()
		if localFlag := cmd.Flag("local"); localFlag == nil {
			t.Errorf("Command %s does not implement the --local flag", name)
		}
		if dryRunFlag := cmd.Flag("dry-run"); dryRunFlag == nil {
			t.Errorf("Command %s does not implement the --dry-run flag", name)
		}
		ensureLocalAndDryRunFlagsOnChildren(t, cmd, name+".")
	}
}

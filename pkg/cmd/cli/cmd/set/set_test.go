package set

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

// localFlagExceptions is the list of commands (children of set) that do not
// yet implement the --local flag.
// FIXME: Remove commands from this list as the --local flag is implemented
var localFlagExceptions = sets.NewString(
	"build-hook",
	"env",
	"probe",
	"route-backends",
	"triggers",
	"volumes",
)

func TestLocalFlag(t *testing.T) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	errout := &bytes.Buffer{}
	f := clientcmd.NewFactory(nil)
	setCmd := NewCmdSet("", f, in, out, errout)
	ensureLocalFlagOnChildren(t, setCmd, "")
}

func ensureLocalFlagOnChildren(t *testing.T, c *cobra.Command, prefix string) {
	for _, cmd := range c.Commands() {
		name := prefix + cmd.Name()
		if localFlagExceptions.Has(name) {
			continue
		}
		if localFlag := cmd.Flag("local"); localFlag == nil {
			t.Errorf("Command %s does not implement the --local flag", name)
		}
		ensureLocalFlagOnChildren(t, cmd, name+".")
	}
}

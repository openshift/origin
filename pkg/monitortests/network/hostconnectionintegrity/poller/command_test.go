package poller

import (
	"bytes"
	"strings"
	"testing"

	"github.com/openshift/origin/pkg/cmd"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func TestCommandSuppressesInheritedVersionDuringExecution(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	versionPrinted := false
	root := &cobra.Command{
		Use:           "openshift-tests",
		SilenceErrors: true,
		PersistentPreRun: func(command *cobra.Command, args []string) {
			versionPrinted = true
			cmd.PrintVersion(command, args)
		},
	}
	disruption := &cobra.Command{Use: "disruption"}
	disruption.AddCommand(NewCommand(genericclioptions.IOStreams{Out: &stdout, ErrOut: &stderr}))
	root.AddCommand(disruption)
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"disruption", "watch-established-connections"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--my-node-name is required") {
		t.Fatalf("expected normal RunE validation, got %v", err)
	}
	if versionPrinted {
		t.Error("inherited version pre-run executed")
	}
	if stdout.Len() != 0 {
		t.Errorf("non-JSON stdout: %q", stdout.String())
	}
}

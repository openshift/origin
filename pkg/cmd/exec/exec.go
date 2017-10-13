package exec

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// NewExecShim returns a cobra command that shells out the specified target with exactly the same args
func NewExecShim(name, target string, knownArgs ...string) *cobra.Command {
	return &cobra.Command{
		Use:                name,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			execCmd := exec.Command(target, append(knownArgs, args...)...)
			execCmd.Stdin = os.Stdin
			execCmd.Stdout = os.Stdout
			execCmd.Stderr = os.Stderr
			if err := execCmd.Run(); err != nil {
				return err
			}
			return nil
		},
	}
}

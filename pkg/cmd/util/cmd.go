package util

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
)

func DefaultSubCommandRun(out io.Writer) func(c *cobra.Command, args []string) {
	return func(c *cobra.Command, args []string) {
		c.SetOutput(out)

		if len(args) > 0 {
			kcmdutil.CheckErr(kcmdutil.UsageError(c, fmt.Sprintf(`unknown command "%s"`, strings.Join(args, " "))))
		}

		c.Help()
	}
}

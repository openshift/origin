package version

import (
	"fmt"

	"github.com/openshift/origin/pkg/version"
	"github.com/spf13/cobra"
)

func NewCommandVersion(name string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("Command '%s' (main)", name),
		Long:  fmt.Sprintf("Command '%s' (main)", name),
		Run: func(c *cobra.Command, args []string) {
			fmt.Println("OpenShift", version.Get().String())
		},
	}
}

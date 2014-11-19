package auth

import (
	r "github.com/openshift/origin/pkg/cmd/resource"
	"github.com/spf13/cobra"
)

// Root command

func NewCmdLogout(resource string) *cobra.Command {
	return r.NewCmdRoot(resource)
}

package auth

import (
	"fmt"
	"os"

	p "github.com/openshift/origin/pkg/cmd/util/printer"
	"github.com/spf13/cobra"
)

// Root command

func NewCmdLogin(resource string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   resource,
		Short: fmt.Sprintf("Command '%s' (main)", resource),
		Long:  fmt.Sprintf("Command '%s' (main)", resource),
		Run: func(c *cobra.Command, args []string) {
			printer := p.TerminalPrinter{}

			login := c.Flags().Lookup("login").Value.String()
			printer.Printf("Logging in %s...", login)

			if password := c.Flags().Lookup("password").Value.String(); len(password) == 0 {
				promptForString("password")
			}
		},
	}

	cmd.Flags().StringP("login", "l", "", "OpenShift login (username or email)")
	cmd.Flags().StringP("password", "p", "", "OpenShift password")
	return cmd
}

func promptForString(field string) string {
	fmt.Printf("Please enter %s: ", field)
	var result string
	fmt.Fscan(os.Stdin, &result)
	return result
}

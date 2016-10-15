package cluster

import (
	"fmt"
	"io"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	ConsoleRecommendedName = "console"

	consoleLongDescription = `
Opens the OpenShift console of the local cluster in a web browser.

If you only want to get the URL of the console, use --url.
`

	cmdConsoleExample = `
  # Open the OpenShift console in a web browser
  %[1]s

  # Only show the console URL
  %[1]s --url
`
)

type ConsoleConfig struct {
	OnlyShowUrl bool
}

func NewCmdConsole(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	config := &ConsoleConfig{}

	cmd := &cobra.Command{
		Use:     name,
		Short:   "Open the OpenShift console in a web browser",
		Long:    consoleLongDescription,
		Example: fmt.Sprintf(cmdConsoleExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			err := config.Run(f, out)
			kcmdutil.CheckErr(err)
		},
	}
	cmd.Flags().BoolVar(&config.OnlyShowUrl, "url", false, "Only show the URL of the console")
	return cmd
}

func (c *ConsoleConfig) Run(f *clientcmd.Factory, out io.Writer) error {
	config, err := f.OpenShiftClientConfig.ClientConfig()
	if err != nil {
		return err
	}

	if (c.OnlyShowUrl) {
		_, err = fmt.Fprintf(out, config.Host)
		return err
	} else {
		return browser.OpenURL(config.Host)
	}
}

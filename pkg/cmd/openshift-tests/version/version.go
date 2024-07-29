package version

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/openshift/origin/pkg/cmd"
	"github.com/openshift/origin/pkg/version"
)

// NewVersionCommand prints out openshift-tests version information, in a similar style to other kube tools,
// e.g. https://github.com/openshift/kubernetes/blob/6892b57d65d25fb0588693bce3d338d8b8b1d2b4/cmd/kubeadm/app/cmd/version.go#L67
func NewVersionCommand(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Long:  "Report version information for openshift-tests",
		Short: "Report version information for openshift-tests",
		// Set persistent pre run to empty so we don't double print version info
		PersistentPreRun: cmd.NoPrintVersion,
		RunE: func(cmd *cobra.Command, args []string) error {
			v := version.Get()
			const flag = "output"
			of, err := cmd.Flags().GetString(flag)
			if err != nil {
				return errors.Wrapf(err, "error accessing flag %s for command %s", flag, cmd.Name())
			}
			switch of {
			case "":
				fmt.Fprintf(streams.Out, "openshift-tests version %s\n", v.GitVersion)
			case "short":
				fmt.Fprintf(streams.Out, "%s\n", v.GitVersion)
			case "yaml":
				y, err := yaml.Marshal(&v)
				if err != nil {
					return err
				}
				fmt.Fprintln(streams.Out, string(y))
			case "json":
				y, err := json.MarshalIndent(&v, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(streams.Out, string(y))
			default:
				return errors.Errorf("invalid output format: %s", of)
			}

			return nil
		},
	}
	cmd.Flags().StringP("output", "o", "", "Output format; available options are 'yaml', 'json' and 'short'")
	return cmd
}

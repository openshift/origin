package cmd

import (
	"fmt"
	"io"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kubectl "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/spf13/cobra"

	latest "github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/client"
	describe "github.com/openshift/origin/pkg/cmd/cli/describe"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

const (
	rollbackLong = `Revert part of an application back to a previous deployment.

When you run this command your deployment configuration will be updated to match
the provided deployment. By default only the pod and container configuration
will be changed and scaling or trigger settings will be left as-is. Note that
environment variables and volumes are included in rollbacks, so if you've
recently updated security credentials in your environment your previous
deployment may not have the correct values.

Any image triggers present in the rolled back configuration will be disabled
with a warning. This is to help prevent your rolled back deployment from being
replaced by a triggered deployment soon after your rollback. To re-enable the
triggers, use the 'deploy' command.

If you would like to review the outcome of the rollback, pass '--dry-run' to print
a human-readable representation of the updated deployment configuration instead of
executing the rollback. This is useful if you're not quite sure what the outcome
will be.`

	rollbackExample = `  // Perform a rollback
  $ %[1]s rollback deployment-1

  // See what the rollback will look like, but don't perform the rollback
  $ %[1]s rollback deployment-1 --dry-run

  // Perform the rollback manually by piping the JSON of the new config back to %[1]s
  $ %[1]s rollback deployment-1 --output=json | %[1]s update deploymentConfigs deployment -f -`
)

// NewCmdRollback creates a CLI rollback command.
func NewCmdRollback(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	rollback := &deployapi.DeploymentConfigRollback{
		Spec: deployapi.DeploymentConfigRollbackSpec{
			IncludeTemplate: true,
		},
	}

	cmd := &cobra.Command{
		Use:     "rollback DEPLOYMENT",
		Short:   "Revert part of an application back to a previous deployment",
		Long:    rollbackLong,
		Example: fmt.Sprintf(rollbackExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			// Validate arguments
			if len(args) == 0 || len(args[0]) == 0 {
				cmdutil.CheckErr(cmdutil.UsageError(cmd, "A deployment name is required."))
			}

			// Extract arguments
			format := cmdutil.GetFlagString(cmd, "output")
			template := cmdutil.GetFlagString(cmd, "template")
			dryRun := cmdutil.GetFlagBool(cmd, "dry-run")

			// Get globally provided stuff
			namespace, err := f.DefaultNamespace()
			cmdutil.CheckErr(err)
			oClient, kClient, err := f.Clients()
			cmdutil.CheckErr(err)

			// Set up the rollback config
			rollback.Spec.From.Name = args[0]

			// Make a helper and generate a rolled back config
			helper := newHelper(oClient, kClient)
			config, err := helper.Generate(namespace, rollback)
			cmdutil.CheckErr(err)

			// If this is a dry run, print and exit
			if dryRun {
				err := helper.Describe(config, out)
				cmdutil.CheckErr(err)
				return
			}

			// If an output format is specified, print and exit
			if len(format) > 0 {
				err := helper.Print(config, format, template, out)
				cmdutil.CheckErr(err)
				return
			}

			// Perform the rollback
			rolledback, err := helper.Update(config)
			cmdutil.CheckErr(err)

			// Notify the user of any disabled image triggers
			fmt.Fprintf(out, "#%d rolled back to %s\n", rolledback.LatestVersion, rollback.Spec.From.Name)
			for _, trigger := range rolledback.Triggers {
				disabled := []string{}
				if trigger.Type == deployapi.DeploymentTriggerOnImageChange && !trigger.ImageChangeParams.Automatic {
					disabled = append(disabled, trigger.ImageChangeParams.From.Name)
				}
				if len(disabled) > 0 {
					reenable := fmt.Sprintf("%s deploy %s --enable-triggers", fullName, rolledback.Name)
					fmt.Fprintf(cmd.Out(), "Warning: the following images triggers were disabled: %s\n  You can re-enable them with: %s\n", strings.Join(disabled, ","), reenable)
				}
			}
		},
	}

	cmd.Flags().BoolVar(&rollback.Spec.IncludeTriggers, "change-triggers", false, "Include the previous deployment's triggers in the rollback")
	cmd.Flags().BoolVar(&rollback.Spec.IncludeStrategy, "change-strategy", false, "Include the previous deployment's strategy in the rollback")
	cmd.Flags().BoolVar(&rollback.Spec.IncludeReplicationMeta, "change-scaling-settings", false, "Include the previous deployment's replicationController replica count and selector in the rollback")
	cmd.Flags().BoolP("dry-run", "d", false, "Instead of performing the rollback, describe what the rollback will look like in human-readable form")
	cmd.Flags().StringP("output", "o", "", "Instead of performing the rollback, print the updated deployment configuration in the specified format (json|yaml|template|templatefile)")
	cmd.Flags().StringP("template", "t", "", "Template string or path to template file to use when -o=template or -o=templatefile.")

	return cmd
}

// newHelper makes a hew helper using real clients.
func newHelper(oClient client.Interface, kClient kclient.Interface) *helper {
	return &helper{
		generateRollback: func(namespace string, config *deployapi.DeploymentConfigRollback) (*deployapi.DeploymentConfig, error) {
			return oClient.DeploymentConfigs(namespace).Rollback(config)
		},
		describe: func(config *deployapi.DeploymentConfig) (string, error) {
			describer := describe.NewDeploymentConfigDescriberForConfig(oClient, kClient, config)
			return describer.Describe(config.Namespace, config.Name)
		},
		updateConfig: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
			return oClient.DeploymentConfigs(namespace).Update(config)
		},
	}
}

// helper knows how to perform various rollback related tasks.
type helper struct {
	// generateRollback generates a rolled back config from the input config
	generateRollback func(namespace string, config *deployapi.DeploymentConfigRollback) (*deployapi.DeploymentConfig, error)
	// describe returns the describer output for config
	describe func(config *deployapi.DeploymentConfig) (string, error)
	// updateConfig persists config
	updateConfig func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
}

// Generate generates a rolled back DeploymentConfig.
func (r *helper) Generate(namespace string, config *deployapi.DeploymentConfigRollback) (*deployapi.DeploymentConfig, error) {
	return r.generateRollback(namespace, config)
}

// Describe describes a DeploymentConfig.
func (r *helper) Describe(config *deployapi.DeploymentConfig, out io.Writer) error {
	description, err := r.describe(config)
	if err != nil {
		return err
	}
	out.Write([]byte(description))
	return nil
}

// Print prints a deployment config in the specified format with the given
// template.
func (r *helper) Print(config *deployapi.DeploymentConfig, format, template string, out io.Writer) error {
	printer, _, err := kubectl.GetPrinter(format, template)
	if err != nil {
		return err
	}
	versionedPrinter := kubectl.NewVersionedPrinter(printer, kapi.Scheme, latest.Version)
	versionedPrinter.PrintObj(config, out)
	return nil
}

// Update persists the given DeploymentConfig.
func (r *helper) Update(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	return r.updateConfig(config.Namespace, config)
}

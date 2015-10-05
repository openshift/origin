package cmd

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kubectl "k8s.io/kubernetes/pkg/kubectl"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"

	latest "github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/client"
	describe "github.com/openshift/origin/pkg/cmd/cli/describe"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

const (
	rollbackLong = `
Revert an application back to a previous deployment

When you run this command your deployment configuration will be updated to
match a previous deployment. By default only the pod and container
configuration will be changed and scaling or trigger settings will be left as-
is. Note that environment variables and volumes are included in rollbacks, so
if you've recently updated security credentials in your environment your
previous deployment may not have the correct values.

Any image triggers present in the rolled back configuration will be disabled
with a warning. This is to help prevent your rolled back deployment from being
replaced by a triggered deployment soon after your rollback. To re-enable the
triggers, use the 'deploy' command.

If you would like to review the outcome of the rollback, pass '--dry-run' to print
a human-readable representation of the updated deployment configuration instead of
executing the rollback. This is useful if you're not quite sure what the outcome
will be.`

	rollbackExample = `  # Perform a rollback to the last successfully completed deployment for a deploymentconfig
  $ %[1]s rollback frontend

  # See what a rollback to version 3 will look like, but don't perform the rollback
  $ %[1]s rollback frontend --to-version=3 --dry-run

  # Perform a rollback to a specific deployment
  $ %[1]s rollback frontend-2

  # Perform the rollback manually by piping the JSON of the new config back to %[1]s
  $ %[1]s rollback frontend --output=json | %[1]s update deploymentConfigs deployment -f -`
)

// NewCmdRollback creates a CLI rollback command.
func NewCmdRollback(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	opts := &RollbackOptions{}
	cmd := &cobra.Command{
		Use:     "rollback (DEPLOYMENTCONFIG | DEPLOYMENT)",
		Short:   "Revert part of an application back to a previous deployment",
		Long:    rollbackLong,
		Example: fmt.Sprintf(rollbackExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if err := opts.Complete(f, args, out); err != nil {
				cmdutil.CheckErr(cmdutil.UsageError(cmd, err.Error()))
			}

			if err := opts.Validate(); err != nil {
				cmdutil.CheckErr(cmdutil.UsageError(cmd, err.Error()))
			}

			if err := opts.Run(); err != nil {
				cmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().BoolVar(&opts.IncludeTriggers, "change-triggers", false, "Include the previous deployment's triggers in the rollback")
	cmd.Flags().BoolVar(&opts.IncludeStrategy, "change-strategy", false, "Include the previous deployment's strategy in the rollback")
	cmd.Flags().BoolVar(&opts.IncludeScalingSettings, "change-scaling-settings", false, "Include the previous deployment's replicationController replica count and selector in the rollback")
	cmd.Flags().BoolVarP(&opts.DryRun, "dry-run", "d", false, "Instead of performing the rollback, describe what the rollback will look like in human-readable form")
	cmd.Flags().StringVarP(&opts.Format, "output", "o", "", "Instead of performing the rollback, print the updated deployment configuration in the specified format (json|yaml|template|templatefile)")
	cmd.Flags().StringVarP(&opts.Template, "template", "t", "", "Template string or path to template file to use when -o=template or -o=templatefile.")
	cmd.Flags().IntVar(&opts.DesiredVersion, "to-version", 0, "A config version to rollback to. Specifying version 0 is the same as omitting a version (the version will be auto-detected). This option is ignored when specifying a deployment.")

	return cmd
}

// RollbackOptions contains all the necessary state to perform a rollback.
type RollbackOptions struct {
	Namespace              string
	TargetName             string
	DesiredVersion         int
	Format                 string
	Template               string
	DryRun                 bool
	IncludeTriggers        bool
	IncludeStrategy        bool
	IncludeScalingSettings bool

	// out is a place to write user-facing output.
	out io.Writer
	// oc is an openshift client.
	oc client.Interface
	// kc is a kube client.
	kc kclient.Interface
	// getBuilder returns a new builder each time it is called. A
	// resource.Builder is stateful and isn't safe to reuse (e.g. across
	// resource types).
	getBuilder func() *resource.Builder
}

// Complete turns a partially defined RollbackActions into a solvent structure
// which can be validated and used for a rollback.
func (o *RollbackOptions) Complete(f *clientcmd.Factory, args []string, out io.Writer) error {
	// Extract basic flags.
	if len(args) == 1 {
		o.TargetName = args[0]
	}
	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}
	o.Namespace = namespace

	// Set up client based support.
	mapper, typer := f.Object()
	o.getBuilder = func() *resource.Builder {
		return resource.NewBuilder(mapper, typer, f.ClientMapperForCommand())
	}

	oClient, kClient, err := f.Clients()
	if err != nil {
		return err
	}
	o.oc = oClient
	o.kc = kClient

	o.out = out
	return nil
}

// Validate ensures that a RollbackOptions is valid and can be used to execute
// a rollback.
func (o *RollbackOptions) Validate() error {
	if len(o.TargetName) == 0 {
		return fmt.Errorf("a deployment or deploymentconfig name is required")
	}
	if o.DesiredVersion < 0 {
		return fmt.Errorf("the to version must be >= 0")
	}
	if o.out == nil {
		return fmt.Errorf("out must not be nil")
	}
	if o.oc == nil {
		return fmt.Errorf("oc must not be nil")
	}
	if o.kc == nil {
		return fmt.Errorf("kc must not be nil")
	}
	if o.getBuilder == nil {
		return fmt.Errorf("getBuilder must not be nil")
	} else {
		b := o.getBuilder()
		if b == nil {
			return fmt.Errorf("getBuilder must return a resource.Builder")
		}
	}
	return nil
}

// Run performs a rollback.
func (o *RollbackOptions) Run() error {
	// Get the resource referenced in the command args.
	obj, err := o.findResource(o.TargetName)
	if err != nil {
		return err
	}

	// Interpret the resource to resolve a target for rollback.
	var target *kapi.ReplicationController
	switch r := obj.(type) {
	case *kapi.ReplicationController:
		// A specific deployment was used.
		target = r
	case *deployapi.DeploymentConfig:
		// A deploymentconfig was used. Find the target deployment by the
		// specified version, or by a lookup of the last completed deployment if
		// no version was supplied.
		deployment, err := o.findTargetDeployment(r, o.DesiredVersion)
		if err != nil {
			return err
		}
		target = deployment
	}
	if target == nil {
		return fmt.Errorf("%s is not a valid deployment or deploymentconfig", o.TargetName)
	}

	// Set up the rollback and generate a new rolled back config.
	rollback := &deployapi.DeploymentConfigRollback{
		Spec: deployapi.DeploymentConfigRollbackSpec{
			From: kapi.ObjectReference{
				Name: target.Name,
			},
			IncludeTemplate:        true,
			IncludeTriggers:        o.IncludeTriggers,
			IncludeStrategy:        o.IncludeStrategy,
			IncludeReplicationMeta: o.IncludeScalingSettings,
		},
	}
	newConfig, err := o.oc.DeploymentConfigs(o.Namespace).Rollback(rollback)
	if err != nil {
		return err
	}

	// If this is a dry run, print and exit.
	if o.DryRun {
		describer := describe.NewDeploymentConfigDescriberForConfig(o.oc, o.kc, newConfig)
		description, err := describer.Describe(newConfig.Namespace, newConfig.Name)
		if err != nil {
			return err
		}
		o.out.Write([]byte(description))
		return nil
	}

	// If an output format is specified, print and exit.
	if len(o.Format) > 0 {
		printer, _, err := kubectl.GetPrinter(o.Format, o.Template)
		if err != nil {
			return err
		}
		versionedPrinter := kubectl.NewVersionedPrinter(printer, kapi.Scheme, latest.Version)
		versionedPrinter.PrintObj(newConfig, o.out)
		return nil
	}

	// Perform a real rollback.
	rolledback, err := o.oc.DeploymentConfigs(newConfig.Namespace).Update(newConfig)
	if err != nil {
		return err
	}

	// Print warnings about any image triggers disabled during the rollback.
	fmt.Fprintf(o.out, "#%d rolled back to %s\n", rolledback.LatestVersion, rollback.Spec.From.Name)
	for _, trigger := range rolledback.Triggers {
		disabled := []string{}
		if trigger.Type == deployapi.DeploymentTriggerOnImageChange && !trigger.ImageChangeParams.Automatic {
			disabled = append(disabled, trigger.ImageChangeParams.From.Name)
		}
		if len(disabled) > 0 {
			reenable := fmt.Sprintf("oc deploy %s --enable-triggers -n %s", rolledback.Name, o.Namespace)
			fmt.Fprintf(o.out, "Warning: the following images triggers were disabled: %s\n  You can re-enable them with: %s\n", strings.Join(disabled, ","), reenable)
		}
	}

	return nil
}

// findResource tries to find a deployment or deploymentconfig named
// targetName using a resource.Builder. For compatibility, if the resource
// name is unprefixed, treat it as an rc first and a dc second.
func (o *RollbackOptions) findResource(targetName string) (runtime.Object, error) {
	candidates := []string{}
	if strings.Index(targetName, "/") == -1 {
		candidates = append(candidates, "rc/"+targetName)
		candidates = append(candidates, "dc/"+targetName)
	} else {
		candidates = append(candidates, targetName)
	}
	var obj runtime.Object
	for _, name := range candidates {
		r := o.getBuilder().
			NamespaceParam(o.Namespace).
			ResourceTypeOrNameArgs(false, name).
			SingleResourceType().
			Do()
		if r.Err() != nil {
			return nil, r.Err()
		}
		resultObj, err := r.Object()
		if err != nil {
			// If the resource wasn't found, try another candidate.
			if kerrors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		obj = resultObj
		break
	}
	if obj == nil {
		return nil, fmt.Errorf("%s is not a valid deployment or deploymentconfig", targetName)
	}
	return obj, nil
}

// findTargetDeployment finds the deployment which is the rollback target by
// searching for deployments associated with config. If desiredVersion is >0,
// the deployment matching desiredVersion will be returned. If desiredVersion
// is <=0, the last completed deployment which is older than the config's
// version will be returned.
func (o *RollbackOptions) findTargetDeployment(config *deployapi.DeploymentConfig, desiredVersion int) (*kapi.ReplicationController, error) {
	// Find deployments for the config sorted by version descending.
	deployments, err := o.kc.ReplicationControllers(config.Namespace).List(deployutil.ConfigSelector(config.Name))
	if err != nil {
		return nil, err
	}
	sort.Sort(deployutil.ByLatestVersionDesc(deployments.Items))

	// Find the target deployment for rollback. If a version was specified,
	// use the version for a search. Otherwise, use the last completed
	// deployment.
	var target *kapi.ReplicationController
	for _, deployment := range deployments.Items {
		version := deployutil.DeploymentVersionFor(&deployment)
		if desiredVersion > 0 {
			if version == desiredVersion {
				target = &deployment
				break
			}
		} else {
			if version < config.LatestVersion && deployutil.DeploymentStatusFor(&deployment) == deployapi.DeploymentStatusComplete {
				target = &deployment
				break
			}
		}
	}
	if target == nil {
		return nil, fmt.Errorf("couldn't find deployment for rollback")
	}
	return target, nil
}

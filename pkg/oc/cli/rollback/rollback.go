package rollback

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/resource"

	appsv1 "github.com/openshift/api/apps/v1"
	appstypedclient "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	appsutil "github.com/openshift/origin/pkg/apps/util"
	"github.com/openshift/origin/pkg/oc/util/ocscheme"
)

var (
	rollbackLong = templates.LongDesc(`
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
		will be.`)

	rollbackExample = templates.Examples(`
		# Perform a rollback to the last successfully completed deployment for a deploymentconfig
	  %[1]s rollback frontend

	  # See what a rollback to version 3 will look like, but don't perform the rollback
	  %[1]s rollback frontend --to-version=3 --dry-run

	  # Perform a rollback to a specific deployment
	  %[1]s rollback frontend-2

	  # Perform the rollback manually by piping the JSON of the new config back to %[1]s
	  %[1]s rollback frontend -o json | %[1]s replace dc/frontend -f -

	  # Print the updated deployment configuration in JSON format instead of performing the rollback
	  %[1]s rollback frontend -o json`)
)

// RollbackOptions contains all the necessary state to perform a rollback.
type RollbackOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	Namespace              string
	TargetName             string
	DesiredVersion         int64
	Format                 string
	Template               string
	DryRun                 bool
	IncludeTriggers        bool
	IncludeStrategy        bool
	IncludeScalingSettings bool

	appsClient appstypedclient.AppsV1Interface
	kubeClient kubernetes.Interface

	// builder returns a new builder each time it is called. A
	// resource.Builder is stateful and isn't safe to reuse (e.g. across
	// resource types).
	builder func() *resource.Builder

	ToPrinter func(string) (printers.ResourcePrinter, error)

	genericclioptions.IOStreams
}

func NewRollbackOptions(streams genericclioptions.IOStreams) *RollbackOptions {
	return &RollbackOptions{
		PrintFlags: genericclioptions.NewPrintFlags("rolled back").WithTypeSetter(ocscheme.PrintingInternalScheme),
		IOStreams:  streams,
	}
}

// NewCmdRollback creates a CLI rollback command.
func NewCmdRollback(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	opts := NewRollbackOptions(streams)
	cmd := &cobra.Command{
		Use:     "rollback (DEPLOYMENTCONFIG | DEPLOYMENT)",
		Short:   "Revert part of an application back to a previous deployment",
		Long:    rollbackLong,
		Example: fmt.Sprintf(rollbackExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if err := opts.Complete(f, cmd, args, streams.Out); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := opts.Validate(); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := opts.Run(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	opts.PrintFlags.AddFlags(cmd)

	cmd.Flags().BoolVar(&opts.IncludeTriggers, "change-triggers", false, "If true, include the previous deployment's triggers in the rollback")
	cmd.Flags().BoolVar(&opts.IncludeStrategy, "change-strategy", false, "If true, include the previous deployment's strategy in the rollback")
	cmd.Flags().BoolVar(&opts.IncludeScalingSettings, "change-scaling-settings", false, "If true, include the previous deployment's replicationController replica count and selector in the rollback")
	cmd.Flags().BoolVarP(&opts.DryRun, "dry-run", "d", false, "Instead of performing the rollback, describe what the rollback will look like in human-readable form")
	cmd.MarkFlagFilename("template")

	cmd.Flags().Int64Var(&opts.DesiredVersion, "to-version", opts.DesiredVersion, "A config version to rollback to. Specifying version 0 is the same as omitting a version (the version will be auto-detected). This option is ignored when specifying a deployment.")

	return cmd
}

// Complete turns a partially defined RollbackActions into a solvent structure
// which can be validated and used for a rollback.
func (o *RollbackOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string, out io.Writer) error {
	// Extract basic flags.
	if len(args) == 1 {
		o.TargetName = args[0]
	}
	namespace, _, err := f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	o.Namespace = namespace

	// Set up client based support.
	o.builder = f.NewBuilder

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}

	o.appsClient, err = appstypedclient.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	o.kubeClient, err = kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	o.Format = kcmdutil.GetFlagString(cmd, "output")

	o.ToPrinter = func(message string) (printers.ResourcePrinter, error) {
		o.PrintFlags.NamePrintFlags.Operation = message
		if o.DryRun {
			o.PrintFlags.Complete("%s (dry run)")
		}

		return o.PrintFlags.ToPrinter()
	}

	return nil
}

// Validate ensures that a RollbackOptions is valid and can be used to execute
// a rollback.
func (o *RollbackOptions) Validate() error {
	if len(o.TargetName) == 0 {
		return fmt.Errorf("a deployment or deployment config name is required")
	}
	if o.DesiredVersion < 0 {
		return fmt.Errorf("the to version must be >= 0")
	}
	return nil
}

// Run performs a rollback.
func (o *RollbackOptions) Run() error {
	// Get the resource referenced in the command args.
	obj, mapping, err := o.findResource(o.TargetName)
	if err != nil {
		return err
	}

	configName := ""

	// Interpret the resource to resolve a target for rollback.
	var target *corev1.ReplicationController
	switch r := obj.(type) {
	case *corev1.ReplicationController:
		dcName := appsutil.DeploymentConfigNameFor(r)
		dc, err := o.appsClient.DeploymentConfigs(r.Namespace).Get(dcName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if dc.Spec.Paused {
			return fmt.Errorf("cannot rollback a paused deployment config")
		}

		// A specific deployment was used.
		target = r
		configName = appsutil.DeploymentConfigNameFor(obj)
	case *appsv1.DeploymentConfig:
		if r.Spec.Paused {
			return fmt.Errorf("cannot rollback a paused deployment config")
		}
		// A deploymentconfig was used. Find the target deployment by the
		// specified version, or by a lookup of the last completed deployment if
		// no version was supplied.
		deployment, err := o.findTargetDeployment(r, o.DesiredVersion)
		if err != nil {
			return err
		}
		target = deployment
		configName = r.Name
	}
	if target == nil {
		return fmt.Errorf("%s is not a valid deployment or deployment config", o.TargetName)
	}

	// Set up the rollback and generate a new rolled back config.
	rollback := &appsv1.DeploymentConfigRollback{
		Name: configName,
		Spec: appsv1.DeploymentConfigRollbackSpec{
			From: corev1.ObjectReference{
				Name: target.Name,
			},
			Revision:               int64(o.DesiredVersion),
			IncludeTemplate:        true,
			IncludeTriggers:        o.IncludeTriggers,
			IncludeStrategy:        o.IncludeStrategy,
			IncludeReplicationMeta: o.IncludeScalingSettings,
		},
	}
	newConfig, err := o.appsClient.DeploymentConfigs(o.Namespace).Rollback(configName, rollback)
	if err != nil {
		return err
	}

	// If an output format is specified or this is a DryRun, print and exit.
	if len(o.Format) > 0 || o.DryRun {
		printer, err := o.ToPrinter(fmt.Sprintf("rolled back to %s", rollback.Spec.From.Name))
		if err != nil {
			return err
		}
		return printer.PrintObj(kcmdutil.AsDefaultVersionedOrOriginal(newConfig, mapping), o.Out)
	}

	// Perform a real rollback.
	rolledback, err := o.appsClient.DeploymentConfigs(newConfig.Namespace).Update(newConfig)
	if err != nil {
		return err
	}

	// Print warnings about any image triggers disabled during the rollback.
	successMessage := fmt.Sprintf("deployment #%d rolled back to %s", rolledback.Status.LatestVersion, rollback.Spec.From.Name)
	for _, trigger := range rolledback.Spec.Triggers {
		disabled := []string{}
		if trigger.Type == appsv1.DeploymentTriggerOnImageChange && !trigger.ImageChangeParams.Automatic {
			disabled = append(disabled, trigger.ImageChangeParams.From.Name)
		}
		if len(disabled) > 0 {
			reenable := fmt.Sprintf("oc set triggers dc/%s --auto", rolledback.Name)
			successMessage = fmt.Sprintf("%s\nWarning: the following images triggers were disabled: %s\n  You can re-enable them with: %s", successMessage, strings.Join(disabled, ","), reenable)
		}
	}

	printer, err := o.ToPrinter(successMessage)
	if err != nil {
		return err
	}

	return printer.PrintObj(kcmdutil.AsDefaultVersionedOrOriginal(rolledback, mapping), o.Out)
}

// findResource tries to find a deployment or deploymentconfig named
// targetName using a resource.Builder. For compatibility, if the resource
// name is unprefixed, treat it as an rc first and a dc second.
func (o *RollbackOptions) findResource(targetName string) (runtime.Object, *meta.RESTMapping, error) {
	candidates := []string{}
	if strings.Index(targetName, "/") == -1 {
		candidates = append(candidates, "rc/"+targetName)
		candidates = append(candidates, "dc/"+targetName)
	} else {
		candidates = append(candidates, targetName)
	}
	var obj runtime.Object
	var m *meta.RESTMapping
	for _, name := range candidates {
		r := o.builder().
			WithScheme(ocscheme.ReadingInternalScheme, ocscheme.ReadingInternalScheme.PrioritizedVersionsAllGroups()...).
			NamespaceParam(o.Namespace).
			ResourceTypeOrNameArgs(false, name).
			SingleResourceType().
			Do()
		if r.Err() != nil {
			return nil, nil, r.Err()
		}

		resultObj, err := r.Object()
		if err != nil {
			// If the resource wasn't found, try another candidate.
			if kerrors.IsNotFound(err) {
				continue
			}
			return nil, nil, err
		}
		obj = resultObj
		mapping, err := r.ResourceMapping()
		if err != nil {
			return nil, nil, err
		}

		m = mapping
		break
	}
	if obj == nil {
		return nil, nil, fmt.Errorf("%s is not a valid deployment or deployment config", targetName)
	}
	return obj, m, nil
}

// findTargetDeployment finds the deployment which is the rollback target by
// searching for deployments associated with config. If desiredVersion is >0,
// the deployment matching desiredVersion will be returned. If desiredVersion
// is <=0, the last completed deployment which is older than the config's
// version will be returned.
func (o *RollbackOptions) findTargetDeployment(config *appsv1.DeploymentConfig, desiredVersion int64) (*corev1.ReplicationController, error) {
	// Find deployments for the config sorted by version descending.
	deploymentList, err := o.kubeClient.CoreV1().ReplicationControllers(config.Namespace).List(metav1.ListOptions{LabelSelector: appsutil.ConfigSelector(config.Name).String()})
	if err != nil {
		return nil, err
	}
	deployments := make([]*corev1.ReplicationController, 0, len(deploymentList.Items))
	for i := range deploymentList.Items {
		deployments = append(deployments, &deploymentList.Items[i])
	}
	sort.Sort(appsutil.ByLatestVersionDesc(deployments))

	// Find the target deployment for rollback. If a version was specified,
	// use the version for a search. Otherwise, use the last completed
	// deployment.
	var target *corev1.ReplicationController
	for _, deployment := range deployments {
		version := appsutil.DeploymentVersionFor(deployment)
		if desiredVersion > 0 {
			if version == desiredVersion {
				target = deployment
				break
			}
		} else {
			if version < config.Status.LatestVersion && appsutil.IsCompleteDeployment(deployment) {
				target = deployment
				break
			}
		}
	}
	if target == nil {
		return nil, fmt.Errorf("couldn't find deployment for rollback")
	}
	return target, nil
}

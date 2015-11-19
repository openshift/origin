package policy

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"sort"
)

// ReconcileSCCRecommendedName is the recommended command name
const ReconcileSCCRecommendedName = "reconcile-sccs"

type ReconcileSCCOptions struct {
	// confirmed indicates that the data should be persisted
	Confirmed bool
	// union controls if we make additive changes to the users/groups fields or overwrite them
	// as well as preserving existing priorities (unset priorities will always be reconciled)
	Union bool
	// is the name of the openshift infrastructure namespace.  It is provided here so that
	// the command doesn't need to try and parse the policy config.
	InfraNamespace string

	Out    io.Writer
	Output string

	SCCClient kclient.SecurityContextConstraintInterface
	NSClient  kclient.NamespaceInterface
}

const (
	reconcileSCCLong = `
Replace cluster SCCs to match the recommended bootstrap policy

This command will inspect the cluster SCCs against the recommended bootstrap SCCs.
Any cluster SCC that does not match will be replaced by the recommended SCC.
This command will not remove any additional cluster SCCs.  By default, this command
will not remove additional users and groups that have been granted access to the SCC and
will preserve existing priorities (but will always reconcile unset priorities and the policy
definition).

You can see which cluster SCCs have recommended changes by choosing an output type.`

	reconcileSCCExample = `  # Display the cluster SCCs that would be modified
  $ %[1]s

  # Update cluster SCCs that don't match the current defaults preserving additional grants
  # for users and group and keeping any priorities that are already set
  $ %[1]s --confirm

  # Replace existing users, groups, and priorities that do not match defaults
  $ %[1]s --additive-only=false --confirm`
)

// NewDefaultReconcileSCCOptions provides a ReconcileSCCOptions with default settings.
func NewDefaultReconcileSCCOptions() *ReconcileSCCOptions {
	return &ReconcileSCCOptions{
		Union:          true,
		InfraNamespace: bootstrappolicy.DefaultOpenShiftInfraNamespace,
	}
}

// NewCmdReconcileSCC implements the OpenShift cli reconcile-sccs command.
func NewCmdReconcileSCC(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	o := NewDefaultReconcileSCCOptions()
	o.Out = out

	cmd := &cobra.Command{
		Use:     name,
		Short:   "Replace cluster SCCs to match the recommended bootstrap policy",
		Long:    reconcileSCCLong,
		Example: fmt.Sprintf(reconcileSCCExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if err := o.Complete(cmd, f, args); err != nil {
				kcmdutil.CheckErr(err)
			}
			if err := o.Validate(); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}
			if err := o.RunReconcileSCCs(cmd, f); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().BoolVar(&o.Confirmed, "confirm", o.Confirmed, "Specify that cluster SCCs should be modified. Defaults to false, displaying what would be replaced but not actually replacing anything.")
	cmd.Flags().BoolVar(&o.Union, "additive-only", o.Union, "Preserves extra users and groups in the SCC as well as existing priorities.")
	cmd.Flags().StringVar(&o.InfraNamespace, "infrastructure-namespace", o.InfraNamespace, "Name of the infrastructure namespace.")
	kcmdutil.AddPrinterFlags(cmd)
	cmd.Flags().Lookup("output").DefValue = "yaml"
	cmd.Flags().Lookup("output").Value.Set("yaml")
	return cmd
}

func (o *ReconcileSCCOptions) Complete(cmd *cobra.Command, f *clientcmd.Factory, args []string) error {
	if len(args) != 0 {
		return kcmdutil.UsageError(cmd, "no arguments are allowed")
	}

	_, kClient, err := f.Clients()
	if err != nil {
		return err
	}
	o.SCCClient = kClient.SecurityContextConstraints()
	o.NSClient = kClient.Namespaces()
	o.Output = kcmdutil.GetFlagString(cmd, "output")

	return nil
}

func (o *ReconcileSCCOptions) Validate() error {
	if o.SCCClient == nil {
		return errors.New("a SCC client is required")
	}
	if o.Output != "yaml" && o.Output != "json" && o.Output != "" {
		return fmt.Errorf("unknown output specified: %s", o.Output)
	}
	if _, err := o.NSClient.Get(o.InfraNamespace); err != nil {
		return fmt.Errorf("%s is not a valid namespace", o.InfraNamespace)
	}
	return nil
}

// RunReconcileSCCs contains the functionality for the reconcile-sccs command for making or
// previewing changes.
func (o *ReconcileSCCOptions) RunReconcileSCCs(cmd *cobra.Command, f *clientcmd.Factory) error {
	// get sccs that need updated
	changedSCCs, err := o.ChangedSCCs()
	if err != nil {
		return err
	}

	if len(changedSCCs) == 0 {
		return nil
	}

	if !o.Confirmed {
		list := &kapi.List{}
		for _, item := range changedSCCs {
			list.Items = append(list.Items, item)
		}
		if err := f.Factory.PrintObject(cmd, list, o.Out); err != nil {
			return err
		}
	}

	if o.Confirmed {
		return o.ReplaceChangedSCCs(changedSCCs)
	}
	return nil
}

// ChangedSCCs returns the SCCs that must be created and/or updated to match the
// recommended bootstrap SCCs.
func (o *ReconcileSCCOptions) ChangedSCCs() ([]*kapi.SecurityContextConstraints, error) {
	changedSCCs := []*kapi.SecurityContextConstraints{}

	groups, users := bootstrappolicy.GetBoostrapSCCAccess(o.InfraNamespace)
	bootstrapSCCs := bootstrappolicy.GetBootstrapSecurityContextConstraints(groups, users)

	for i := range bootstrapSCCs {
		expectedSCC := &bootstrapSCCs[i]
		actualSCC, err := o.SCCClient.Get(expectedSCC.Name)
		// if not found it needs to be created
		if kapierrors.IsNotFound(err) {
			changedSCCs = append(changedSCCs, expectedSCC)
			continue
		}
		if err != nil {
			return nil, err
		}

		// if found then we need to diff to see if it needs updated
		if updatedSCC, needsUpdating := o.computeUpdatedSCC(*expectedSCC, *actualSCC); needsUpdating {
			changedSCCs = append(changedSCCs, updatedSCC)
		}
	}
	return changedSCCs, nil
}

// ReplaceChangedSCCs persists the changed SCCs.
func (o *ReconcileSCCOptions) ReplaceChangedSCCs(changedSCCs []*kapi.SecurityContextConstraints) error {
	for i := range changedSCCs {
		_, err := o.SCCClient.Get(changedSCCs[i].Name)
		if err != nil && !kapierrors.IsNotFound(err) {
			return err
		}

		if kapierrors.IsNotFound(err) {
			createdSCC, err := o.SCCClient.Create(changedSCCs[i])
			if err != nil {
				return err
			}
			fmt.Fprintf(o.Out, "securitycontextconstraints/%s\n", createdSCC.Name)
			continue
		}

		updatedSCC, err := o.SCCClient.Update(changedSCCs[i])
		if err != nil {
			return err
		}
		fmt.Fprintf(o.Out, "securitycontextconstraints/%s\n", updatedSCC.Name)
	}
	return nil
}

// computeUpdatedSCC determines if the expected SCC looks like the actual SCC
// it does this by making the expected SCC mirror the actual SCC for items that
// we are not reconciling and performing a diff (ignoring changes to metadata).
// If a diff is produced then the expected SCC is submitted as needing an update.
func (o *ReconcileSCCOptions) computeUpdatedSCC(expected kapi.SecurityContextConstraints, actual kapi.SecurityContextConstraints) (*kapi.SecurityContextConstraints, bool) {
	needsUpdate := false

	// if unioning old and new groups/users then make the expected contain all
	// also preserve and set priorities
	if o.Union {
		groupSet := sets.NewString(actual.Groups...)
		groupSet.Insert(expected.Groups...)
		expected.Groups = groupSet.List()

		userSet := sets.NewString(actual.Users...)
		userSet.Insert(expected.Users...)
		expected.Users = userSet.List()

		if actual.Priority != nil {
			expected.Priority = actual.Priority
		}
	}

	// sort users and groups to remove any variants in order when diffing
	sort.StringSlice(actual.Groups).Sort()
	sort.StringSlice(actual.Users).Sort()
	sort.StringSlice(expected.Groups).Sort()
	sort.StringSlice(expected.Users).Sort()

	// make a copy of the expected scc here so we can ignore metadata diffs.
	updated := expected
	expected.ObjectMeta = actual.ObjectMeta

	if !kapi.Semantic.DeepEqual(expected, actual) {
		needsUpdate = true
	}

	return &updated, needsUpdate
}

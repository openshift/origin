package policy

import (
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"

	sccutil "github.com/openshift/origin/pkg/security/securitycontextconstraints/util"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	securitytypedclient "github.com/openshift/origin/pkg/security/generated/internalclientset/typed/security/internalversion"
)

// ReconcileSCCRecommendedName is the recommended command name
const ReconcileSCCRecommendedName = "reconcile-sccs"

type ReconcileSCCOptions struct {
	// confirmed indicates that the data should be persisted
	Confirmed bool
	// union controls if we make additive changes to the users/groups/labels/annotations fields
	// or overwrite them as well as preserving existing priorities (unset priorities will
	// always be reconciled)
	Union bool
	// is the name of the openshift infrastructure namespace.  It is provided here so that
	// the command doesn't need to try and parse the policy config.
	InfraNamespace string

	Out    io.Writer
	Output string

	SCCClient securitytypedclient.SecurityContextConstraintsInterface
	NSClient  kcoreclient.NamespaceInterface
}

var (
	reconcileSCCLong = templates.LongDesc(`
		Replace cluster SCCs to match the recommended bootstrap policy

		This command will inspect the cluster SCCs against the recommended bootstrap SCCs.
		Any cluster SCC that does not match will be replaced by the recommended SCC.
		This command will not remove any additional cluster SCCs.  By default, this command
		will not remove additional users and groups that have been granted access to the SCC and
		will preserve existing priorities (but will always reconcile unset priorities and the policy
		definition), labels, and annotations.

		You can see which cluster SCCs have recommended changes by choosing an output type.`)

	reconcileSCCExample = templates.Examples(`
		# Display the cluster SCCs that would be modified
	  %[1]s

	  # Update cluster SCCs that don't match the current defaults preserving additional grants
	  # for users, groups, labels, annotations and keeping any priorities that are already set
	  %[1]s --confirm

	  # Replace existing users, groups, labels, annotations, and priorities that do not match defaults
	  %[1]s --additive-only=false --confirm`)
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
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}
			if err := o.RunReconcileSCCs(cmd, f); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().BoolVar(&o.Confirmed, "confirm", o.Confirmed, "If true, specify that cluster SCCs should be modified. Defaults to false, displaying what would be replaced but not actually replacing anything.")
	cmd.Flags().BoolVar(&o.Union, "additive-only", o.Union, "If true, preserves extra users, groups, labels and annotations in the SCC as well as existing priorities.")
	cmd.Flags().StringVar(&o.InfraNamespace, "infrastructure-namespace", o.InfraNamespace, "Name of the infrastructure namespace.")
	kcmdutil.AddPrinterFlags(cmd)
	cmd.Flags().Lookup("output").DefValue = "yaml"
	cmd.Flags().Lookup("output").Value.Set("yaml")
	return cmd
}

func (o *ReconcileSCCOptions) Complete(cmd *cobra.Command, f *clientcmd.Factory, args []string) error {
	if len(args) != 0 {
		return kcmdutil.UsageErrorf(cmd, "no arguments are allowed")
	}

	kClient, err := f.ClientSet()
	if err != nil {
		return err
	}
	securityClient, err := f.OpenshiftInternalSecurityClient()
	if err != nil {
		return err
	}
	o.SCCClient = securityClient.Security().SecurityContextConstraints()
	o.NSClient = kClient.Core().Namespaces()
	o.Output = kcmdutil.GetFlagString(cmd, "output")

	return nil
}

func (o *ReconcileSCCOptions) Validate() error {
	if o.SCCClient == nil {
		return errors.New("a SCC client is required")
	}
	if _, err := o.NSClient.Get(o.InfraNamespace, metav1.GetOptions{}); err != nil {
		return fmt.Errorf("Failed to GET reconcile SCC namespace %s: %v", o.InfraNamespace, err)
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
		mapper, _ := f.Object()
		fn := cmdutil.VersionedPrintObject(f.PrintObject, cmd, mapper, o.Out)
		if err := fn(list); err != nil {
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
func (o *ReconcileSCCOptions) ChangedSCCs() ([]*securityapi.SecurityContextConstraints, error) {
	changedSCCs := []*securityapi.SecurityContextConstraints{}

	groups, users := bootstrappolicy.GetBoostrapSCCAccess(o.InfraNamespace)
	bootstrapSCCs := bootstrappolicy.GetBootstrapSecurityContextConstraints(groups, users)

	for _, expectedSCC := range bootstrapSCCs {
		actualSCC, err := o.SCCClient.Get(expectedSCC.Name, metav1.GetOptions{})
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
func (o *ReconcileSCCOptions) ReplaceChangedSCCs(changedSCCs []*securityapi.SecurityContextConstraints) error {
	for i := range changedSCCs {
		_, err := o.SCCClient.Get(changedSCCs[i].Name, metav1.GetOptions{})
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
func (o *ReconcileSCCOptions) computeUpdatedSCC(expected securityapi.SecurityContextConstraints, actual securityapi.SecurityContextConstraints) (*securityapi.SecurityContextConstraints, bool) {
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

		// preserve labels and annotations
		expected.Labels = MergeMaps(expected.Labels, actual.Labels)
		expected.Annotations = MergeMaps(expected.Annotations, actual.Annotations)
	}

	// sort volumes to remove variants in order
	sortVolumes(&expected)
	sortVolumes(&actual)

	// sort users and groups to remove any variants in order when diffing
	sort.StringSlice(actual.Groups).Sort()
	sort.StringSlice(actual.Users).Sort()
	sort.StringSlice(expected.Groups).Sort()
	sort.StringSlice(expected.Users).Sort()

	// compute the updated scc as follows:
	// 1. start with the expected scc
	// 2. take the objectmeta from the actual scc (preserves the resource version and uid)
	// 3. add back the labels and annotations from the expected scc (which were already merged if unioning was desired)
	updated := expected
	updated.ObjectMeta = actual.ObjectMeta
	updated.ObjectMeta.Labels = expected.Labels
	updated.ObjectMeta.Annotations = expected.Annotations

	if !kapihelper.Semantic.DeepEqual(updated, actual) {
		needsUpdate = true
	}

	return &updated, needsUpdate
}

// sortVolumes sorts the volume slice of the SCC in place.
func sortVolumes(scc *securityapi.SecurityContextConstraints) {
	if scc.Volumes == nil || len(scc.Volumes) == 0 {
		return
	}
	volumes := sccutil.FSTypeToStringSet(scc.Volumes).List()
	sort.StringSlice(volumes).Sort()
	scc.Volumes = sliceToFSType(volumes)
}

// sliceToFSType converts a string slice into FStypes.
func sliceToFSType(s []string) []securityapi.FSType {
	fsTypes := []securityapi.FSType{}
	for _, v := range s {
		fsTypes = append(fsTypes, securityapi.FSType(v))
	}
	return fsTypes
}

// MergeMaps will merge to map[string]string instances, with
// keys from the second argument overwriting keys from the
// first argument, in case of duplicates.
func MergeMaps(a, b map[string]string) map[string]string {
	if a == nil && b == nil {
		return nil
	}

	res := make(map[string]string)

	for k, v := range a {
		res[k] = v
	}

	for k, v := range b {
		res[k] = v
	}

	return res
}

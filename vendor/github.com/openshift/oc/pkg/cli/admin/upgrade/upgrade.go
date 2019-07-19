package upgrade

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/blang/semver"
	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	imagereference "github.com/openshift/library-go/pkg/image/reference"
)

func NewOptions(streams genericclioptions.IOStreams) *Options {
	return &Options{
		IOStreams: streams,
	}
}

func New(f kcmdutil.Factory, parentName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewOptions(streams)
	cmd := &cobra.Command{
		Use:   "upgrade --to=VERSION",
		Short: "Upgrade a cluster",
		Long: templates.LongDesc(`
			Upgrade the cluster to a newer version

			This command will request that the cluster begin an upgrade. If no arguments are passed
			the command will retrieve the current version info and display whether an upgrade is
			in progress or whether any errors might prevent an upgrade, as well as show the suggested
			updates available to the cluster. Information about compatible updates is periodically
			retrieved from the update server and cached on the cluster - these are updates that are
			known to be supported as upgrades from the current version.

			Passing --to=VERSION will upgrade the cluster to one of the available updates or report
			an error if no such version exists. The cluster will then upgrade itself and report
			status that is available via "oc get clusterversion" and "oc describe clusterversion".

			If the cluster is already being upgrade, or the cluster version has a failing or invalid
			state you may pass --force to continue the upgrade anyway.

			If there are no versions available, or a bug in the cluster version operator prevents
			updates from being retrieved, the more powerful and dangerous --to-image=IMAGE option
			may be used. This forces the cluster to upgrade to the contents of the specified release
			image, regardless of whether that upgrade is safe to apply to the current version. While
			rolling back to a previous micro version (4.0.2 -> 4.0.1) may be safe, upgrading more
			than one minor version ahead (4.0 -> 4.2) or downgrading one minor version (4.1 -> 4.0)
			is likely to cause data corruption or to completely break a cluster.
		`),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Run())
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&o.To, "to", o.To, "Specify the version to upgrade to. The version must be on the list of previous or available updates.")
	flags.StringVar(&o.ToImage, "to-image", o.ToImage, "Provide a release image to upgrade to. WARNING: This option does not check for upgrade compatibility and may break your cluster.")
	flags.BoolVar(&o.ToLatestAvailable, "to-latest", o.ToLatestAvailable, "Use the next available version")
	flags.BoolVar(&o.Clear, "clear", o.Clear, "If an upgrade has been requested but not yet downloaded, cancel the update. This has no effect once the update has started.")
	flags.BoolVar(&o.Force, "force", o.Force, "Upgrade even if an upgrade is in process or other error is blocking update.")
	return cmd
}

type Options struct {
	genericclioptions.IOStreams

	To                string
	ToImage           string
	ToLatestAvailable bool

	Force bool
	Clear bool

	Client configv1client.Interface
}

func (o *Options) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	if o.Clear && (len(o.ToImage) > 0 || len(o.To) > 0 || o.ToLatestAvailable) {
		return fmt.Errorf("--clear may not be specified with any other flags")
	}
	if len(o.To) > 0 && len(o.ToImage) > 0 {
		return fmt.Errorf("only one of --to or --to-image may be provided")
	}

	if len(o.To) > 0 {
		if _, err := semver.Parse(o.To); err != nil {
			return fmt.Errorf("--to must be a semantic version (e.g. 4.0.1 or 4.1.0-nightly-20181104): %v", err)
		}
	}
	// defend against simple mistakes (4.0.1 is a valid container image)
	if len(o.ToImage) > 0 {
		ref, err := imagereference.Parse(o.ToImage)
		if err != nil {
			return fmt.Errorf("--to-image must be a valid image pull spec: %v", err)
		}
		if len(ref.Registry) == 0 && len(ref.Namespace) == 0 {
			return fmt.Errorf("--to-image must be a valid image pull spec: no registry or repository specified")
		}
		if len(ref.ID) == 0 && len(ref.Tag) == 0 {
			return fmt.Errorf("--to-image must be a valid image pull spec: no tag or digest specified")
		}
	}

	cfg, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	client, err := configv1client.NewForConfig(cfg)
	if err != nil {
		return err
	}
	o.Client = client
	return nil
}

func (o *Options) Run() error {
	cv, err := o.Client.ConfigV1().ClusterVersions().Get("version", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("No cluster version information available - you must be connected to a v4.0 OpenShift server to fetch the current version")
		}
		return err
	}

	switch {
	case o.Clear:
		if cv.Spec.DesiredUpdate == nil {
			fmt.Fprintf(o.Out, "info: No update in progress\n")
			return nil
		}
		original := cv.Spec.DesiredUpdate
		cv.Spec.DesiredUpdate = nil
		updated, err := o.Client.ConfigV1().ClusterVersions().Patch(cv.Name, types.MergePatchType, []byte(`{"spec":{"desiredUpdate":null}}`))
		if err != nil {
			return fmt.Errorf("Unable to cancel current rollout: %v", err)
		}
		if updateIsEquivalent(*original, updated.Status.Desired) {
			fmt.Fprintf(o.Out, "Cleared the update field, still at %s\n", updateVersionString(updated.Status.Desired))
		} else {
			fmt.Fprintf(o.Out, "Cancelled requested upgrade to %s\n", updateVersionString(*original))
		}
		return nil

	case o.ToLatestAvailable:
		if len(cv.Status.AvailableUpdates) == 0 {
			fmt.Fprintf(o.Out, "info: Cluster is already at the latest available version %s\n", cv.Status.Desired.Version)
			return nil
		}

		if !o.Force {
			if err := checkForUpgrade(cv); err != nil {
				return err
			}
		}

		sortSemanticVersions(cv.Status.AvailableUpdates)
		update := cv.Status.AvailableUpdates[len(cv.Status.AvailableUpdates)-1]
		cv.Spec.DesiredUpdate = &update

		_, err := o.Client.ConfigV1().ClusterVersions().Update(cv)
		if err != nil {
			return fmt.Errorf("Unable to upgrade to latest version %s: %v", update.Version, err)
		}

		if len(update.Version) > 0 {
			fmt.Fprintf(o.Out, "Updating to latest version %s\n", update.Version)
		} else {
			fmt.Fprintf(o.Out, "Updating to latest release image %s\n", update.Image)
		}

		return nil

	case len(o.To) > 0, len(o.ToImage) > 0:
		var update *configv1.Update
		if len(o.To) > 0 {
			if o.To == cv.Status.Desired.Version {
				fmt.Fprintf(o.Out, "info: Cluster is already at version %s\n", o.To)
				return nil
			}
			for _, available := range cv.Status.AvailableUpdates {
				if available.Version == o.To {
					update = &available
					break
				}
			}
			if update == nil {
				if len(cv.Status.AvailableUpdates) == 0 {
					if c := findCondition(cv.Status.Conditions, configv1.RetrievedUpdates); c != nil && c.Status == configv1.ConditionFalse {
						return fmt.Errorf("Can't look up image for version %s. %v", o.To, c.Message)
					}
					return fmt.Errorf("No available updates, specify --to-image or wait for new updates to be available")
				}
				return fmt.Errorf("The update %s is not one of the available updates: %s", o.To, strings.Join(versionStrings(cv.Status.AvailableUpdates), ", "))
			}
		}
		if len(o.ToImage) > 0 {
			if o.ToImage == cv.Status.Desired.Image && !o.Force {
				fmt.Fprintf(o.Out, "info: Cluster is already using release image %s\n", o.ToImage)
				return nil
			}
			update = &configv1.Update{
				Version: "",
				Image:   o.ToImage,
			}
		}

		if o.Force {
			update.Force = true
		} else {
			if err := checkForUpgrade(cv); err != nil {
				return err
			}
		}

		cv.Spec.DesiredUpdate = update

		_, err := o.Client.ConfigV1().ClusterVersions().Update(cv)
		if err != nil {
			return fmt.Errorf("Unable to upgrade: %v", err)
		}

		if len(update.Version) > 0 {
			fmt.Fprintf(o.Out, "Updating to %s\n", update.Version)
		} else {
			fmt.Fprintf(o.Out, "Updating to release image %s\n", update.Image)
		}

		return nil

	default:
		if c := findCondition(cv.Status.Conditions, configv1.OperatorDegraded); c != nil && c.Status == configv1.ConditionTrue {
			prefix := "No upgrade is possible due to an error"
			if c := findCondition(cv.Status.Conditions, configv1.OperatorProgressing); c != nil && c.Status == configv1.ConditionTrue && len(c.Message) > 0 {
				prefix = c.Message
			}
			if len(c.Message) > 0 {
				return fmt.Errorf("%s:\n\n  Reason: %s\n  Message: %s\n\n", prefix, c.Reason, c.Message)
			}
			return fmt.Errorf("The cluster can't be upgraded, see `oc describe clusterversion`")
		}

		if c := findCondition(cv.Status.Conditions, configv1.OperatorProgressing); c != nil && len(c.Message) > 0 {
			if c.Status == configv1.ConditionTrue {
				fmt.Fprintf(o.Out, "info: An upgrade is in progress. %s\n", c.Message)
			} else {
				fmt.Fprintln(o.Out, c.Message)
			}
		} else {
			fmt.Fprintln(o.ErrOut, "warning: No current status info, see `oc describe clusterversion` for more details")
		}
		fmt.Fprintln(o.Out)

		if len(cv.Status.AvailableUpdates) > 0 {
			fmt.Fprintf(o.Out, "Updates:\n\n")
			w := tabwriter.NewWriter(o.Out, 0, 2, 1, ' ', 0)
			fmt.Fprintf(w, "VERSION\tIMAGE\n")
			// TODO: add metadata about version
			for _, update := range cv.Status.AvailableUpdates {
				fmt.Fprintf(w, "%s\t%s\n", update.Version, update.Image)
			}
			w.Flush()
			if c := findCondition(cv.Status.Conditions, configv1.RetrievedUpdates); c != nil && c.Status == configv1.ConditionFalse {
				fmt.Fprintf(o.ErrOut, "warning: Cannot refresh available updates:\n  Reason: %s\n  Message: %s\n\n", c.Reason, c.Message)
			}
		} else {
			if c := findCondition(cv.Status.Conditions, configv1.RetrievedUpdates); c != nil && c.Status == configv1.ConditionFalse {
				fmt.Fprintf(o.ErrOut, "warning: Cannot display available updates:\n  Reason: %s\n  Message: %s\n\n", c.Reason, c.Message)
			} else {
				fmt.Fprintf(o.Out, "No updates available. You may force an upgrade to a specific release image, but doing so may not be supported and result in downtime or data loss.\n")
			}
		}

		// TODO: print previous versions
	}

	return nil
}

func errorList(errs []error) string {
	if len(errs) == 1 {
		return errs[0].Error()
	}
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, "\n\n")
	for _, err := range errs {
		fmt.Fprintf(buf, "* %v\n", err)
	}
	return buf.String()
}

func updateVersionString(update configv1.Update) string {
	if len(update.Version) > 0 {
		return update.Version
	}
	if len(update.Image) > 0 {
		return update.Image
	}
	return "<unknown>"
}

func stringArrContains(arr []string, s string) bool {
	for _, item := range arr {
		if item == s {
			return true
		}
	}
	return false
}

func writeTabSection(out io.Writer, fn func(w io.Writer)) {
	w := tabwriter.NewWriter(out, 0, 4, 1, ' ', 0)
	fn(w)
	w.Flush()
}

func updateIsEquivalent(a, b configv1.Update) bool {
	switch {
	case len(a.Image) > 0 && len(b.Image) > 0:
		return a.Image == b.Image
	case len(a.Version) > 0 && len(b.Version) > 0:
		return a.Version == b.Version
	default:
		return false
	}
}

// sortSemanticVersions sorts the input slice in increasing order.
func sortSemanticVersions(versions []configv1.Update) {
	sort.Slice(versions, func(i, j int) bool {
		a, errA := semver.Parse(versions[i].Version)
		b, errB := semver.Parse(versions[j].Version)
		if errA == nil && errB != nil {
			return false
		}
		if errB == nil && errA != nil {
			return true
		}
		if errA != nil && errB != nil {
			return versions[i].Version < versions[j].Version
		}
		return a.LT(b)
	})
}

func versionStrings(updates []configv1.Update) []string {
	var arr []string
	for _, update := range updates {
		arr = append(arr, update.Version)
	}
	return arr
}

func findCondition(conditions []configv1.ClusterOperatorStatusCondition, name configv1.ClusterStatusConditionType) *configv1.ClusterOperatorStatusCondition {
	for i := range conditions {
		if conditions[i].Type == name {
			return &conditions[i]
		}
	}
	return nil
}

func checkForUpgrade(cv *configv1.ClusterVersion) error {
	if c := findCondition(cv.Status.Conditions, "Invalid"); c != nil && c.Status == configv1.ConditionTrue {
		return fmt.Errorf("The cluster version object is invalid, you must correct the invalid state first.\n\n  Reason: %s\n  Message: %s\n\n", c.Reason, c.Message)
	}
	if c := findCondition(cv.Status.Conditions, configv1.OperatorDegraded); c != nil && c.Status == configv1.ConditionTrue {
		return fmt.Errorf("The cluster is experiencing an upgrade-blocking error, use --force to upgrade anyway.\n\n  Reason: %s\n  Message: %s\n\n", c.Reason, c.Message)
	}
	if c := findCondition(cv.Status.Conditions, configv1.OperatorProgressing); c != nil && c.Status == configv1.ConditionTrue {
		return fmt.Errorf("Already upgrading, pass --force to override.\n\n  Reason: %s\n  Message: %s\n\n", c.Reason, c.Message)
	}
	return nil
}

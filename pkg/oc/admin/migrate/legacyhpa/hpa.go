package legacyhpa

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	autoscaling "k8s.io/kubernetes/pkg/apis/autoscaling"
	autoscalingclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/autoscaling/internalversion"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	"github.com/openshift/origin/pkg/oc/admin/migrate"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

var (
	defaultMigrations = map[metav1.TypeMeta]metav1.TypeMeta{
		// legacy oapi group
		{"DeploymentConfig", "v1"}: {"DeploymentConfig", "apps.openshift.io/v1"},
		// legacy oapi group, for the lazy
		{"DeploymentConfig", ""}: {"DeploymentConfig", "apps.openshift.io/v1"},

		// webconsole shenaniganry
		{"DeploymentConfig", "extensions/v1beta1"}:      {"DeploymentConfig", "apps.openshift.io/v1"},
		{"Deployment", "extensions/v1beta1"}:            {"Deployment", "apps/v1"},
		{"ReplicaSet", "extensions/v1beta1"}:            {"ReplicaSet", "apps/v1"},
		{"ReplicationController", "extensions/v1beta1"}: {"ReplicationController", "v1"},
	}

	internalMigrateLegacyHPALong = templates.LongDesc(fmt.Sprintf(`
		Migrate Horizontal Pod Autoscalers to refer to new API groups

		This command locates and updates every Horizontal Pod Autoscaler which refers to a particular
		group-version-kind to refer to some other, equivalent group-version-kind.

		The following transformations will occur:

%s`, prettyPrintMigrations(defaultMigrations)))

	internalMigrateLegacyHPAExample = templates.Examples(`
		# Perform a dry-run of updating all objects
	  %[1]s

	  # To actually perform the update, the confirm flag must be appended
	  %[1]s --confirm

	  # Migrate a specific group-version-kind to the latest preferred version
	  %[1]s --initial=extensions/v1beta1.ReplicaSet --confirm

	  # Migrate a specific group-version-kind to a specific group-version-kind
	  %[1]s --initial=v1.DeploymentConfig --final=apps.openshift.io/v1.DeploymentConfig --confirm`)
)

func prettyPrintMigrations(versionKinds map[metav1.TypeMeta]metav1.TypeMeta) string {
	lines := make([]string, 0, len(versionKinds))
	for initial, final := range versionKinds {
		line := fmt.Sprintf("		- %s.%s --> %s.%s", initial.APIVersion, initial.Kind, final.APIVersion, final.Kind)
		lines = append(lines, line)
	}
	sort.Strings(lines)

	return strings.Join(lines, "\n")
}

type MigrateLegacyHPAOptions struct {
	// maps initial gvks to final gvks in the same format
	// as HPAs use (CrossVersionObjectReferences) for ease of access.
	finalVersionKinds map[metav1.TypeMeta]metav1.TypeMeta

	hpaClient autoscalingclient.AutoscalingInterface

	migrate.ResourceOptions
}

// NewCmdMigrateLegacyAPI implements a MigrateLegacyHPA command
func NewCmdMigrateLegacyHPA(name, fullName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	options := &MigrateLegacyHPAOptions{
		ResourceOptions: migrate.ResourceOptions{
			In:     in,
			Out:    out,
			ErrOut: errout,

			AllNamespaces: true,
			Include:       []string{"horizontalpodautoscalers.autoscaling"},
		},
	}
	cmd := &cobra.Command{
		Use:     name,
		Short:   "Update HPAs to point to the latest group-version-kinds",
		Long:    internalMigrateLegacyHPALong,
		Example: fmt.Sprintf(internalMigrateLegacyHPAExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete(name, f, cmd, args))
			kcmdutil.CheckErr(options.Validate())
			kcmdutil.CheckErr(options.Run())
		},
	}
	options.ResourceOptions.Bind(cmd)

	return cmd
}

func (o *MigrateLegacyHPAOptions) Complete(name string, f *clientcmd.Factory, c *cobra.Command, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("%s takes no positional arguments", name)
	}

	o.ResourceOptions.SaveFn = o.save
	if err := o.ResourceOptions.Complete(f, c); err != nil {
		return err
	}

	o.finalVersionKinds = make(map[metav1.TypeMeta]metav1.TypeMeta)

	// copy all manual transformations in
	for initial, final := range defaultMigrations {
		o.finalVersionKinds[initial] = final
	}

	kubeClientSet, err := f.ClientSet()
	if err != nil {
		return err
	}
	o.hpaClient = kubeClientSet.Autoscaling()

	return nil
}

func (o MigrateLegacyHPAOptions) Validate() error {
	if len(o.ResourceOptions.Include) != 1 || o.ResourceOptions.Include[0] != "horizontalpodautoscalers.autoscaling" {
		return fmt.Errorf("the only supported resources are horizontalpodautoscalers")
	}
	return o.ResourceOptions.Validate()
}

func (o MigrateLegacyHPAOptions) Run() error {
	return o.ResourceOptions.Visitor().Visit(func(info *resource.Info) (migrate.Reporter, error) {
		return o.checkAndTransform(info.Object)
	})
}

func (o *MigrateLegacyHPAOptions) checkAndTransform(hpaRaw runtime.Object) (migrate.Reporter, error) {
	hpa, wasHPA := hpaRaw.(*autoscaling.HorizontalPodAutoscaler)
	if !wasHPA {
		return nil, fmt.Errorf("unrecognized object %#v", hpaRaw)
	}

	currentVersionKind := metav1.TypeMeta{
		APIVersion: hpa.Spec.ScaleTargetRef.APIVersion,
		Kind:       hpa.Spec.ScaleTargetRef.Kind,
	}

	newVersionKind := o.latestVersionKind(currentVersionKind)

	if currentVersionKind != newVersionKind {
		hpa.Spec.ScaleTargetRef.APIVersion = newVersionKind.APIVersion
		hpa.Spec.ScaleTargetRef.Kind = newVersionKind.Kind
		return migrate.ReporterBool(true), nil
	}

	return migrate.ReporterBool(false), nil
}

func (o *MigrateLegacyHPAOptions) latestVersionKind(current metav1.TypeMeta) metav1.TypeMeta {
	if newVersionKind, isKnown := o.finalVersionKinds[current]; isKnown {
		return newVersionKind
	}

	return current
}

// save invokes the API to alter an object. The reporter passed to this method is the same returned by
// the migration visitor method. It should return an error  if the input type cannot be saved
// It returns migrate.ErrRecalculate if migration should be re-run on the provided object.
func (o *MigrateLegacyHPAOptions) save(info *resource.Info, reporter migrate.Reporter) error {
	hpa, wasHPA := info.Object.(*autoscaling.HorizontalPodAutoscaler)
	if !wasHPA {
		return fmt.Errorf("unrecognized object %#v", info.Object)
	}

	_, err := o.hpaClient.HorizontalPodAutoscalers(hpa.Namespace).Update(hpa)
	return migrate.DefaultRetriable(info, err)
}

package create

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/api/unversioned"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	quotaapi "github.com/openshift/origin/pkg/quota/api"
)

const ClusterQuotaRecommendedName = "clusterresourcequota"

var (
	clusterQuotaLong = templates.LongDesc(`
		Create a cluster resource quota that controls certain resources.

		Cluster resource quota objects defined quota restrictions that span multiple projects based on label selectors.`)

	clusterQuotaExample = templates.Examples(`
		# Create a cluster resource quota limited to 10 pods
  	%[1]s limit-bob --project-annotation-selector=openshift.io/requester=user-bob --hard=pods=10`)
)

type CreateClusterQuotaOptions struct {
	ClusterQuota *quotaapi.ClusterResourceQuota
	Client       client.ClusterResourceQuotasInterface

	DryRun bool

	Mapper       meta.RESTMapper
	OutputFormat string
	Out          io.Writer
	Printer      ObjectPrinter
}

// NewCmdCreateServiceAccount is a macro command to create a new service account
func NewCmdCreateClusterQuota(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	o := &CreateClusterQuotaOptions{Out: out}

	cmd := &cobra.Command{
		Use:     name + " NAME --project-label-selector=key=value [--hard=RESOURCE=QUANTITY]...",
		Short:   "Create cluster resource quota resource.",
		Long:    clusterQuotaLong,
		Example: fmt.Sprintf(clusterQuotaExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(cmd, f, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
		Aliases: []string{"clusterquota"},
	}

	cmdutil.AddPrinterFlags(cmd)
	cmdutil.AddDryRunFlag(cmd)
	cmd.Flags().String("project-label-selector", "", "The project label selector for the cluster resource quota")
	cmd.Flags().String("project-annotation-selector", "", "The project annotation selector for the cluster resource quota")
	cmd.Flags().StringSlice("hard", []string{}, "The resource to constrain: RESOURCE=QUANTITY (pods=10)")
	return cmd
}

func (o *CreateClusterQuotaOptions) Complete(cmd *cobra.Command, f *clientcmd.Factory, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("NAME is required: %v", args)
	}

	o.DryRun = cmdutil.GetFlagBool(cmd, "dry-run")

	var labelSelector *unversioned.LabelSelector
	labelSelectorString := cmdutil.GetFlagString(cmd, "project-label-selector")
	if len(labelSelectorString) > 0 {
		var err error
		labelSelector, err = unversioned.ParseToLabelSelector(labelSelectorString)
		if err != nil {
			return err
		}
	}

	annotationSelector, err := parseAnnotationSelector(cmdutil.GetFlagString(cmd, "project-annotation-selector"))
	if err != nil {
		return err
	}

	o.ClusterQuota = &quotaapi.ClusterResourceQuota{
		ObjectMeta: kapi.ObjectMeta{Name: args[0]},
		Spec: quotaapi.ClusterResourceQuotaSpec{
			Selector: quotaapi.ClusterResourceQuotaSelector{
				LabelSelector:      labelSelector,
				AnnotationSelector: annotationSelector,
			},
			Quota: kapi.ResourceQuotaSpec{
				Hard: kapi.ResourceList{},
			},
		},
	}

	for _, resourceCount := range cmdutil.GetFlagStringSlice(cmd, "hard") {
		tokens := strings.Split(resourceCount, "=")
		if len(tokens) != 2 {
			return fmt.Errorf("%v must in the form of resource=quantity", resourceCount)
		}
		quantity, err := resource.ParseQuantity(tokens[1])
		if err != nil {
			return err
		}
		o.ClusterQuota.Spec.Quota.Hard[kapi.ResourceName(tokens[0])] = quantity
	}

	o.Client, _, err = f.Clients()
	if err != nil {
		return err
	}

	o.Mapper, _ = f.Object(false)
	o.OutputFormat = cmdutil.GetFlagString(cmd, "output")

	o.Printer = func(obj runtime.Object, out io.Writer) error {
		return f.PrintObject(cmd, o.Mapper, obj, out)
	}

	return nil
}

func (o *CreateClusterQuotaOptions) Validate() error {
	if o.ClusterQuota == nil {
		return fmt.Errorf("ClusterQuota is required")
	}
	if o.Client == nil {
		return fmt.Errorf("Client is required")
	}
	if o.Mapper == nil {
		return fmt.Errorf("Mapper is required")
	}
	if o.Out == nil {
		return fmt.Errorf("Out is required")
	}
	if o.Printer == nil {
		return fmt.Errorf("Printer is required")
	}

	return nil
}

func (o *CreateClusterQuotaOptions) Run() error {
	actualObj := o.ClusterQuota

	var err error
	if !o.DryRun {
		actualObj, err = o.Client.ClusterResourceQuotas().Create(o.ClusterQuota)
		if err != nil {
			return err
		}
	}

	if useShortOutput := o.OutputFormat == "name"; useShortOutput || len(o.OutputFormat) == 0 {
		cmdutil.PrintSuccess(o.Mapper, useShortOutput, o.Out, "clusterquota", actualObj.Name, o.DryRun, "created")
		return nil
	}

	return o.Printer(actualObj, o.Out)
}

// parseAnnotationSelector just parses key=value,key=value=...,
// further validation is left to be done server-side.
func parseAnnotationSelector(s string) (map[string]string, error) {
	if len(s) == 0 {
		return nil, nil
	}
	stringReader := strings.NewReader(s)
	csvReader := csv.NewReader(stringReader)
	annotations, err := csvReader.Read()
	if err != nil {
		return nil, err
	}
	parsed := map[string]string{}
	for _, annotation := range annotations {
		parts := strings.SplitN(annotation, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("Malformed annotation selector, expected %q: %s", "key=value", annotation)
		}
		parsed[parts[0]] = parts[1]
	}
	return parsed, nil
}

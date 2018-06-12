package create

import (
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	kapiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	quotav1 "github.com/openshift/api/quota/v1"
	quotav1client "github.com/openshift/client-go/quota/clientset/versioned/typed/quota/v1"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
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
	*CreateSubcommandOptions

	LabelSelectorStr      string
	AnnotationSelectorStr string
	Hard                  []string

	LabelSelector      *metav1.LabelSelector
	AnnotationSelector map[string]string

	Client quotav1client.ClusterResourceQuotasGetter
}

// NewCmdCreateClusterQuota is a macro command to create a new cluster quota.
func NewCmdCreateClusterQuota(name, fullName string, f *clientcmd.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CreateClusterQuotaOptions{
		CreateSubcommandOptions: NewCreateSubcommandOptions(streams),
	}
	cmd := &cobra.Command{
		Use:     name + " NAME --project-label-selector=key=value [--hard=RESOURCE=QUANTITY]...",
		Short:   "Create cluster resource quota resource.",
		Long:    clusterQuotaLong,
		Example: fmt.Sprintf(clusterQuotaExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(cmd, f, args))
			cmdutil.CheckErr(o.Run())
		},
		Aliases: []string{"clusterquota"},
	}
	cmd.Flags().StringVar(&o.LabelSelectorStr, "project-label-selector", o.LabelSelectorStr, "The project label selector for the cluster resource quota")
	cmd.Flags().StringVar(&o.AnnotationSelectorStr, "project-annotation-selector", o.AnnotationSelectorStr, "The project annotation selector for the cluster resource quota")
	cmd.Flags().StringSliceVar(&o.Hard, "hard", o.Hard, "The resource to constrain: RESOURCE=QUANTITY (pods=10)")

	o.PrintFlags.AddFlags(cmd)
	cmdutil.AddDryRunFlag(cmd)

	return cmd
}

func (o *CreateClusterQuotaOptions) Complete(cmd *cobra.Command, f *clientcmd.Factory, args []string) error {
	var err error
	if len(o.LabelSelectorStr) > 0 {
		o.LabelSelector, err = metav1.ParseToLabelSelector(o.LabelSelectorStr)
		if err != nil {
			return err
		}
	}

	o.AnnotationSelector, err = parseAnnotationSelector(o.AnnotationSelectorStr)
	if err != nil {
		return err
	}

	clientConfig, err := f.ClientConfig()
	if err != nil {
		return err
	}
	o.Client, err = quotav1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	return o.CreateSubcommandOptions.Complete(f, cmd, args)
}

func (o *CreateClusterQuotaOptions) Run() error {
	clusterQuota := &quotav1.ClusterResourceQuota{
		// this is ok because we know exactly how we want to be serialized
		TypeMeta:   metav1.TypeMeta{APIVersion: quotav1.SchemeGroupVersion.String(), Kind: "ClusterResourceQuota"},
		ObjectMeta: metav1.ObjectMeta{Name: o.Name},
		Spec: quotav1.ClusterResourceQuotaSpec{
			Selector: quotav1.ClusterResourceQuotaSelector{
				LabelSelector:      o.LabelSelector,
				AnnotationSelector: o.AnnotationSelector,
			},
			Quota: kapiv1.ResourceQuotaSpec{
				Hard: kapiv1.ResourceList{},
			},
		},
	}

	for _, resourceCount := range o.Hard {
		tokens := strings.Split(resourceCount, "=")
		if len(tokens) != 2 {
			return fmt.Errorf("%v must in the form of resource=quantity", resourceCount)
		}
		quantity, err := resource.ParseQuantity(tokens[1])
		if err != nil {
			return err
		}
		clusterQuota.Spec.Quota.Hard[kapiv1.ResourceName(tokens[0])] = quantity
	}

	if !o.DryRun {
		var err error
		clusterQuota, err = o.Client.ClusterResourceQuotas().Create(clusterQuota)
		if err != nil {
			return err
		}
	}

	return o.PrintObj(clusterQuota)
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

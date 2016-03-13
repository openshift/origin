package quota

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/describe"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	imageapi "github.com/openshift/origin/pkg/image/api"
	imagequota "github.com/openshift/origin/pkg/quota/image"
)

const QuotaRecommendedName = "quota"

const quotaLong = `todo`

func NewCommandQuota(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	opts := &QuotaOptions{}
	cmd := &cobra.Command{
		Use:   name,
		Short: "todo",
		Long:  quotaLong,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(opts.Complete(f, cmd, args, out))
			kcmdutil.CheckErr(opts.Validate(cmd))
			kcmdutil.CheckErr(opts.Run())
		},
	}

	return cmd
}

type QuotaOptions struct {
	// internal values
	Namespace string

	// helpers
	out      io.Writer
	osClient client.Interface
	kClient  kclient.Interface
}

// Complete turns a partially defined QuotaOptions into a solvent structure
// which can be validated and used for showing quota usage.
func (o *QuotaOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, out io.Writer) error {
	osClient, kClient, err := f.Clients()
	if err != nil {
		return err
	}
	o.Namespace = cmd.Flag("namespace").Value.String()

	o.osClient = osClient
	o.kClient = kClient
	o.out = out

	return nil
}

// Validate ensures that a QuotaOptions is valid and can be used to execute command.
func (o *QuotaOptions) Validate(cmd *cobra.Command) error {
	return nil
}

// Run contains all the necessary functionality to show current quota usage.
func (o *QuotaOptions) Run() error {
	opts := kapi.ListOptions{}
	if len(o.Namespace) != 0 {
		opts.FieldSelector = fields.SelectorFromSet(fields.Set(map[string]string{"metadata.name": o.Namespace}))
	}
	nss, err := o.kClient.Namespaces().List(opts)
	if err != nil {
		return err
	}

	for _, ns := range nss.Items {
		total := int64(0)
		fmt.Fprintf(o.out, "Namespace: %s\n", ns.Name)
		quotas, err := o.kClient.ResourceQuotas(ns.Name).List(kapi.ListOptions{})
		if err != nil {
			return err
		}
		fmt.Fprintf(o.out, "Quotas:\n")
		for _, quota := range quotas.Items {
			fmt.Fprintf(o.out, "%s:\n", quota.Name)
			fmt.Fprintf(o.out, formatQuota(quota, imageapi.ResourceProjectImagesSize))
			fmt.Fprintf(o.out, formatQuota(quota, imageapi.ResourceImageStreamSize))
			fmt.Fprintf(o.out, formatQuota(quota, imageapi.ResourceImageSize))
		}
		if len(quotas.Items) == 0 {
			fmt.Fprintf(o.out, "- <none>\n")
		}

		fmt.Fprintf(o.out, "Image Streams:\n")
		iss, err := o.osClient.ImageStreams(ns.Name).List(kapi.ListOptions{})
		if err != nil {
			return err
		}
		for _, is := range iss.Items {
			quantity := imagequota.GetImageStreamSize(o.osClient, &is, make(map[string]*imageapi.Image))
			fmt.Fprintf(o.out, "- %s\t\t%s\n", is.Name, describe.FormatQuantity(quantity, describe.Giga))
			total += quantity.Value()
		}
		if len(iss.Items) == 0 {
			fmt.Fprintf(o.out, "- <none>\n")
		}
		fmt.Fprintf(o.out, "Total: %s\n\n", describe.FormatQuantity(
			resource.NewQuantity(total, resource.BinarySI), describe.Giga))
	}
	return nil
}

func formatQuota(quota kapi.ResourceQuota, resource kapi.ResourceName) string {
	if value, ok := quota.Spec.Hard[resource]; ok {
		return fmt.Sprintf("- %v\t\t%s\n", resource, describe.FormatQuantity(&value, describe.Giga))
	}
	return fmt.Sprintf("- %v\t\t<unset>\n", resource)
}

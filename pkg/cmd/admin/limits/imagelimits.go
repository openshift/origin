package limits

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

const ImageLimitsRecommendedName = "image-limits"

func NewCommandImageLimits(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	opts := &ImageLimitsOptions{}
	cmd := &cobra.Command{
		Use:   name,
		Short: "Lists information about Image sizes and defined Image limits",
		Long:  "Lists information about Image sizes and defined Image limits",
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(opts.Complete(f, cmd, args, out))
			kcmdutil.CheckErr(opts.Validate(cmd))
			kcmdutil.CheckErr(opts.Run())
		},
	}

	return cmd
}

type ImageLimitsOptions struct {
	// internal values
	Namespace string

	// helpers
	out      io.Writer
	osClient client.Interface
	kClient  kclient.Interface
}

// Complete turns a partially defined ImageLimitsOptions into a solvent structure
// which can be validated and used for showing limits usage.
func (o *ImageLimitsOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, out io.Writer) error {
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

// Validate ensures that a ImageLimitsOptions is valid and can be used to execute command.
func (o ImageLimitsOptions) Validate(cmd *cobra.Command) error {
	return nil
}

// Run contains all the necessary functionality to show current image limits usage.
func (o ImageLimitsOptions) Run() error {
	s, err := tabbedString(func(out *tabwriter.Writer) error {
		opts := kapi.ListOptions{}
		if len(o.Namespace) != 0 {
			opts.FieldSelector = fields.SelectorFromSet(fields.Set(map[string]string{"metadata.name": o.Namespace}))
		}
		nss, err := o.kClient.Namespaces().List(opts)
		if err != nil {
			return err
		}

		for _, ns := range nss.Items {
			fmt.Fprintf(out, "Namespace:\t%s\n", ns.Name)

			limits, err := o.kClient.LimitRanges(ns.Name).List(kapi.ListOptions{})
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "LimitRanges:\n")
			if len(limits.Items) == 0 {
				fmt.Fprintf(out, "- <none>\n")
			}
			for _, limit := range limits.Items {
				fmt.Fprintf(out, "- %s\t%s\n", limit.Name, getLimit(limit, imageapi.LimitTypeImage))
			}

			iss, err := o.osClient.ImageStreams(ns.Name).List(kapi.ListOptions{})
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "Image Streams:\n")
			if len(iss.Items) == 0 {
				fmt.Fprintf(out, "- <none>\n")
			}
			for _, is := range iss.Items {
				list := []string{}
				for name, tag := range is.Status.Tags {
					list = append(list, fmt.Sprintf("%s (%s)", name, o.getTagSize(&tag)))
				}
				tags := strings.Join(list, ", ")
				fmt.Fprintf(out, "- %s\t%s\n", is.Name, tags)
			}

			fmt.Fprint(out, "\n")
		}
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(o.out, "%s", s)
	return nil
}

// getTagSize reads single tag size.
func (o *ImageLimitsOptions) getTagSize(tagInfo *imageapi.TagEventList) string {
	if len(tagInfo.Items) == 0 {
		return "0"
	}
	// only the first entry is current tag, the rest is tag history
	tag := tagInfo.Items[0]
	image, err := o.osClient.Images().Get(tag.Image)
	if err != nil {
		return "0"
	}
	// TODO read size with Stat if this is 0
	return formatSize(image.DockerImageMetadata.Size)
}

// getLimit reads specific limit type from the list of Limits specified in a
// LimitRange object.
func getLimit(limitRange kapi.LimitRange, limitType kapi.LimitType) string {
	for _, limit := range limitRange.Spec.Limits {
		if limit.Type != limitType {
			continue
		}
		limitQuantity, ok := limit.Max[kapi.ResourceStorage]
		if !ok {
			continue
		}
		return limitQuantity.String()
	}
	return fmt.Sprintf("<unset>")
}

type scale struct {
	scale uint64
	unit  string
}

var (
	mega = scale{20, "MiB"}
	giga = scale{30, "GiB"}
)

// formatSize prints size choosing scale based on the passed number. Manual scaling
// is done here to make sure we print correct binary values for image size.
func formatSize(size int64) string {
	scale := mega
	if size >= (1 << 30) {
		scale = giga
	}
	integer := size >> scale.scale
	// fraction is the reminder of a division shifted by one order of magnitude
	fraction := (size % (1 << scale.scale)) >> (scale.scale - 10)
	// additionally we present only 2 digits after dot, so divide by 10
	fraction = fraction / 10
	if fraction > 0 {
		return fmt.Sprintf("%d.%02d%s", integer, fraction, scale.unit)
	}
	return fmt.Sprintf("%d%s", integer, scale.unit)
}

func tabbedString(f func(*tabwriter.Writer) error) (string, error) {
	out := new(tabwriter.Writer)
	buf := &bytes.Buffer{}
	out.Init(buf, 0, 8, 1, '\t', 0)

	err := f(out)
	if err != nil {
		return "", err
	}

	out.Flush()
	str := string(buf.String())
	return str, nil
}

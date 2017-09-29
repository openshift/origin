package images

import (
	"fmt"
	"io"
	"strings"

	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/index"
	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/kubectl"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

type ImageIndexOptions struct {
	Out io.Writer

	Client osclient.Interface

	ImageName   string
	Ancestors   bool
	Descendants bool
	Siblings    bool
	Printer     kubectl.ResourcePrinter
}

const imagesLong = `TBD`

func NewCmdImages(fullName string, f *clientcmd.Factory, out, errout io.Writer) *cobra.Command {
	o := &ImageIndexOptions{Out: out}

	cmd := &cobra.Command{
		Use:   "images IMAGE [--ancestors|--descendants|--siblings]",
		Short: "Query images",
		Long:  imagesLong,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().BoolVarP(&o.Ancestors, "ancestors", "a", false, "Query the given image ancestors")
	cmd.Flags().BoolVarP(&o.Descendants, "descendants", "d", false, "Query the given image descendants")
	cmd.Flags().BoolVarP(&o.Siblings, "siblings", "s", false, "Query the given image siblings")

	return cmd
}

func (o *ImageIndexOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("exactly one IMAGE is allowed")
	}
	o.ImageName = args[0]

	client, _, _, err := f.Clients()
	if err != nil {
		return err
	}
	o.Client = client
	o.Printer, _ = f.Printer(nil, kubectl.PrintOptions{})

	return nil
}

func (o *ImageIndexOptions) Validate() error {
	if !strings.HasPrefix(o.ImageName, "sha256:") {
		return fmt.Errorf(`the image must start with "sha256:" prefix`)
	}
	return nil
}

func (o *ImageIndexOptions) Run() error {
	image, err := o.Client.Images().Get(o.ImageName)
	if err != nil {
		return err
	}

	stopChan := make(chan struct{})
	i := index.NewImageIndex(o.Client.Images(), stopChan)
	i.WaitForSyncedStores()

	list := imageapi.ImageList{Items: []imageapi.Image{}}
	var items []*imageapi.Image

	if o.Ancestors {
		items, err = i.Ancestors(image)
		for _, i := range items {
			list.Items = append(list.Items, *i)
		}
	}
	if o.Descendants {
		items, err = i.Descendants(image)
		for _, i := range items {
			list.Items = append(list.Items, *i)
		}
	}
	if o.Siblings {
		items, err = i.Siblings(image)
		for _, i := range items {
			list.Items = append(list.Items, *i)
		}
	}
	if err != nil || len(items) == 0 {
		return err
	}

	o.Printer.PrintObj(&list, o.Out)

	return nil
}

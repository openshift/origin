package release

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	digest "github.com/opencontainers/go-digest"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	"github.com/openshift/origin/pkg/image/apis/image/docker10"
	imagereference "github.com/openshift/origin/pkg/image/apis/image/reference"
	"github.com/openshift/origin/pkg/oc/cli/image/extract"
)

func NewExtractOptions(streams genericclioptions.IOStreams) *ExtractOptions {
	return &ExtractOptions{
		IOStreams: streams,
		Directory: ".",
	}
}

func NewExtract(f kcmdutil.Factory, parentName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewExtractOptions(streams)
	cmd := &cobra.Command{
		Use:   "extract",
		Short: "Extract the contents of an update payload to disk",
		Long: templates.LongDesc(`
			Extracts the contents of an OpenShift update payload to disk for inspection or
			debugging.

			Experimental: This command is under active development and may change without notice.
		`),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(cmd, args))
			kcmdutil.CheckErr(o.Run())
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&o.From, "from", o.From, "Image containing the release payload.")
	flags.StringVar(&o.File, "file", o.File, "Extract a single file from the payload to standard output.")
	flags.StringVar(&o.Directory, "to", o.Directory, "Directory to write release contents to, defaults to the current directory.")
	return cmd
}

type ExtractOptions struct {
	genericclioptions.IOStreams

	From string

	Directory string
	File      string
}

func (o *ExtractOptions) Complete(cmd *cobra.Command, args []string) error {
	return nil
}

func (o *ExtractOptions) Run() error {
	if len(o.From) == 0 {
		return fmt.Errorf("must specify an image containing a release payload with --from")
	}
	if o.Directory != "." && len(o.File) > 0 {
		return fmt.Errorf("only one of --to and --file may be set")
	}

	dir := o.Directory
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	src := o.From
	ref, err := imagereference.Parse(src)
	if err != nil {
		return err
	}
	opts := extract.NewOptions(genericclioptions.IOStreams{Out: o.Out, ErrOut: o.ErrOut})

	switch {
	case len(o.File) > 0:
		opts.OnlyFiles = true
		opts.RemovePermissions = true
		opts.Mappings = []extract.Mapping{
			{
				ImageRef: ref,

				From: "release-manifests/",
				To:   dir,
			},
		}
		found := false
		opts.TarEntryCallback = func(hdr *tar.Header, _ extract.LayerInfo, r io.Reader) (bool, error) {
			if hdr.Name != o.File {
				return true, nil
			}
			if _, err := io.Copy(o.Out, r); err != nil {
				return false, err
			}
			found = true
			return false, nil
		}
		if err := opts.Run(); err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("image did not contain %s", o.File)
		}
		return nil

	default:
		opts.OnlyFiles = true
		opts.RemovePermissions = true
		opts.Mappings = []extract.Mapping{
			{
				ImageRef: ref,

				From: "release-manifests/",
				To:   dir,
			},
		}
		opts.ImageMetadataCallback = func(m *extract.Mapping, dgst digest.Digest, config *docker10.DockerImageConfig) {
			if len(ref.ID) > 0 {
				fmt.Fprintf(o.Out, "Extracted release payload created at %s\n", config.Created.Format(time.RFC3339))
			} else {
				fmt.Fprintf(o.Out, "Extracted release payload from digest %s created at %s\n", dgst, config.Created.Format(time.RFC3339))
			}
		}
		return opts.Run()
	}
}

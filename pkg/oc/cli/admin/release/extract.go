package release

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/golang/glog"

	"github.com/spf13/cobra"

	digest "github.com/opencontainers/go-digest"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	configv1client "github.com/openshift/client-go/config/clientset/versioned"
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
			Extract the contents of a release image to disk

			Extracts the contents of an OpenShift update image to disk for inspection or
			debugging. Update images contain manifests and metadata about the operators that
			must be installed on the cluster for a given version.

			Instead of extracting the manifests, you can specify --git=DIR to perform a Git
			checkout of the source code that comprises the release. A warning will be printed
			if the component is not associated with source code. The command will not perform
			any destructive actions on your behalf except for executing a 'git checkout' which
			may change the current branch. Requires 'git' to be on your path.

			Experimental: This command is under active development and may change without notice.
		`),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Run())
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&o.RegistryConfig, "registry-config", "a", o.RegistryConfig, "Path to your registry credentials (defaults to ~/.docker/config.json)")
	flags.StringVar(&o.GitExtractDir, "git", o.GitExtractDir, "Check out the sources that created this release into the provided dir. Repos will be created at <dir>/<host>/<path>. Requires 'git' on your path.")
	flags.StringVar(&o.From, "from", o.From, "Image containing the release payload.")
	flags.StringVar(&o.File, "file", o.File, "Extract a single file from the payload to standard output.")
	flags.StringVar(&o.Directory, "to", o.Directory, "Directory to write release contents to, defaults to the current directory.")
	return cmd
}

type ExtractOptions struct {
	genericclioptions.IOStreams

	From string

	// GitExtractDir is the path of a root directory to extract the source of a release to.
	GitExtractDir string

	Directory string
	File      string

	RegistryConfig string

	ImageMetadataCallback func(m *extract.Mapping, dgst digest.Digest, config *docker10.DockerImageConfig)
}

func (o *ExtractOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	switch {
	case len(args) == 0 && len(o.From) == 0:
		cfg, err := f.ToRESTConfig()
		if err != nil {
			return fmt.Errorf("info expects one argument, or a connection to an OpenShift 4.x server: %v", err)
		}
		client, err := configv1client.NewForConfig(cfg)
		if err != nil {
			return fmt.Errorf("info expects one argument, or a connection to an OpenShift 4.x server: %v", err)
		}
		cv, err := client.Config().ClusterVersions().Get("version", metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return fmt.Errorf("you must be connected to an OpenShift 4.x server to fetch the current version")
			}
			return fmt.Errorf("info expects one argument, or a connection to an OpenShift 4.x server: %v", err)
		}
		image := cv.Status.Desired.Image
		if len(image) == 0 && cv.Spec.DesiredUpdate != nil {
			image = cv.Spec.DesiredUpdate.Image
		}
		if len(image) == 0 {
			return fmt.Errorf("the server is not reporting a release image at this time, please specify an image to extract")
		}
		o.From = image

	case len(args) == 1 && len(o.From) > 0, len(args) > 1:
		return fmt.Errorf("you may only specify a single image via --from or argument")

	case len(args) == 1:
		o.From = args[0]
	}
	return nil
}

func (o *ExtractOptions) Run() error {
	if len(o.From) == 0 {
		return fmt.Errorf("must specify an image containing a release payload with --from")
	}
	if o.Directory != "." && len(o.File) > 0 {
		return fmt.Errorf("only one of --to and --file may be set")
	}

	if len(o.GitExtractDir) > 0 {
		return o.extractGit(o.GitExtractDir)
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
	opts.RegistryConfig = o.RegistryConfig

	switch {
	case len(o.File) > 0:
		if o.ImageMetadataCallback != nil {
			opts.ImageMetadataCallback = o.ImageMetadataCallback
		}
		opts.OnlyFiles = true
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
		opts.Mappings = []extract.Mapping{
			{
				ImageRef: ref,

				From: "release-manifests/",
				To:   dir,
			},
		}
		opts.ImageMetadataCallback = func(m *extract.Mapping, dgst digest.Digest, config *docker10.DockerImageConfig) {
			if o.ImageMetadataCallback != nil {
				o.ImageMetadataCallback(m, dgst, config)
			}
			if len(ref.ID) > 0 {
				fmt.Fprintf(o.Out, "Extracted release payload created at %s\n", config.Created.Format(time.RFC3339))
			} else {
				fmt.Fprintf(o.Out, "Extracted release payload from digest %s created at %s\n", dgst, config.Created.Format(time.RFC3339))
			}
		}
		return opts.Run()
	}
}

func (o *ExtractOptions) extractGit(dir string) error {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	release, err := NewInfoOptions(o.IOStreams).LoadReleaseInfo(o.From)
	if err != nil {
		return err
	}

	cloner := &git{}

	hadErrors := false
	alreadyExtracted := make(map[string]string)
	for _, ref := range release.References.Spec.Tags {
		repo := ref.Annotations[annotationBuildSourceLocation]
		commit := ref.Annotations[annotationBuildSourceCommit]
		if len(repo) == 0 || len(commit) == 0 {
			if glog.V(2) {
				glog.Infof("Tag %s has no source info", ref.Name)
			} else {
				fmt.Fprintf(o.ErrOut, "warning: Tag %s has no source info\n", ref.Name)
			}
			continue
		}
		if oldCommit, ok := alreadyExtracted[repo]; ok {
			if oldCommit != commit {
				fmt.Fprintf(o.ErrOut, "warning: Repo %s referenced more than once with different commits, only checking out the first reference\n", repo)
			}
			continue
		}
		alreadyExtracted[repo] = commit

		basePath, err := sourceLocationAsRelativePath(dir, repo)
		if err != nil {
			return err
		}

		var extractedRepo *git
		fi, err := os.Stat(basePath)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			if err := os.MkdirAll(basePath, 0750); err != nil {
				return err
			}
		} else {
			if !fi.IsDir() {
				return fmt.Errorf("repo path %s is not a directory", basePath)
			}
		}
		extractedRepo, err = cloner.ChangeContext(basePath)
		if err != nil {
			if err != noSuchRepo {
				return err
			}
			glog.V(2).Infof("Cloning %s ...", repo)
			if err := extractedRepo.Clone(repo, o.Out, o.ErrOut); err != nil {
				hadErrors = true
				fmt.Fprintf(o.ErrOut, "error: cloning %s: %v\n", repo, err)
				continue
			}
		}
		glog.V(2).Infof("Checkout %s from %s ...", commit, repo)
		if err := extractedRepo.CheckoutCommit(repo, commit); err != nil {
			hadErrors = true
			fmt.Fprintf(o.ErrOut, "error: checking out commit for %s: %v\n", repo, err)
			continue
		}
	}
	if hadErrors {
		return kcmdutil.ErrExit
	}
	return nil
}

package prune

import (
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/prune"
	"github.com/spf13/cobra"
)

const imagesLongDesc = `%s %s - prunes images`

const PruneImagesRecommendedName = "images"

type pruneImagesConfig struct {
	Confirm             bool
	KeepYoungerThan     time.Duration
	KeepTagRevisions    int
	CABundle            string
	RegistryUrlOverride string
}

func NewCmdPruneImages(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	cfg := &pruneImagesConfig{
		Confirm:          false,
		KeepYoungerThan:  60 * time.Minute,
		KeepTagRevisions: 3,
	}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Remove unreferenced images",
		Long:  fmt.Sprintf(imagesLongDesc, parentName, name),

		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				glog.Fatal("No arguments are allowed to this command")
			}

			osClient, kClient, registryClient, err := getClients(f, cfg)
			cmdutil.CheckErr(err)

			allImages, err := osClient.Images().List(labels.Everything(), fields.Everything())
			cmdutil.CheckErr(err)

			allStreams, err := osClient.ImageStreams(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
			cmdutil.CheckErr(err)

			allPods, err := kClient.Pods(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
			cmdutil.CheckErr(err)

			allRCs, err := kClient.ReplicationControllers(kapi.NamespaceAll).List(labels.Everything())
			cmdutil.CheckErr(err)

			allBCs, err := osClient.BuildConfigs(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
			cmdutil.CheckErr(err)

			allBuilds, err := osClient.Builds(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
			cmdutil.CheckErr(err)

			allDCs, err := osClient.DeploymentConfigs(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
			cmdutil.CheckErr(err)

			dryRun := cfg.Confirm == false

			options := prune.ImageRegistryPrunerOptions{
				KeepYoungerThan:  cfg.KeepYoungerThan,
				KeepTagRevisions: cfg.KeepTagRevisions,
				Images:           allImages,
				Streams:          allStreams,
				Pods:             allPods,
				RCs:              allRCs,
				BCs:              allBCs,
				Builds:           allBuilds,
				DCs:              allDCs,
				DryRun:           dryRun,
				RegistryClient:   registryClient,
				RegistryURL:      cfg.RegistryUrlOverride,
			}
			pruner := prune.NewImageRegistryPruner(options)

			// this tabwriter is used by the describing*Pruners below for their output
			w := tabwriter.NewWriter(out, 10, 4, 3, ' ', 0)
			defer w.Flush()

			imagePruner := &describingImagePruner{w: w}
			imageStreamPruner := &describingImageStreamPruner{w: w}
			layerPruner := &describingLayerPruner{w: w}
			blobPruner := &describingBlobPruner{w: w}
			manifestPruner := &describingManifestPruner{w: w}

			switch cfg.Confirm {
			case true:
				imagePruner.delegate = prune.NewDeletingImagePruner(osClient.Images())
				imageStreamPruner.delegate = prune.NewDeletingImageStreamPruner(osClient)
				layerPruner.delegate = prune.NewDeletingLayerPruner()
				blobPruner.delegate = prune.NewDeletingBlobPruner()
				manifestPruner.delegate = prune.NewDeletingManifestPruner()
			default:
				fmt.Fprintln(os.Stderr, "Dry run enabled - no modifications will be made. Add --confirm to remove images")
			}

			err = pruner.Prune(imagePruner, imageStreamPruner, layerPruner, blobPruner, manifestPruner)
			cmdutil.CheckErr(err)
		},
	}

	cmd.Flags().BoolVar(&cfg.Confirm, "confirm", cfg.Confirm, "Specify that image pruning should proceed. Defaults to false, displaying what would be deleted but not actually deleting anything.")
	cmd.Flags().DurationVar(&cfg.KeepYoungerThan, "keep-younger-than", cfg.KeepYoungerThan, "Specify the minimum age of a build for it to be considered a candidate for pruning.")
	cmd.Flags().IntVar(&cfg.KeepTagRevisions, "keep-tag-revisions", cfg.KeepTagRevisions, "Specify the number of image revisions for a tag in an image stream that will be preserved.")
	cmd.Flags().StringVar(&cfg.CABundle, "certificate-authority", cfg.CABundle, "The path to a certificate authority bundle to use when communicating with the managed Docker registries. Defaults to the certificate authority data from the current user's config file.")
	cmd.Flags().StringVar(&cfg.RegistryUrlOverride, "registry-url", cfg.RegistryUrlOverride, "The address to use when contacting the registry, instead of using the default value. This is useful if you can't resolve or reach the registry (e.g.; the default is a cluster-internal URL) but you do have an alternative route that works.")

	return cmd
}

// describingImageStreamPruner prints information about each image stream update.
// If a delegate exists, its PruneImageStream function is invoked prior to returning.
type describingImageStreamPruner struct {
	w             io.Writer
	delegate      prune.ImageStreamPruner
	headerPrinted bool
}

var _ prune.ImageStreamPruner = &describingImageStreamPruner{}

func (p *describingImageStreamPruner) PruneImageStream(stream *imageapi.ImageStream, image *imageapi.Image, updatedTags []string) (*imageapi.ImageStream, error) {
	if !p.headerPrinted {
		p.headerPrinted = true
		fmt.Fprintln(p.w, "Deleting references from image streams to images ...")
		fmt.Fprintln(p.w, "STREAM\tIMAGE\tTAGS")
	}

	fmt.Fprintf(p.w, "%s/%s\t%s\t%s\n", stream.Namespace, stream.Name, image.Name, strings.Join(updatedTags, ", "))

	if p.delegate == nil {
		return stream, nil
	}

	updatedStream, err := p.delegate.PruneImageStream(stream, image, updatedTags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error updating image stream %s/%s to remove references to image %s: %v\n", stream.Namespace, stream.Name, image.Name, err)
	}

	return updatedStream, err
}

// describingImagePruner prints information about each image being deleted.
// If a delegate exists, its PruneImage function is invoked prior to returning.
type describingImagePruner struct {
	w             io.Writer
	delegate      prune.ImagePruner
	headerPrinted bool
}

var _ prune.ImagePruner = &describingImagePruner{}

func (p *describingImagePruner) PruneImage(image *imageapi.Image) error {
	if !p.headerPrinted {
		p.headerPrinted = true
		fmt.Fprintln(p.w, "\nDeleting images from server ...")
		fmt.Fprintln(p.w, "IMAGE")
	}

	fmt.Fprintf(p.w, "%s\n", image.Name)

	if p.delegate == nil {
		return nil
	}

	err := p.delegate.PruneImage(image)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error deleting image %s from server: %v\n", image.Name, err)
	}

	return err
}

// describingLayerPruner prints information about each repo layer link being
// deleted. If a delegate exists, its PruneLayer function is invoked prior to
// returning.
type describingLayerPruner struct {
	w             io.Writer
	delegate      prune.LayerPruner
	headerPrinted bool
}

var _ prune.LayerPruner = &describingLayerPruner{}

func (p *describingLayerPruner) PruneLayer(registryClient *http.Client, registryURL, repo, layer string) error {
	if !p.headerPrinted {
		p.headerPrinted = true
		fmt.Fprintln(p.w, "\nDeleting registry repository layer links ...")
		fmt.Fprintln(p.w, "REPO\tLAYER")
	}

	fmt.Fprintf(p.w, "%s\t%s\n", repo, layer)

	if p.delegate == nil {
		return nil
	}

	err := p.delegate.PruneLayer(registryClient, registryURL, repo, layer)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error deleting repository %s layer link %s from the registry: %v\n", repo, layer, err)
	}

	return err
}

// describingBlobPruner prints information about each blob being deleted. If a
// delegate exists, its PruneBlob function is invoked prior to returning.
type describingBlobPruner struct {
	w             io.Writer
	delegate      prune.BlobPruner
	headerPrinted bool
}

var _ prune.BlobPruner = &describingBlobPruner{}

func (p *describingBlobPruner) PruneBlob(registryClient *http.Client, registryURL, layer string) error {
	if !p.headerPrinted {
		p.headerPrinted = true
		fmt.Fprintln(p.w, "\nDeleting registry layer blobs ...")
		fmt.Fprintln(p.w, "BLOB")
	}

	fmt.Fprintf(p.w, "%s\n", layer)

	if p.delegate == nil {
		return nil
	}

	err := p.delegate.PruneBlob(registryClient, registryURL, layer)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error deleting blob %s from the registry: %v\n", layer, err)
	}

	return err
}

// describingManifestPruner prints information about each repo manifest being
// deleted. If a delegate exists, its PruneManifest function is invoked prior
// to returning.
type describingManifestPruner struct {
	w             io.Writer
	delegate      prune.ManifestPruner
	headerPrinted bool
}

var _ prune.ManifestPruner = &describingManifestPruner{}

func (p *describingManifestPruner) PruneManifest(registryClient *http.Client, registryURL, repo, manifest string) error {
	if !p.headerPrinted {
		p.headerPrinted = true
		fmt.Fprintln(p.w, "\nDeleting registry repository manifest data ...")
		fmt.Fprintln(p.w, "REPO\tIMAGE")
	}

	fmt.Fprintf(p.w, "%s\t%s\n", repo, manifest)

	if p.delegate == nil {
		return nil
	}

	err := p.delegate.PruneManifest(registryClient, registryURL, repo, manifest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error deleting data for repository %s image manifest %s from the registry: %v\n", repo, manifest, err)
	}

	return err
}

// getClients returns a Kube client, OpenShift client, and registry client.
func getClients(f *clientcmd.Factory, cfg *pruneImagesConfig) (*client.Client, *kclient.Client, *http.Client, error) {
	clientConfig, err := f.OpenShiftClientConfig.ClientConfig()
	if err != nil {
		return nil, nil, nil, err
	}

	var (
		token          string
		osClient       *client.Client
		kClient        *kclient.Client
		registryClient *http.Client
	)

	switch {
	case len(clientConfig.BearerToken) > 0:
		osClient, kClient, err = f.Clients()
		if err != nil {
			return nil, nil, nil, err
		}
		token = clientConfig.BearerToken
	default:
		err = errors.New("You must use a client config with a token")
		return nil, nil, nil, err
	}

	// copy the config
	registryClientConfig := *clientConfig

	// zero out everything we don't want to use
	registryClientConfig.BearerToken = ""
	registryClientConfig.CertFile = ""
	registryClientConfig.CertData = []byte{}
	registryClientConfig.KeyFile = ""
	registryClientConfig.KeyData = []byte{}

	// we have to set a username to something for the Docker login
	// but it's not actually used
	registryClientConfig.Username = "unused"

	// set the "password" to be the token
	registryClientConfig.Password = token

	tlsConfig, err := kclient.TLSConfigFor(&registryClientConfig)
	if err != nil {
		return nil, nil, nil, err
	}

	// if the user specified a CA on the command line, add it to the
	// client config's CA roots
	if len(cfg.CABundle) > 0 {
		data, err := ioutil.ReadFile(cfg.CABundle)
		if err != nil {
			return nil, nil, nil, err
		}

		if tlsConfig.RootCAs == nil {
			tlsConfig.RootCAs = x509.NewCertPool()
		}

		tlsConfig.RootCAs.AppendCertsFromPEM(data)
	}

	transport := http.Transport{
		TLSClientConfig: tlsConfig,
	}

	wrappedTransport, err := kclient.HTTPWrappersForConfig(&registryClientConfig, &transport)
	if err != nil {
		return nil, nil, nil, err
	}

	registryClient = &http.Client{
		Transport: wrappedTransport,
	}

	return osClient, kClient, registryClient, nil
}

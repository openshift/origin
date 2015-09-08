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

	"github.com/spf13/cobra"
	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/prune"
	oserrors "github.com/openshift/origin/pkg/util/errors"
)

const (
	imagesLongDesc = `%s %s - prunes images`
	// PruneImagesRecommendedName is the recommended command name
	PruneImagesRecommendedName = "images"
)

// PruneImagesOptions holds all the required options for prune images
type PruneImagesOptions struct {
	Pruner prune.ImageRegistryPruner
	Client client.Interface
	Out    io.Writer

	Confirm          bool
	KeepYoungerThan  time.Duration
	KeepTagRevisions int

	CABundle            string
	RegistryUrlOverride string
}

// NewCmdPruneImages implements the OpenShift cli prune images command
func NewCmdPruneImages(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	opts := &PruneImagesOptions{
		Confirm:          false,
		KeepYoungerThan:  60 * time.Minute,
		KeepTagRevisions: 3,
	}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Remove unreferenced images",
		Long:  fmt.Sprintf(imagesLongDesc, parentName, name),

		Run: func(cmd *cobra.Command, args []string) {
			if err := opts.Complete(f, args, out); err != nil {
				cmdutil.CheckErr(err)
			}

			if err := opts.Validate(); err != nil {
				cmdutil.CheckErr(cmdutil.UsageError(cmd, err.Error()))
			}

			if err := opts.RunPruneImages(); err != nil {
				cmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().BoolVar(&opts.Confirm, "confirm", opts.Confirm, "Specify that image pruning should proceed. Defaults to false, displaying what would be deleted but not actually deleting anything.")
	cmd.Flags().DurationVar(&opts.KeepYoungerThan, "keep-younger-than", opts.KeepYoungerThan, "Specify the minimum age of an image for it to be considered a candidate for pruning.")
	cmd.Flags().IntVar(&opts.KeepTagRevisions, "keep-tag-revisions", opts.KeepTagRevisions, "Specify the number of image revisions for a tag in an image stream that will be preserved.")
	cmd.Flags().StringVar(&opts.CABundle, "certificate-authority", opts.CABundle, "The path to a certificate authority bundle to use when communicating with the managed Docker registries. Defaults to the certificate authority data from the current user's config file.")
	cmd.Flags().StringVar(&opts.RegistryUrlOverride, "registry-url", opts.RegistryUrlOverride, "The address to use when contacting the registry, instead of using the default value. This is useful if you can't resolve or reach the registry (e.g.; the default is a cluster-internal URL) but you do have an alternative route that works.")

	return cmd
}

// Complete the options for prune images
func (o *PruneImagesOptions) Complete(f *clientcmd.Factory, args []string, out io.Writer) error {
	if len(args) > 0 {
		return errors.New("no arguments are allowed to this command")
	}

	o.Out = out

	osClient, kClient, _, err := getClients(f, o.CABundle)
	if err != nil {
		return err
	}
	o.Client = osClient

	allImages, err := osClient.Images().List(labels.Everything(), fields.Everything())
	if err != nil {
		return err
	}

	allStreams, err := osClient.ImageStreams(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
	if err != nil {
		return err
	}

	allPods, err := kClient.Pods(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
	if err != nil {
		return err
	}

	allRCs, err := kClient.ReplicationControllers(kapi.NamespaceAll).List(labels.Everything())
	if err != nil {
		return err
	}

	allBCs, err := osClient.BuildConfigs(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
	// We need to tolerate 'not found' errors for buildConfigs since they may be disabled in Atomic
	err = oserrors.TolerateNotFoundError(err)
	if err != nil {
		return err
	}

	allBuilds, err := osClient.Builds(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
	// We need to tolerate 'not found' errors for builds since they may be disabled in Atomic
	err = oserrors.TolerateNotFoundError(err)
	if err != nil {
		return err
	}

	allDCs, err := osClient.DeploymentConfigs(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
	if err != nil {
		return err
	}

	options := prune.ImageRegistryPrunerOptions{
		KeepYoungerThan:  o.KeepYoungerThan,
		KeepTagRevisions: o.KeepTagRevisions,
		Images:           allImages,
		Streams:          allStreams,
		Pods:             allPods,
		RCs:              allRCs,
		BCs:              allBCs,
		Builds:           allBuilds,
		DCs:              allDCs,
		DryRun:           o.Confirm == false,
	}

	o.Pruner = prune.NewImageRegistryPruner(options)

	return nil
}

// Validate the options for prune images
func (o *PruneImagesOptions) Validate() error {
	if o.Pruner == nil && o.Confirm {
		return errors.New("an image pruner needs to be specified")
	}
	if o.Client == nil {
		return errors.New("a client needs to be specified")
	}
	if o.Out == nil {
		return errors.New("a writer needs to be specified")
	}
	return nil
}

// RunPruneImages runs the prune images cli command
func (o *PruneImagesOptions) RunPruneImages() error {
	// this tabwriter is used by the describing*Pruners below for their output
	w := tabwriter.NewWriter(o.Out, 10, 4, 3, ' ', 0)
	defer w.Flush()

	imagePruner := &describingImagePruner{w: w}
	imageStreamPruner := &describingImageStreamPruner{w: w}

	if o.Confirm {
		imagePruner.delegate = prune.NewDeletingImagePruner(o.Client.Images())
		imageStreamPruner.delegate = prune.NewDeletingImageStreamPruner(o.Client)
	} else {
		fmt.Fprintln(os.Stderr, "Dry run enabled - no modifications will be made. Add --confirm to remove images")
	}

	return o.Pruner.Prune(imagePruner, imageStreamPruner)
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

// getClients returns a Kube client, OpenShift client, and registry client.
func getClients(f *clientcmd.Factory, caBundle string) (*client.Client, *kclient.Client, *http.Client, error) {
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
	if len(caBundle) > 0 {
		data, err := ioutil.ReadFile(caBundle)
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

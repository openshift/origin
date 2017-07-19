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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knet "k8s.io/apimachinery/pkg/util/net"
	restclient "k8s.io/client-go/rest"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/prune"
	oserrors "github.com/openshift/origin/pkg/util/errors"
	"github.com/openshift/origin/pkg/util/netutils"
)

// PruneImagesRecommendedName is the recommended command name
const PruneImagesRecommendedName = "images"

var (
	imagesLongDesc = templates.LongDesc(`
		Remove image stream tags, images, and image layers by age or usage

		This command removes historical image stream tags, unused images, and unreferenced image
		layers from the integrated registry. By default, all images are considered as candidates.
		The command can be instructed to consider only images that have been directly pushed to the
		registry by supplying --all=false flag.

		By default, the prune operation performs a dry run making no changes to internal registry. A
		--confirm flag is needed for changes to be effective.

		Only a user with a cluster role %s or higher who is logged-in will be able to actually
		delete the images.

		If the registry is secured with a certificate signed by a self-signed root certificate
		authority other than the one present in current user's config, you may need to specify it
		using --certificate-authority flag.

		Insecure connection is allowed in following cases unless certificate-authority is specified:
		 1. --force-insecure is given
		 2. user's config allows for insecure connection (the user logged in to the cluster with
			--insecure-skip-tls-verify or allowed for insecure connection)
		 3. registry url is not given or it's a private/link-local address`)

	imagesExample = templates.Examples(`
	  # See, what the prune command would delete if only images more than an hour old and obsoleted
	  # by 3 newer revisions under the same tag were considered.
	  %[1]s %[2]s --keep-tag-revisions=3 --keep-younger-than=60m

	  # To actually perform the prune operation, the confirm flag must be appended
	  %[1]s %[2]s --keep-tag-revisions=3 --keep-younger-than=60m --confirm

	  # See, what the prune command would delete if we're interested in removing images
	  # exceeding currently set limit ranges ('openshift.io/Image')
	  %[1]s %[2]s --prune-over-size-limit

	  # To actually perform the prune operation, the confirm flag must be appended
	  %[1]s %[2]s --prune-over-size-limit --confirm`)
)

var (
	defaultKeepYoungerThan         = 60 * time.Minute
	defaultKeepTagRevisions        = 3
	defaultPruneImageOverSizeLimit = false
)

// PruneImagesOptions holds all the required options for pruning images.
type PruneImagesOptions struct {
	Confirm             bool
	KeepYoungerThan     *time.Duration
	KeepTagRevisions    *int
	PruneOverSizeLimit  *bool
	AllImages           *bool
	CABundle            string
	RegistryUrlOverride string
	Namespace           string
	ForceInsecure       bool

	OSClient       client.Interface
	KClient        kclientset.Interface
	RegistryClient *http.Client
	Out            io.Writer
	Insecure       bool
}

// NewCmdPruneImages implements the OpenShift cli prune images command.
func NewCmdPruneImages(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	allImages := true
	opts := &PruneImagesOptions{
		Confirm:            false,
		KeepYoungerThan:    &defaultKeepYoungerThan,
		KeepTagRevisions:   &defaultKeepTagRevisions,
		PruneOverSizeLimit: &defaultPruneImageOverSizeLimit,
		AllImages:          &allImages,
	}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Remove unreferenced images",
		Long:  fmt.Sprintf(imagesLongDesc, bootstrappolicy.ImagePrunerRoleName),

		Example: fmt.Sprintf(imagesExample, parentName, name),

		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(opts.Complete(f, cmd, args, out))
			kcmdutil.CheckErr(opts.Validate())
			kcmdutil.CheckErr(opts.Run())
		},
	}

	cmd.Flags().BoolVar(&opts.Confirm, "confirm", opts.Confirm, "If true, specify that image pruning should proceed. Defaults to false, displaying what would be deleted but not actually deleting anything.")
	cmd.Flags().BoolVar(opts.AllImages, "all", *opts.AllImages, "Include images that were not pushed to the registry but have been mirrored by pullthrough.")
	cmd.Flags().DurationVar(opts.KeepYoungerThan, "keep-younger-than", *opts.KeepYoungerThan, "Specify the minimum age of an image for it to be considered a candidate for pruning.")
	cmd.Flags().IntVar(opts.KeepTagRevisions, "keep-tag-revisions", *opts.KeepTagRevisions, "Specify the number of image revisions for a tag in an image stream that will be preserved.")
	cmd.Flags().BoolVar(opts.PruneOverSizeLimit, "prune-over-size-limit", *opts.PruneOverSizeLimit, "Specify if images which are exceeding LimitRanges (see 'openshift.io/Image'), specified in the same namespace, should be considered for pruning. This flag cannot be combined with --keep-younger-than nor --keep-tag-revisions.")
	cmd.Flags().StringVar(&opts.CABundle, "certificate-authority", opts.CABundle, "The path to a certificate authority bundle to use when communicating with the managed Docker registries. Defaults to the certificate authority data from the current user's config file. It cannot be used together with --force-insecure.")
	cmd.Flags().StringVar(&opts.RegistryUrlOverride, "registry-url", opts.RegistryUrlOverride, "The address to use when contacting the registry, instead of using the default value. This is useful if you can't resolve or reach the registry (e.g.; the default is a cluster-internal URL) but you do have an alternative route that works.")
	cmd.Flags().BoolVar(&opts.ForceInsecure, "force-insecure", opts.ForceInsecure, "If true, allow an insecure connection to the docker registry that is hosted via HTTP or has an invalid HTTPS certificate. Whenever possible, use --certificate-authority instead of this dangerous option.")

	return cmd
}

// Complete turns a partially defined PruneImagesOptions into a solvent structure
// which can be validated and used for pruning images.
func (o *PruneImagesOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, out io.Writer) error {
	if len(args) > 0 {
		return kcmdutil.UsageError(cmd, "no arguments are allowed to this command")
	}

	if !cmd.Flags().Lookup("prune-over-size-limit").Changed {
		o.PruneOverSizeLimit = nil
	} else {
		if !cmd.Flags().Lookup("keep-younger-than").Changed {
			o.KeepYoungerThan = nil
		}
		if !cmd.Flags().Lookup("keep-tag-revisions").Changed {
			o.KeepTagRevisions = nil
		}
	}

	o.Namespace = metav1.NamespaceAll
	if cmd.Flags().Lookup("namespace").Changed {
		var err error
		o.Namespace, _, err = f.DefaultNamespace()
		if err != nil {
			return err
		}
	}
	o.Out = out

	clientConfig, err := f.ClientConfig()
	if err != nil {
		return err
	}

	o.Insecure = o.ForceInsecure
	if !o.Insecure && len(o.CABundle) == 0 {
		o.Insecure = clientConfig.TLSClientConfig.Insecure || len(o.RegistryUrlOverride) == 0 || netutils.IsPrivateAddress(o.RegistryUrlOverride)
	}
	osClient, kClient, registryClient, err := getClients(f, o.CABundle, o.Insecure)
	if err != nil {
		return err
	}
	o.OSClient = osClient
	o.KClient = kClient
	o.RegistryClient = registryClient

	return nil
}

// Validate ensures that a PruneImagesOptions is valid and can be used to execute pruning.
func (o PruneImagesOptions) Validate() error {
	if o.PruneOverSizeLimit != nil && (o.KeepYoungerThan != nil || o.KeepTagRevisions != nil) {
		return fmt.Errorf("--prune-over-size-limit cannot be specified with --keep-tag-revisions nor --keep-younger-than")
	}
	if o.KeepYoungerThan != nil && *o.KeepYoungerThan < 0 {
		return fmt.Errorf("--keep-younger-than must be greater than or equal to 0")
	}
	if o.KeepTagRevisions != nil && *o.KeepTagRevisions < 0 {
		return fmt.Errorf("--keep-tag-revisions must be greater than or equal to 0")
	}
	// golang validation tighten and our code actually expects this to be scheme-less
	// TODO figure out how to validate
	// if _, err := url.Parse(o.RegistryUrlOverride); err != nil {
	// 	return fmt.Errorf("invalid --registry-url flag: %v", err)
	// }
	if o.ForceInsecure && len(o.CABundle) > 0 {
		return fmt.Errorf("--certificate-authority cannot be specified with --force-insecure")
	}
	return nil
}

// Run contains all the necessary functionality for the OpenShift cli prune images command.
func (o PruneImagesOptions) Run() error {
	allImages, err := o.OSClient.Images().List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	allStreams, err := o.OSClient.ImageStreams(o.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	allPods, err := o.KClient.Core().Pods(o.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	allRCs, err := o.KClient.Core().ReplicationControllers(o.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	allBCs, err := o.OSClient.BuildConfigs(o.Namespace).List(metav1.ListOptions{})
	// We need to tolerate 'not found' errors for buildConfigs since they may be disabled in Atomic
	err = oserrors.TolerateNotFoundError(err)
	if err != nil {
		return err
	}

	allBuilds, err := o.OSClient.Builds(o.Namespace).List(metav1.ListOptions{})
	// We need to tolerate 'not found' errors for builds since they may be disabled in Atomic
	err = oserrors.TolerateNotFoundError(err)
	if err != nil {
		return err
	}

	allDCs, err := o.OSClient.DeploymentConfigs(o.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	limitRangesList, err := o.KClient.Core().LimitRanges(o.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	limitRangesMap := make(map[string][]*kapi.LimitRange)
	for i := range limitRangesList.Items {
		limit := limitRangesList.Items[i]
		limits, ok := limitRangesMap[limit.Namespace]
		if !ok {
			limits = []*kapi.LimitRange{}
		}
		limits = append(limits, &limit)
		limitRangesMap[limit.Namespace] = limits
	}

	options := prune.PrunerOptions{
		KeepYoungerThan:    o.KeepYoungerThan,
		KeepTagRevisions:   o.KeepTagRevisions,
		PruneOverSizeLimit: o.PruneOverSizeLimit,
		AllImages:          o.AllImages,
		Images:             allImages,
		Streams:            allStreams,
		Pods:               allPods,
		RCs:                allRCs,
		BCs:                allBCs,
		Builds:             allBuilds,
		DCs:                allDCs,
		LimitRanges:        limitRangesMap,
		DryRun:             o.Confirm == false,
		RegistryClient:     o.RegistryClient,
		RegistryURL:        o.RegistryUrlOverride,
		Insecure:           o.Insecure,
	}
	if o.Namespace != metav1.NamespaceAll {
		options.Namespace = o.Namespace
	}
	pruner := prune.NewPruner(options)

	w := tabwriter.NewWriter(o.Out, 10, 4, 3, ' ', 0)
	defer w.Flush()

	imageDeleter := &describingImageDeleter{w: w}
	imageStreamDeleter := &describingImageStreamDeleter{w: w}
	layerLinkDeleter := &describingLayerLinkDeleter{w: w}
	blobDeleter := &describingBlobDeleter{w: w}
	manifestDeleter := &describingManifestDeleter{w: w}

	if o.Confirm {
		imageDeleter.delegate = prune.NewImageDeleter(o.OSClient.Images())
		imageStreamDeleter.delegate = prune.NewImageStreamDeleter(o.OSClient)
		layerLinkDeleter.delegate = prune.NewLayerLinkDeleter()
		blobDeleter.delegate = prune.NewBlobDeleter()
		manifestDeleter.delegate = prune.NewManifestDeleter()
	} else {
		fmt.Fprintln(os.Stderr, "Dry run enabled - no modifications will be made. Add --confirm to remove images")
	}

	return pruner.Prune(imageDeleter, imageStreamDeleter, layerLinkDeleter, blobDeleter, manifestDeleter)
}

// describingImageStreamDeleter prints information about each image stream update.
// If a delegate exists, its DeleteImageStream function is invoked prior to returning.
type describingImageStreamDeleter struct {
	w             io.Writer
	delegate      prune.ImageStreamDeleter
	headerPrinted bool
}

var _ prune.ImageStreamDeleter = &describingImageStreamDeleter{}

func (p *describingImageStreamDeleter) DeleteImageStream(stream *imageapi.ImageStream, image *imageapi.Image, updatedTags []string) (*imageapi.ImageStream, error) {
	if !p.headerPrinted {
		p.headerPrinted = true
		fmt.Fprintln(p.w, "Deleting references from image streams to images ...")
		fmt.Fprintln(p.w, "STREAM\tIMAGE\tTAGS")
	}

	fmt.Fprintf(p.w, "%s/%s\t%s\t%s\n", stream.Namespace, stream.Name, image.Name, strings.Join(updatedTags, ", "))

	if p.delegate == nil {
		return stream, nil
	}

	updatedStream, err := p.delegate.DeleteImageStream(stream, image, updatedTags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error updating image stream %s/%s to remove references to image %s: %v\n", stream.Namespace, stream.Name, image.Name, err)
	}

	return updatedStream, err
}

// describingImageDeleter prints information about each image being deleted.
// If a delegate exists, its DeleteImage function is invoked prior to returning.
type describingImageDeleter struct {
	w             io.Writer
	delegate      prune.ImageDeleter
	headerPrinted bool
}

var _ prune.ImageDeleter = &describingImageDeleter{}

func (p *describingImageDeleter) DeleteImage(image *imageapi.Image) error {
	if !p.headerPrinted {
		p.headerPrinted = true
		fmt.Fprintln(p.w, "\nDeleting images from server ...")
		fmt.Fprintln(p.w, "IMAGE")
	}

	fmt.Fprintf(p.w, "%s\n", image.Name)

	if p.delegate == nil {
		return nil
	}

	err := p.delegate.DeleteImage(image)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error deleting image %s from server: %v\n", image.Name, err)
	}

	return err
}

// describingLayerLinkDeleter prints information about each repo layer link being deleted. If a delegate
// exists, its DeleteLayerLink function is invoked prior to returning.
type describingLayerLinkDeleter struct {
	w             io.Writer
	delegate      prune.LayerLinkDeleter
	headerPrinted bool
}

var _ prune.LayerLinkDeleter = &describingLayerLinkDeleter{}

func (p *describingLayerLinkDeleter) DeleteLayerLink(registryClient *http.Client, registryURL, repo, name string) error {
	if !p.headerPrinted {
		p.headerPrinted = true
		fmt.Fprintln(p.w, "\nDeleting registry repository layer links ...")
		fmt.Fprintln(p.w, "REPO\tLAYER LINK")
	}

	fmt.Fprintf(p.w, "%s\t%s\n", repo, name)

	if p.delegate == nil {
		return nil
	}

	err := p.delegate.DeleteLayerLink(registryClient, registryURL, repo, name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error deleting repository %s layer link %s from the registry: %v\n", repo, name, err)
	}

	return err
}

// describingBlobDeleter prints information about each blob being deleted. If a
// delegate exists, its DeleteBlob function is invoked prior to returning.
type describingBlobDeleter struct {
	w             io.Writer
	delegate      prune.BlobDeleter
	headerPrinted bool
}

var _ prune.BlobDeleter = &describingBlobDeleter{}

func (p *describingBlobDeleter) DeleteBlob(registryClient *http.Client, registryURL, layer string) error {
	if !p.headerPrinted {
		p.headerPrinted = true
		fmt.Fprintln(p.w, "\nDeleting registry layer blobs ...")
		fmt.Fprintln(p.w, "BLOB")
	}

	fmt.Fprintf(p.w, "%s\n", layer)

	if p.delegate == nil {
		return nil
	}

	err := p.delegate.DeleteBlob(registryClient, registryURL, layer)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error deleting blob %s from the registry: %v\n", layer, err)
	}

	return err
}

// describingManifestDeleter prints information about each repo manifest being
// deleted. If a delegate exists, its DeleteManifest function is invoked prior
// to returning.
type describingManifestDeleter struct {
	w             io.Writer
	delegate      prune.ManifestDeleter
	headerPrinted bool
}

var _ prune.ManifestDeleter = &describingManifestDeleter{}

func (p *describingManifestDeleter) DeleteManifest(registryClient *http.Client, registryURL, repo, manifest string) error {
	if !p.headerPrinted {
		p.headerPrinted = true
		fmt.Fprintln(p.w, "\nDeleting registry repository manifest data ...")
		fmt.Fprintln(p.w, "REPO\tIMAGE")
	}

	fmt.Fprintf(p.w, "%s\t%s\n", repo, manifest)

	if p.delegate == nil {
		return nil
	}

	err := p.delegate.DeleteManifest(registryClient, registryURL, repo, manifest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error deleting data for repository %s image manifest %s from the registry: %v\n", repo, manifest, err)
	}

	return err
}

// getClients returns a Kube client, OpenShift client, and registry client. Note that
// registryCABundle and registryInsecure=true are mutually exclusive. If registryInsecure=true is
// specified, the ca bundle is ignored.
func getClients(f *clientcmd.Factory, registryCABundle string, registryInsecure bool) (*client.Client, kclientset.Interface, *http.Client, error) {
	clientConfig, err := f.ClientConfig()
	if err != nil {
		return nil, nil, nil, err
	}

	var (
		token          string
		osClient       *client.Client
		kClient        kclientset.Interface
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
		err = errors.New("you must use a client config with a token")
		return nil, nil, nil, err
	}

	cadata := []byte{}
	registryCABundleIncluded := false
	if len(registryCABundle) > 0 {
		cadata, err = ioutil.ReadFile(registryCABundle)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to read registry ca bundle: %v", err)
		}
	}

	// copy the config
	registryClientConfig := *clientConfig
	registryClientConfig.TLSClientConfig.Insecure = registryInsecure

	// zero out everything we don't want to use
	registryClientConfig.BearerToken = ""
	registryClientConfig.CertFile = ""
	registryClientConfig.CertData = []byte{}
	registryClientConfig.KeyFile = ""
	registryClientConfig.KeyData = []byte{}

	if registryInsecure {
		// it's not allowed to specify insecure flag together with CAs
		registryClientConfig.CAFile = ""
		registryClientConfig.CAData = []byte{}

	} else if len(cadata) > 0 && len(registryClientConfig.CAData) == 0 {
		// If given, we want to append cabundle to the resulting tlsConfig.RootCAs. However, if we
		// leave CAData unset, tlsConfig may not be created. We could append the caBundle to the
		// CAData here directly if we were ok doing a binary magic, which is not the case.
		registryClientConfig.CAData = cadata
		registryCABundleIncluded = true
	}

	// we have to set a username to something for the Docker login but it's not actually used
	registryClientConfig.Username = "unused"

	// set the "password" to be the token
	registryClientConfig.Password = token

	tlsConfig, err := restclient.TLSConfigFor(&registryClientConfig)
	if err != nil {
		return nil, nil, nil, err
	}

	// Add the CA bundle to the client config's CA roots if provided and we haven't done that already.
	// FIXME: handle registryCABundle on one place
	if tlsConfig != nil && len(cadata) > 0 && !registryCABundleIncluded && !registryInsecure {
		if tlsConfig.RootCAs == nil {
			tlsConfig.RootCAs = x509.NewCertPool()
		}
		tlsConfig.RootCAs.AppendCertsFromPEM(cadata)
	}

	transport := knet.SetTransportDefaults(&http.Transport{
		TLSClientConfig: tlsConfig,
	})

	wrappedTransport, err := restclient.HTTPWrappersForConfig(&registryClientConfig, transport)
	if err != nil {
		return nil, nil, nil, err
	}

	registryClient = &http.Client{
		Transport: wrappedTransport,
	}

	return osClient, kClient, registryClient, nil
}

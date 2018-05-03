package prune

import (
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	knet "k8s.io/apimachinery/pkg/util/net"
	discovery "k8s.io/client-go/discovery"
	restclient "k8s.io/client-go/rest"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	appsclient "github.com/openshift/origin/pkg/apps/generated/internalclientset/typed/apps/internalversion"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset/typed/build/internalversion"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset/typed/image/internalversion"
	"github.com/openshift/origin/pkg/oc/admin/prune/imageprune"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	oserrors "github.com/openshift/origin/pkg/util/errors"
	"github.com/openshift/origin/pkg/util/netutils"
	"github.com/openshift/origin/pkg/version"
)

// PruneImagesRecommendedName is the recommended command name
const PruneImagesRecommendedName = "images"

var errNoToken = errors.New("you must use a client config with a token")

const registryURLNotReachable = `(?:operation|connection) timed out|no such host`

var (
	imagesLongDesc = templates.LongDesc(`
		Remove image stream tags, images, and image layers by age or usage

		This command removes historical image stream tags, unused images, and unreferenced image
		layers from the integrated registry. By default, all images are considered as candidates.
		The command can be instructed to consider only images that have been directly pushed to the
		registry by supplying --all=false flag.

		By default, the prune operation performs a dry run making no changes to internal registry. A
		--confirm flag is needed for changes to be effective. The flag requires a valid route to the
		integrated Docker registry. If this command is run outside of the cluster network, the route
		needs to be provided using --registry-url.

		Only a user with a cluster role %s or higher who is logged-in will be able to actually
		delete the images.

		If the registry is secured with a certificate signed by a self-signed root certificate
		authority other than the one present in current user's config, you may need to specify it
		using --certificate-authority flag.

		Insecure connection is allowed in the following cases unless certificate-authority is
		specified:

		 1. --force-insecure is given  
		 2. provided registry-url is prefixed with http://  
		 3. registry url is a private or link-local address  
		 4. user's config allows for insecure connection (the user logged in to the cluster with
			--insecure-skip-tls-verify or allowed for insecure connection)`)

	imagesExample = templates.Examples(`
	  # See, what the prune command would delete if only images and their referrers were more than an hour old
	  # and obsoleted by 3 newer revisions under the same tag were considered.
	  %[1]s %[2]s --keep-tag-revisions=3 --keep-younger-than=60m

	  # To actually perform the prune operation, the confirm flag must be appended
	  %[1]s %[2]s --keep-tag-revisions=3 --keep-younger-than=60m --confirm

	  # See, what the prune command would delete if we're interested in removing images
	  # exceeding currently set limit ranges ('openshift.io/Image')
	  %[1]s %[2]s --prune-over-size-limit

	  # To actually perform the prune operation, the confirm flag must be appended
	  %[1]s %[2]s --prune-over-size-limit --confirm

	  # Force the insecure http protocol with the particular registry host name
	  %[1]s %[2]s --registry-url=http://registry.example.org --confirm

	  # Force a secure connection with a custom certificate authority to the particular registry host name
	  %[1]s %[2]s --registry-url=registry.example.org --certificate-authority=/path/to/custom/ca.crt --confirm`)
)

var (
	defaultKeepYoungerThan         = 60 * time.Minute
	defaultKeepTagRevisions        = 3
	defaultPruneImageOverSizeLimit = false
	defaultPruneRegistry           = true
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
	PruneRegistry       *bool
	IgnoreInvalidRefs   bool

	ClientConfig    *restclient.Config
	AppsClient      appsclient.AppsInterface
	BuildClient     buildclient.BuildInterface
	ImageClient     imageclient.ImageInterface
	DiscoveryClient discovery.DiscoveryInterface
	KubeClient      kclientset.Interface
	Timeout         time.Duration
	Out             io.Writer
	ErrOut          io.Writer
}

// NewCmdPruneImages implements the OpenShift cli prune images command.
func NewCmdPruneImages(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	allImages := true
	opts := &PruneImagesOptions{
		Confirm:            false,
		KeepYoungerThan:    &defaultKeepYoungerThan,
		KeepTagRevisions:   &defaultKeepTagRevisions,
		PruneOverSizeLimit: &defaultPruneImageOverSizeLimit,
		PruneRegistry:      &defaultPruneRegistry,
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

	cmd.Flags().BoolVar(&opts.Confirm, "confirm", opts.Confirm, "If true, specify that image pruning should proceed. Defaults to false, displaying what would be deleted but not actually deleting anything. Requires a valid route to the integrated Docker registry (see --registry-url).")
	cmd.Flags().BoolVar(opts.AllImages, "all", *opts.AllImages, "Include images that were imported from external registries as candidates for pruning.  If pruned, all the mirrored objects associated with them will also be removed from the integrated registry.")
	cmd.Flags().DurationVar(opts.KeepYoungerThan, "keep-younger-than", *opts.KeepYoungerThan, "Specify the minimum age of an image and its referrers for it to be considered a candidate for pruning.")
	cmd.Flags().IntVar(opts.KeepTagRevisions, "keep-tag-revisions", *opts.KeepTagRevisions, "Specify the number of image revisions for a tag in an image stream that will be preserved.")
	cmd.Flags().BoolVar(opts.PruneOverSizeLimit, "prune-over-size-limit", *opts.PruneOverSizeLimit, "Specify if images which are exceeding LimitRanges (see 'openshift.io/Image'), specified in the same namespace, should be considered for pruning. This flag cannot be combined with --keep-younger-than nor --keep-tag-revisions.")
	cmd.Flags().StringVar(&opts.CABundle, "certificate-authority", opts.CABundle, "The path to a certificate authority bundle to use when communicating with the managed Docker registries. Defaults to the certificate authority data from the current user's config file. It cannot be used together with --force-insecure.")
	cmd.Flags().StringVar(&opts.RegistryUrlOverride, "registry-url", opts.RegistryUrlOverride, "The address to use when contacting the registry, instead of using the default value. This is useful if you can't resolve or reach the registry (e.g.; the default is a cluster-internal URL) but you do have an alternative route that works. Particular transport protocol can be enforced using '<scheme>://' prefix.")
	cmd.Flags().BoolVar(&opts.ForceInsecure, "force-insecure", opts.ForceInsecure, "If true, allow an insecure connection to the docker registry that is hosted via HTTP or has an invalid HTTPS certificate. Whenever possible, use --certificate-authority instead of this dangerous option.")
	cmd.Flags().BoolVar(opts.PruneRegistry, "prune-registry", *opts.PruneRegistry, "If false, the prune operation will clean up image API objects, but the none of the associated content in the registry is removed.  Note, if only image API objects are cleaned up through use of this flag, the only means for subsequently cleaning up registry data corresponding to those image API objects is to employ the 'hard prune' administrative task.")
	cmd.Flags().BoolVar(&opts.IgnoreInvalidRefs, "ignore-invalid-refs", opts.IgnoreInvalidRefs, "If true, the pruning process will ignore all errors while parsing image references. This means that the pruning process will ignore the intended connection between the object and the referenced image. As a result an image may be incorrectly deleted as unused.")

	return cmd
}

// Complete turns a partially defined PruneImagesOptions into a solvent structure
// which can be validated and used for pruning images.
func (o *PruneImagesOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, out io.Writer) error {
	if len(args) > 0 {
		return kcmdutil.UsageErrorf(cmd, "no arguments are allowed to this command")
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
	o.ErrOut = os.Stderr

	clientConfig, err := f.ClientConfig()
	if err != nil {
		return err
	}
	o.ClientConfig = clientConfig
	appsClient, buildClient, imageClient, kubeClient, err := getClients(f)
	if err != nil {
		return err
	}
	o.AppsClient = appsClient
	o.BuildClient = buildClient
	o.ImageClient = imageClient
	o.KubeClient = kubeClient
	o.DiscoveryClient = kubeClient.Discovery()

	o.Timeout = clientConfig.Timeout
	if o.Timeout == 0 {
		o.Timeout = time.Duration(10 * time.Second)
	}

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
	if err := imageapi.ValidateRegistryURL(o.RegistryUrlOverride); len(o.RegistryUrlOverride) > 0 && err != nil {
		return fmt.Errorf("invalid --registry-url flag: %v", err)
	}
	if o.ForceInsecure && len(o.CABundle) > 0 {
		return fmt.Errorf("--certificate-authority cannot be specified with --force-insecure")
	}
	if len(o.CABundle) > 0 && strings.HasPrefix(o.RegistryUrlOverride, "http://") {
		return fmt.Errorf("--cerificate-authority cannot be specified for insecure http protocol")
	}
	return nil
}

// Run contains all the necessary functionality for the OpenShift cli prune images command.
func (o PruneImagesOptions) Run() error {
	allImages, err := o.ImageClient.Images().List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	allStreams, err := o.ImageClient.ImageStreams(o.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	allPods, err := o.KubeClient.Core().Pods(o.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	allRCs, err := o.KubeClient.Core().ReplicationControllers(o.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	allBCs, err := o.BuildClient.BuildConfigs(o.Namespace).List(metav1.ListOptions{})
	// We need to tolerate 'not found' errors for buildConfigs since they may be disabled in Atomic
	err = oserrors.TolerateNotFoundError(err)
	if err != nil {
		return err
	}

	allBuilds, err := o.BuildClient.Builds(o.Namespace).List(metav1.ListOptions{})
	// We need to tolerate 'not found' errors for builds since they may be disabled in Atomic
	err = oserrors.TolerateNotFoundError(err)
	if err != nil {
		return err
	}

	allDSs, err := o.KubeClient.Extensions().DaemonSets(o.Namespace).List(metav1.ListOptions{})
	if err != nil {
		// TODO: remove in future (3.9) release
		if !kerrors.IsForbidden(err) {
			return err
		}
		fmt.Fprintf(o.ErrOut, "Failed to list daemonsets: %v\n - * Make sure to update clusterRoleBindings.\n", err)
	}

	allDeployments, err := o.KubeClient.Extensions().Deployments(o.Namespace).List(metav1.ListOptions{})
	if err != nil {
		// TODO: remove in future (3.9) release
		if !kerrors.IsForbidden(err) {
			return err
		}
		fmt.Fprintf(o.ErrOut, "Failed to list deployments: %v\n - * Make sure to update clusterRoleBindings.\n", err)
	}

	allDCs, err := o.AppsClient.DeploymentConfigs(o.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	allRSs, err := o.KubeClient.Extensions().ReplicaSets(o.Namespace).List(metav1.ListOptions{})
	if err != nil {
		// TODO: remove in future (3.9) release
		if !kerrors.IsForbidden(err) {
			return err
		}
		fmt.Fprintf(o.ErrOut, "Failed to list replicasets: %v\n - * Make sure to update clusterRoleBindings.\n", err)
	}

	limitRangesList, err := o.KubeClient.Core().LimitRanges(o.Namespace).List(metav1.ListOptions{})
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

	var (
		registryHost   = o.RegistryUrlOverride
		registryClient *http.Client
		registryPinger imageprune.RegistryPinger
	)

	if o.Confirm {
		if len(registryHost) == 0 {
			registryHost, err = imageprune.DetermineRegistryHost(allImages, allStreams)
			if err != nil {
				return fmt.Errorf("unable to determine registry: %v", err)
			}
		}

		insecure := o.ForceInsecure
		if !insecure && len(o.CABundle) == 0 {
			insecure = o.ClientConfig.TLSClientConfig.Insecure || netutils.IsPrivateAddress(registryHost) ||
				strings.HasPrefix(registryHost, "http://")
		}

		registryClient, err = getRegistryClient(o.ClientConfig, o.CABundle, insecure)
		if err != nil {
			return err
		}
		registryPinger = &imageprune.DefaultRegistryPinger{
			Client:   registryClient,
			Insecure: insecure,
		}
	} else {
		registryPinger = &imageprune.DryRunRegistryPinger{}
	}

	registryURL, err := registryPinger.Ping(registryHost)
	if err != nil {
		if len(o.RegistryUrlOverride) == 0 && regexp.MustCompile(registryURLNotReachable).MatchString(err.Error()) {
			err = fmt.Errorf("%s\n* Please provide a reachable route to the integrated registry using --registry-url.", err.Error())
		}
		return fmt.Errorf("failed to ping registry %s: %v", registryHost, err)
	}

	options := imageprune.PrunerOptions{
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
		DSs:                allDSs,
		Deployments:        allDeployments,
		DCs:                allDCs,
		RSs:                allRSs,
		LimitRanges:        limitRangesMap,
		DryRun:             o.Confirm == false,
		RegistryClient:     registryClient,
		RegistryURL:        registryURL,
		PruneRegistry:      o.PruneRegistry,
		IgnoreInvalidRefs:  o.IgnoreInvalidRefs,
	}
	if o.Namespace != metav1.NamespaceAll {
		options.Namespace = o.Namespace
	}
	pruner, errs := imageprune.NewPruner(options)
	if errs != nil {
		o.printGraphBuildErrors(errs)
		return fmt.Errorf("failed to build graph - no changes made")
	}

	w := tabwriter.NewWriter(o.Out, 10, 4, 3, ' ', 0)
	defer w.Flush()

	imageDeleter := &describingImageDeleter{w: w, errOut: o.ErrOut}
	imageStreamDeleter := &describingImageStreamDeleter{w: w, errOut: o.ErrOut}
	layerLinkDeleter := &describingLayerLinkDeleter{w: w, errOut: o.ErrOut}
	blobDeleter := &describingBlobDeleter{w: w, errOut: o.ErrOut}
	manifestDeleter := &describingManifestDeleter{w: w, errOut: o.ErrOut}

	if o.Confirm {
		imageDeleter.delegate = imageprune.NewImageDeleter(o.ImageClient)
		imageStreamDeleter.delegate = imageprune.NewImageStreamDeleter(o.ImageClient)
		layerLinkDeleter.delegate = imageprune.NewLayerLinkDeleter()
		blobDeleter.delegate = imageprune.NewBlobDeleter()
		manifestDeleter.delegate = imageprune.NewManifestDeleter()
	} else {
		fmt.Fprintln(o.ErrOut, "Dry run enabled - no modifications will be made. Add --confirm to remove images")
	}

	if o.PruneRegistry != nil && !*o.PruneRegistry {
		fmt.Fprintln(o.Out, "Only API objects will be removed.  No modifications to the image registry will be made.")
	}

	return pruner.Prune(imageDeleter, imageStreamDeleter, layerLinkDeleter, blobDeleter, manifestDeleter)
}

func (o *PruneImagesOptions) printGraphBuildErrors(errs kutilerrors.Aggregate) {
	refErrors := []error{}

	fmt.Fprintf(o.ErrOut, "Failed to build graph!\n")

	for _, err := range errs.Errors() {
		if _, ok := err.(*imageprune.ErrBadReference); ok {
			refErrors = append(refErrors, err)
		} else {
			fmt.Fprintf(o.ErrOut, "%v\n", err)
		}
	}

	if len(refErrors) > 0 {
		clientVersion, masterVersion, err := getClientAndMasterVersions(o.DiscoveryClient, o.Timeout)
		if err != nil {
			fmt.Fprintf(o.ErrOut, "Failed to get master API version: %v\n", err)
		}
		fmt.Fprintf(o.ErrOut, "\nThe following objects have invalid references:\n\n")
		for _, err := range refErrors {
			fmt.Fprintf(o.ErrOut, "  %s\n", err)
		}
		fmt.Fprintf(o.ErrOut, "\nEither fix the references or delete the objects to make the pruner proceed.\n")

		if masterVersion != nil && (clientVersion.Major != masterVersion.Major || clientVersion.Minor != masterVersion.Minor) {
			fmt.Fprintf(o.ErrOut, "Client version (%s) doesn't match master (%s), which may allow for different image references. Try to re-run this binary with the same version.\n", clientVersion, masterVersion)
		}
	}
}

// describingImageStreamDeleter prints information about each image stream update.
// If a delegate exists, its DeleteImageStream function is invoked prior to returning.
type describingImageStreamDeleter struct {
	w             io.Writer
	delegate      imageprune.ImageStreamDeleter
	headerPrinted bool
	errOut        io.Writer
}

var _ imageprune.ImageStreamDeleter = &describingImageStreamDeleter{}

func (p *describingImageStreamDeleter) GetImageStream(stream *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	return stream, nil
}

func (p *describingImageStreamDeleter) UpdateImageStream(stream *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	if p.delegate == nil {
		return stream, nil
	}

	updatedStream, err := p.delegate.UpdateImageStream(stream)
	if err != nil {
		fmt.Fprintf(p.errOut, "error updating image stream %s/%s to remove image references: %v\n", stream.Namespace, stream.Name, err)
	}

	return updatedStream, err
}

func (p *describingImageStreamDeleter) NotifyImageStreamPrune(stream *imageapi.ImageStream, updatedTags []string, deletedTags []string) {
	if !p.headerPrinted {
		p.headerPrinted = true
		fmt.Fprintln(p.w, "Deleting references from image streams to images ...")
		fmt.Fprintln(p.w, "STREAM\tACTION\tTAGS")
	}

	if len(updatedTags) > 0 {
		fmt.Fprintf(p.w, "%s/%s\tUpdated\t%s\n", stream.Namespace, stream.Name, strings.Join(updatedTags, ", "))
	}
	if len(deletedTags) > 0 {
		fmt.Fprintf(p.w, "%s/%s\tDeleted\t%s\n", stream.Namespace, stream.Name, strings.Join(deletedTags, ", "))
	}
}

// describingImageDeleter prints information about each image being deleted.
// If a delegate exists, its DeleteImage function is invoked prior to returning.
type describingImageDeleter struct {
	w             io.Writer
	delegate      imageprune.ImageDeleter
	headerPrinted bool
	errOut        io.Writer
}

var _ imageprune.ImageDeleter = &describingImageDeleter{}

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
		fmt.Fprintf(p.errOut, "error deleting image %s from server: %v\n", image.Name, err)
	}

	return err
}

// describingLayerLinkDeleter prints information about each repo layer link being deleted. If a delegate
// exists, its DeleteLayerLink function is invoked prior to returning.
type describingLayerLinkDeleter struct {
	w             io.Writer
	delegate      imageprune.LayerLinkDeleter
	headerPrinted bool
	errOut        io.Writer
}

var _ imageprune.LayerLinkDeleter = &describingLayerLinkDeleter{}

func (p *describingLayerLinkDeleter) DeleteLayerLink(registryClient *http.Client, registryURL *url.URL, repo, name string) error {
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
		fmt.Fprintf(p.errOut, "error deleting repository %s layer link %s from the registry: %v\n", repo, name, err)
	}

	return err
}

// describingBlobDeleter prints information about each blob being deleted. If a
// delegate exists, its DeleteBlob function is invoked prior to returning.
type describingBlobDeleter struct {
	w             io.Writer
	delegate      imageprune.BlobDeleter
	headerPrinted bool
	errOut        io.Writer
}

var _ imageprune.BlobDeleter = &describingBlobDeleter{}

func (p *describingBlobDeleter) DeleteBlob(registryClient *http.Client, registryURL *url.URL, layer string) error {
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
		fmt.Fprintf(p.errOut, "error deleting blob %s from the registry: %v\n", layer, err)
	}

	return err
}

// describingManifestDeleter prints information about each repo manifest being
// deleted. If a delegate exists, its DeleteManifest function is invoked prior
// to returning.
type describingManifestDeleter struct {
	w             io.Writer
	delegate      imageprune.ManifestDeleter
	headerPrinted bool
	errOut        io.Writer
}

var _ imageprune.ManifestDeleter = &describingManifestDeleter{}

func (p *describingManifestDeleter) DeleteManifest(registryClient *http.Client, registryURL *url.URL, repo, manifest string) error {
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
		fmt.Fprintf(p.errOut, "error deleting manifest %s from repository %s: %v\n", manifest, repo, err)
	}

	return err
}

// getClients returns a OpenShift client and Kube client.
func getClients(f *clientcmd.Factory) (appsclient.AppsInterface, buildclient.BuildInterface, imageclient.ImageInterface, kclientset.Interface, error) {
	clientConfig, err := f.ClientConfig()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	if len(clientConfig.BearerToken) == 0 {
		return nil, nil, nil, nil, errNoToken
	}

	kubeClient, err := f.ClientSet()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	appsClient, err := appsclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	buildClient, err := buildclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	imageClient, err := imageclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return appsClient, buildClient, imageClient, kubeClient, err
}

// getRegistryClient returns a registry client. Note that registryCABundle and registryInsecure=true are
// mutually exclusive. If registryInsecure=true is specified, the ca bundle is ignored.
func getRegistryClient(clientConfig *restclient.Config, registryCABundle string, registryInsecure bool) (*http.Client, error) {
	var (
		err                      error
		cadata                   []byte
		registryCABundleIncluded = false
		token                    = clientConfig.BearerToken
	)

	if len(token) == 0 {
		return nil, errNoToken
	}

	if len(registryCABundle) > 0 {
		cadata, err = ioutil.ReadFile(registryCABundle)
		if err != nil {
			return nil, fmt.Errorf("failed to read registry ca bundle: %v", err)
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
		return nil, err
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
		return nil, err
	}

	return &http.Client{
		Transport: wrappedTransport,
	}, nil
}

// getClientAndMasterVersions returns version info for client and master binaries. If it takes too long to get
// a response from the master, timeout error is returned.
func getClientAndMasterVersions(client discovery.DiscoveryInterface, timeout time.Duration) (clientVersion, masterVersion *version.Info, err error) {
	done := make(chan error)

	go func() {
		defer close(done)

		ocVersionBody, err := client.RESTClient().Get().AbsPath("/version/openshift").Do().Raw()
		switch {
		case err == nil:
			var ocServerInfo version.Info
			err = json.Unmarshal(ocVersionBody, &ocServerInfo)
			if err != nil && len(ocVersionBody) > 0 {
				done <- err
				return
			}
			masterVersion = &ocServerInfo

		case kerrors.IsNotFound(err) || kerrors.IsUnauthorized(err) || kerrors.IsForbidden(err):
		default:
			done <- err
			return
		}
	}()

	select {
	case err, closed := <-done:
		if strings.HasSuffix(fmt.Sprintf("%v", err), "connection refused") || kclientcmd.IsEmptyConfig(err) || kclientcmd.IsConfigurationInvalid(err) {
			return nil, nil, err
		}
		if closed && err != nil {
			return nil, nil, err
		}
	// do not block error printing if the master is busy
	case <-time.After(timeout):
		return nil, nil, fmt.Errorf("error: server took too long to respond with version information.")
	}

	v := version.Get()
	clientVersion = &v

	return
}

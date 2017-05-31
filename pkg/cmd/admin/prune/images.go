package prune

import (
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/kubernetes/pkg/client/retry"
	//kv1core "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	kapi "k8s.io/kubernetes/pkg/api"
	kextapi "k8s.io/kubernetes/pkg/apis/extensions"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion/"
	kextclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/extensions/internalversion"
	//kdeployutil "k8s.io/kubernetes/pkg/controller/deployment/util"
	"k8s.io/kubernetes/pkg/kubectl"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	kerrors "k8s.io/kubernetes/staging/src/k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/kubernetes/staging/src/k8s.io/apimachinery/pkg/fields"
	"k8s.io/kubernetes/staging/src/k8s.io/apimachinery/pkg/labels"
	//kruntime "k8s.io/kubernetes/staging/src/k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploycmd "github.com/openshift/origin/pkg/deploy/cmd"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/prune"
	oserrors "github.com/openshift/origin/pkg/util/errors"
)

const (
	// PruneImagesRecommendedName is the recommended command name
	PruneImagesRecommendedName = "images"

	DefaultRegistryNamespace = "default"

	RegistryROModeEnablementEnvVar = "REGISTRY_STORAGE_MAINTENANCE_READONLY"
	RegistryROModeEnablementValue  = `{"enabled":true}`

	registryRedeploymentWaitTimeout = time.Minute * 8
)

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
		delete the images.`)

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
	  %[1]s %[2]s --prune-over-size-limit --confirm
	  
	  # To remove orphaned blobs from registry storage, add the additional flag:
	  %[1]s %[2]s --confirm --prune-orphans-in-ro-mode`)
)

var (
	defaultKeepYoungerThan         = 60 * time.Minute
	defaultKeepTagRevisions        = 3
	defaultPruneImageOverSizeLimit = false
	defaultPruneOrhpansInROMode    = false
)

// PruneImagesOptions holds all the required options for pruning images.
type PruneImagesOptions struct {
	Confirm              bool
	KeepYoungerThan      *time.Duration
	KeepTagRevisions     *int
	PruneOverSizeLimit   *bool
	PruneOrphansInROMode *bool
	AllImages            *bool
	CABundle             string
	RegistryUrlOverride  string
	Namespace            string

	OSClient                 client.Interface
	KClient                  kclientset.Interface
	Decoder                  runtime.Decoder
	RegistryClient           *http.Client
	RegistryPods             *kapi.PodList
	RegistryDeploymentModels []runtime.Object
	Out                      io.Writer
}

// NewCmdPruneImages implements the OpenShift cli prune images command.
func NewCmdPruneImages(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	allImages := true
	opts := &PruneImagesOptions{
		Confirm:              false,
		KeepYoungerThan:      &defaultKeepYoungerThan,
		KeepTagRevisions:     &defaultKeepTagRevisions,
		PruneOverSizeLimit:   &defaultPruneImageOverSizeLimit,
		PruneOrphansInROMode: &defaultPruneOrhpansInROMode,
		AllImages:            &allImages,
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
	cmd.Flags().BoolVar(opts.PruneOrphansInROMode, "prune-orphans-in-ro-mode", *opts.PruneOrphansInROMode, "Restart all registry instances into read-only mode with pushes disabled, do the regular prune followed by removing orphaned blobs (those not referenced in the etcd) from registry storage and restart the registry back to read-write mode. If given without --confirm=true, neither the registry will be restarted, nor the blobs will be deleted.")
	cmd.Flags().StringVar(&opts.CABundle, "certificate-authority", opts.CABundle, "The path to a certificate authority bundle to use when communicating with the managed Docker registries. Defaults to the certificate authority data from the current user's config file.")
	cmd.Flags().StringVar(&opts.RegistryUrlOverride, "registry-url", opts.RegistryUrlOverride, "The address to use when contacting the registry, instead of using the default value. This is useful if you can't resolve or reach the registry (e.g.; the default is a cluster-internal URL) but you do have an alternative route that works.")

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

	osClient, kClient, registryClient, err := getClients(f, o.CABundle)
	if err != nil {
		return err
	}
	o.OSClient = osClient
	o.KClient = kClient
	o.RegistryClient = registryClient
	o.Decoder = f.Decoder(true)

	if *o.PruneOrphansInROMode {
		podList, err := collectRegistryPods(kClient.Core())
		if err != nil {
			return err
		}

		o.RegistryPods = podList

		registryDeploymentModels, err := collectRegistryDeploymentModels(o.OSClient, kClient.Extensions(), kClient.Extensions())
		if err != nil {
			return err
		}

		// TODO: increase back to one
		//if len(registryDeploymentModels) > 1 {
		if len(registryDeploymentModels) > 0 {
			fmt.Fprintln(os.Stderr, "Warning: Found multiple docker-registry deployment models:")
			for _, obj := range registryDeploymentModels {
				fmt.Fprintln(os.Stderr, "Warning:   %s", describeDeploymentModel(obj))
			}
			fmt.Fprintln(os.Stderr, "Warning: Pods instantiated from any of those models will be affected!")
		}

		for _, dm := range registryDeploymentModels {
			failedDeploymentModels := make(map[runtime.Object]error)
			done, err := isDeploymentSuccessfullyDone(o.OSClient, kClient, dm)
			if err != nil {
				failedDeploymentModels[dm] = err
			} else if !done {
				failedDeploymentModels[dm] = fmt.Errorf("unknown reason")
			}
			if len(failedDeploymentModels) > 0 {
				fmt.Fprintln(os.Stderr, "There are incomplete deployments of integrated docker registry:")
				for dm, err := range failedDeploymentModels {
					fmt.Fprintln(os.Stderr, "  %s: %v", dm, err)
				}
				return fmt.Errorf("Please fix the issues before requesting re-deployments to read-only mode!")
			}
		}

		o.RegistryDeploymentModels = registryDeploymentModels
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
	if _, err := url.Parse(o.RegistryUrlOverride); err != nil {
		return fmt.Errorf("invalid --registry-url flag: %v", err)
	}
	return nil
}

// Run contains all the necessary functionality for the OpenShift cli prune images command.
func (o PruneImagesOptions) Run() error {
	err := o.prepareRegistry()
	if err != nil {
		return err
	}

	err = o.doSoftPrune()
	if err != nil {
		o.restoreRegistry(true)
		return err
	}

	if *o.PruneOrphansInROMode {
		err = o.doHardPrune()
		if err != nil {
			o.restoreRegistry(true)
			return err
		}
	}

	return o.restoreRegistry(false)
}

func setContainerEnvVar(c *kapi.Container, key, value string) bool {
	for i, v := range c.Env {
		if v.Name == key {
			if v.Value == value {
				return false
			}
			c.Env[i].Value = value
			return true
		}
	}
	c.Env = append(c.Env, kapi.EnvVar{Name: key, Value: value})
	return true
}

func (o PruneImagesOptions) prepareRegistry() error {
	if !*o.PruneOrphansInROMode {
		return nil
	}

	updatedDeploymentModels := []runtime.Object{}

	var err error
	modifiedPods := 0
	for dmi := range o.RegistryDeploymentModels {
		dm := o.RegistryDeploymentModels[dmi]

		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			var podTemplateSpec *kapi.PodTemplateSpec

			switch t := dm.(type) {
			case *deployapi.DeploymentConfig:
				podTemplateSpec = t.Spec.Template
			case *kextapi.DaemonSet:
				podTemplateSpec = &t.Spec.Template
			case *kextapi.Deployment:
				podTemplateSpec = &t.Spec.Template
			default:
				fmt.Fprintln(os.Stderr, "Skipping unknown deployment model %s", describeDeploymentModel)
				return nil
			}

			modified := false
			for i := range podTemplateSpec.Spec.InitContainers {
				if setContainerEnvVar(&podTemplateSpec.Spec.InitContainers[i], RegistryROModeEnablementEnvVar, RegistryROModeEnablementValue) {
					modified = true
				}
			}
			for i := range podTemplateSpec.Spec.Containers {
				if setContainerEnvVar(&podTemplateSpec.Spec.Containers[i], RegistryROModeEnablementEnvVar, RegistryROModeEnablementValue) {
					modified = true
				}
			}

			if modified {
				modifiedPods += len(getMatchingPods(o.Decoder, dm, o.RegistryPods).Items)
			}

			if !modified || !o.Confirm {
				if o.Confirm {
					glog.V(4).Infof("Deployment model %s already configured for read-only mode", describeDeploymentModel(dm))
				}
				return nil
			}

			glog.V(4).Infof("Configuting deployment model %s for read-only mode", describeDeploymentModel(dm))
			var getErr, updateErr error
			switch t := dm.(type) {
			case *deployapi.DeploymentConfig:
				_, updateErr := o.OSClient.DeploymentConfigs(DefaultRegistryNamespace).Update(t)
				if updateErr != nil && kerrors.IsConflict(updateErr) {
					dm, getErr = o.OSClient.DeploymentConfigs(DefaultRegistryNamespace).Get(t.Name, metav1.GetOptions{})
				}
			case *kextapi.DaemonSet:
				_, updateErr := o.KClient.Extensions().DaemonSets(DefaultRegistryNamespace).Update(t)
				if updateErr == nil || !kerrors.IsConflict(updateErr) {
					dm, getErr = o.KClient.Extensions().DaemonSets(DefaultRegistryNamespace).Get(t.Name, metav1.GetOptions{})
				}
			case *kextapi.Deployment:
				_, updateErr := o.KClient.Extensions().Deployments(DefaultRegistryNamespace).Update(t)
				if updateErr == nil || !kerrors.IsConflict(updateErr) {
					dm, getErr = o.KClient.Extensions().Deployments(DefaultRegistryNamespace).Get(t.Name, metav1.GetOptions{})
				}
			}
			if updateErr != nil {
				if !kerrors.IsConflict(updateErr) {
					glog.Errorf("Failed to update deployment model %s: %v", describeDeploymentModel(dm), updateErr)
					return updateErr
				}
			} else {
				return nil
			}
			// conflict occurred
			if getErr != nil {
				return getErr
			}

			o.RegistryDeploymentModels[dmi] = dm
			updatedDeploymentModels = append(updatedDeploymentModels, dm)

			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to update deployment model %s: %v", describeDeploymentModel, err)
		}
	}

	if len(updatedDeploymentModels) == 0 && err == nil {
		fmt.Fprintln(os.Stderr, "No changes to deployment models are necessary.")
		return nil
	}

	if err != nil {
		if len(updatedDeploymentModels) > 0 && o.Confirm {
			glog.Errorf("failed to reconfigure deployment models: %v", err)
			fmt.Fprintln(os.Stderr, "Reverting changes")
			rollbackUpdatedDeploymentModels(o.OSClient, o.KClient, updatedDeploymentModels)
		}
		return err
	}

	if !o.Confirm {
		if modifiedPods > 0 {
			fmt.Fprintln(os.Stderr, "Would restart %d registry pods to read-only mode", modifiedPods)
		}
		return nil
	}

	err = waitForDeploymentModelsRollout(o.OSClient, o.KClient, updatedDeploymentModels, registryRedeploymentWaitTimeout)
	if err != nil {
		glog.Errorf("failed to wait on registry deployment models to complete re-deployments: %v", err)
		fmt.Fprintln(os.Stderr, "Reverting changes")
		rollbackUpdatedDeploymentModels(o.OSClient, o.KClient, updatedDeploymentModels)
	}

	return err
}

func (o PruneImagesOptions) restoreRegistry(afterFailure bool) error {
	if !*o.PruneOrphansInROMode {
		// no change is required
		return nil
	}
	// TODO
	return nil
}

func (o PruneImagesOptions) doSoftPrune() error {
	allImages, updateErr := o.OSClient.Images().List(metav1.ListOptions{})
	if updateErr != nil {
		return updateErr
	}

	allStreams, updateErr := o.OSClient.ImageStreams(o.Namespace).List(metav1.ListOptions{})
	if updateErr != nil {
		return updateErr
	}

	allPods, updateErr := o.KClient.Core().Pods(o.Namespace).List(metav1.ListOptions{})
	if updateErr != nil {
		return updateErr
	}

	allRCs, updateErr := o.KClient.Core().ReplicationControllers(o.Namespace).List(metav1.ListOptions{})
	if updateErr != nil {
		return updateErr
	}

	allBCs, updateErr := o.OSClient.BuildConfigs(o.Namespace).List(metav1.ListOptions{})
	// We need to tolerate 'not found' errors for buildConfigs since they may be disabled in Atomic
	updateErr = oserrors.TolerateNotFoundError(updateErr)
	if updateErr != nil {
		return updateErr
	}

	allBuilds, updateErr := o.OSClient.Builds(o.Namespace).List(metav1.ListOptions{})
	// We need to tolerate 'not found' errors for builds since they may be disabled in Atomic
	updateErr = oserrors.TolerateNotFoundError(updateErr)
	if updateErr != nil {
		return updateErr
	}

	allDCs, updateErr := o.OSClient.DeploymentConfigs(o.Namespace).List(metav1.ListOptions{})
	if updateErr != nil {
		return updateErr
	}

	limitRangesList, updateErr := o.KClient.Core().LimitRanges(o.Namespace).List(metav1.ListOptions{})
	if updateErr != nil {
		return updateErr
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

func (o PruneImagesOptions) doHardPrune() error {
	// TODO: exec into the registry and run the hard prune
	return nil
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

	updatedStream, updateErr := p.delegate.DeleteImageStream(stream, image, updatedTags)
	if updateErr != nil {
		fmt.Fprintf(os.Stderr, "error updating image stream %s/%s to remove references to image %s: %v\n", stream.Namespace, stream.Name, image.Name, updateErr)
	}

	return updatedStream, updateErr
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

	updateErr := p.delegate.DeleteImage(image)
	if updateErr != nil {
		fmt.Fprintf(os.Stderr, "error deleting image %s from server: %v\n", image.Name, updateErr)
	}

	return updateErr
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

	updateErr := p.delegate.DeleteLayerLink(registryClient, registryURL, repo, name)
	if updateErr != nil {
		fmt.Fprintf(os.Stderr, "error deleting repository %s layer link %s from the registry: %v\n", repo, name, updateErr)
	}

	return updateErr
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

	updateErr := p.delegate.DeleteBlob(registryClient, registryURL, layer)
	if updateErr != nil {
		fmt.Fprintf(os.Stderr, "error deleting blob %s from the registry: %v\n", layer, updateErr)
	}

	return updateErr
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

	updateErr := p.delegate.DeleteManifest(registryClient, registryURL, repo, manifest)
	if updateErr != nil {
		fmt.Fprintf(os.Stderr, "error deleting data for repository %s image manifest %s from the registry: %v\n", repo, manifest, updateErr)
	}

	return updateErr
}

// getClients returns a Kube client, OpenShift client, and registry client.
func getClients(f *clientcmd.Factory, caBundle string) (*client.Client, kclientset.Interface, *http.Client, error) {
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

	tlsConfig, err := restclient.TLSConfigFor(&registryClientConfig)
	if err != nil {
		return nil, nil, nil, err
	}

	// if the user specified a CA on the command line, add it to the
	// client config's CA roots
	if tlsConfig != nil && len(caBundle) > 0 {
		data, err := ioutil.ReadFile(caBundle)
		if err != nil {
			return nil, nil, nil, err
		}

		if tlsConfig.RootCAs == nil {
			tlsConfig.RootCAs = x509.NewCertPool()
		}

		tlsConfig.RootCAs.AppendCertsFromPEM(data)
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

func collectRegistryPods(podsGetter kinternal.PodsGetter) (*kapi.PodList, error) {
	podInterfacer := podsGetter.Pods(DefaultRegistryNamespace)
	// OR label selector would be prefered in this case, unfortunately it's not yet possible
	podList, err := podInterfacer.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result kapi.PodList = *podList
	result.Items = []kapi.Pod{}
	for _, pod := range podList.Items {
		for _, label := range []struct{ key, value string }{
			// DeploymentConfigs
			{deployapi.DeploymentConfigLabel, "docker-registry"},
			// DaemonSets
			{"docker-registry", DefaultRegistryNamespace},
			// TODO: handle upstream deployments
		} {
			if pod.Labels[label.key] != label.value {
				continue
			}
			result.Items = append(result.Items, pod)
			break
		}
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("no registry pods found")
	}

	return podList, nil
}

func collectRegistryDeploymentModels(
	dcNamespacer client.DeploymentConfigsNamespacer,
	deploymentGetter kextclientset.DeploymentsGetter,
	dsGetter kextclientset.DaemonSetsGetter,
) ([]runtime.Object, error) {
	var result []runtime.Object

	dcList, err := dcNamespacer.DeploymentConfigs(DefaultRegistryNamespace).List(metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{kapi.ObjectNameField: "docker-registry"}).String(),
	})
	if err != nil {
		return nil, err
	}
	for i := range dcList.Items {
		dc := &dcList.Items[i]
		result = append(result, dc)
	}

	deploymentList, err := deploymentGetter.Deployments(DefaultRegistryNamespace).List(metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{kapi.ObjectNameField: "docker-registry"}).String(),
	})
	if err != nil {
		return nil, err
	}
	for i := range deploymentList.Items {
		deployment := &deploymentList.Items[i]
		result = append(result, deployment)
	}

	dsList, err := dsGetter.DaemonSets(DefaultRegistryNamespace).List(metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{kapi.ObjectNameField: "docker-registry"}).String(),
	})
	if err != nil {
		return nil, err
	}
	for i := range dsList.Items {
		ds := &dsList.Items[i]
		result = append(result, ds)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no deploymentconfig, daemonset nor deployment named docker-registry found in the default namespace")
	}

	return result, nil
}

func getMatchingPods(decoder runtime.Decoder, deploymentModel runtime.Object, podList *kapi.PodList) kapi.PodList {
	result := *podList
	result.Items = []kapi.Pod{}

	for i := range podList.Items {
		pod := &podList.Items[i]
		sref := kapi.SerializedReference{}
		annotation := pod.Annotations[kapi.CreatedByAnnotation]
		err := runtime.DecodeInto(decoder, []byte(annotation), &sref)
		if err != nil {
			fmt.Fprintln(os.Stderr, "failed to decode %s annotation %q of pod %s", kapi.CreatedByAnnotation, annotation, pod.Name)
			continue
		}

		dmMeta := deploymentModel.(metav1.ObjectMetaAccessor).GetObjectMeta()
		if sref.Reference.Kind == deploymentModel.GetObjectKind().GroupVersionKind().Kind &&
			sref.Reference.Namespace == dmMeta.GetNamespace() &&
			sref.Reference.Name == dmMeta.GetName() &&
			sref.Reference.UID == dmMeta.GetUID() {
			result.Items = append(result.Items, *pod)
		}
	}

	return result
}

func isDeploymentSuccessfullyDone(
	osClient client.Interface,
	kClient kclientset.Interface,
	dm runtime.Object,
) (bool, error) {
	dmMeta := dm.(metav1.ObjectMetaAccessor).GetObjectMeta()
	//m.GetNamespace(), m.GetName())
	var statusViewer kubectl.StatusViewer
	var err error

	statusViewer, err = getStatusViewerForDeploymentModel(osClient, kClient, dm)
	if err != nil {
		return false, err
	}

	description, done, err := statusViewer.Status(dmMeta.GetNamespace(), dmMeta.GetName(), 0)
	if err != nil {
		return false, err
	}
	glog.V(4).Infof("Deployment model %s done=%t, reason=%v", describeDeploymentModel(dm), done, description)
	return done, nil
}

func describeDeploymentModel(dm runtime.Object) string {
	dmMeta := dm.(metav1.ObjectMetaAccessor).GetObjectMeta()
	return fmt.Sprintf("%s[%s/%s]",
		dm.GetObjectKind().GroupVersionKind().Kind,
		dmMeta.GetNamespace(), dmMeta.GetName())
}

func waitForDeploymentModelsRollout(
	osClient client.Interface,
	kClient kclientset.Interface,
	deploymentModels []runtime.Object,
	timeout time.Duration,
) error {
	// Watcher at index i corresponds to a deploymentModels[i].
	// When the deployment model is done or the watcher terminates, the watcher
	// gets set to nil.
	watchers := make([]watch.Interface, 0, len(deploymentModels))

	// this is a sink for all the watchers
	eventChan := make(chan watch.Event)
	watcherTerminatedChan := make(chan int)
	defer func() {
		for _, w := range watchers {
			if w != nil {
				w.Stop()
			}
		}
		close(eventChan)
		close(watcherTerminatedChan)
	}()

	for watcherIndex, dm := range deploymentModels {
		var watcher watch.Interface
		var err error

		watchOptions := metav1.ListOptions{
			FieldSelector: fields.SelectorFromSet(fields.Set{
				kapi.ObjectNameField: dm.(metav1.ObjectMetaAccessor).
					GetObjectMeta().GetName(),
			}).String()}
		switch dm.(type) {
		case *deployapi.DeploymentConfig:
			watcher, err = osClient.DeploymentConfigs(DefaultRegistryNamespace).Watch(watchOptions)
		case *kextapi.DaemonSet:
			watcher, err = kClient.Extensions().DaemonSets(DefaultRegistryNamespace).Watch(watchOptions)
		case *kextapi.Deployment:
			watcher, err = kClient.Extensions().Deployments(DefaultRegistryNamespace).Watch(watchOptions)
		}

		if err != nil {
			return err
		}
		go func() {
			for {
				select {
				case e, more := <-watcher.ResultChan():
					eventChan <- e
					if !more {
						watcherTerminatedChan <- watcherIndex
						return
					}
				}
			}
		}()
		watchers = append(watchers, watcher)
	}

	started := time.Now()

	for {
		allWatchersTerminated := true
		for _, w := range watchers {
			if w != nil {
				allWatchersTerminated = false
				break
			}
		}
		if allWatchersTerminated {
			// we are done
			break
		}

		select {
		case e := <-eventChan:
			switch e.Type {
			case watch.Deleted:
				// paranoid mode on
				return fmt.Errorf("deployment model %s has been deleted from outside while pruning", describeDeploymentModel(e.Object))
			case watch.Added:
				return fmt.Errorf("deployment model %s has been added from outside while pruning", describeDeploymentModel(e.Object))
				// paranoid mode off
			case watch.Error:
				// TODO: pretty print the kapi.Status object
				glog.Errorf("deployment watch error: %#+v", e.Object)
				// ignore
				continue
			}

			dm := e.Object
			if dm == nil {
				continue
			}
			index := -1
			for i, candidate := range deploymentModels {
				// TODO: is there some simpler way to generically match
				// identity of two objects of possibly different kinds?
				if dm.GetObjectKind().GroupVersionKind().Kind ==
					candidate.GetObjectKind().GroupVersionKind().Kind &&
					dm.(metav1.ObjectMetaAccessor).GetObjectMeta().GetUID() ==
						candidate.(metav1.ObjectMetaAccessor).GetObjectMeta().GetUID() {
					index = i
					break
				}
			}
			if index == -1 {
				// paranoid mode on
				// TODO: this may be serious (e.g. somebody added a new
				// docker-registry deployment since we collected them at the
				// beginning); shall we terminate instead of just logging?
				glog.Errorf("received notification about unexpected deployment model %s", describeDeploymentModel(dm))
				//paranoid mode off
				continue
			}

			done, err := isDeploymentSuccessfullyDone(osClient, kClient, dm)
			if err != nil {
				return err
			}
			if !done {
				continue
			}

			glog.V(1).Infof("TODO(remove me) terminating watch with index i=%d (%s)", index, describeDeploymentModel(deploymentModels[index]))
			// all the watchers should be terminated here
			watchers[index].Stop()
			watchers[index] = nil

		case i := <-watcherTerminatedChan:
			glog.V(1).Infof("TODO(remove me) watcher with index=%d (%s) terminated", i, describeDeploymentModel(deploymentModels[i]))
			if w := watchers[i]; w != nil {
				// paranoid mode on
				// if we get here, it means the watcher terminated on its own,
				// which is scary
				watchers[i].Stop()
				watchers[i] = nil
				// TODO: shall we try to set the watcher up again?
				return fmt.Errorf("watcher for deployment model %s unexpectedly terminated", describeDeploymentModel(deploymentModels[i]))
				// paranoid mode off
			}
		case <-time.After(started.Add(timeout).Sub(time.Now())):
			return fmt.Errorf("timeout occurred while waiting on deployments to finish")
		}
	}

	return nil
}

func rollbackUpdatedDeploymentModels(
	osClient client.Interface,
	kClient kclientset.Interface,
	deploymentModels []runtime.Object,
) error {
	// TODO: rollback to previous state of all deployment models we have
	// modified
	return nil
}

func getStatusViewerForDeploymentModel(
	osClient client.Interface,
	kClient kclientset.Interface,
	dm runtime.Object,
) (kubectl.StatusViewer, error) {
	switch dm.(type) {
	case *deployapi.DeploymentConfig:
		return deploycmd.NewDeploymentConfigStatusViewer(osClient), nil
	default:
		return kubectl.StatusViewerFor(dm.GetObjectKind().GroupVersionKind().GroupKind(), kClient)
	}
}

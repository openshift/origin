package images

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"golang.org/x/exp/slices"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension"
	"github.com/openshift/library-go/pkg/image/reference"
	"github.com/openshift/origin/pkg/clioptions/imagesetup"
	"github.com/openshift/origin/pkg/cmd"
	"github.com/openshift/origin/pkg/test/extensions"
	"github.com/openshift/origin/test/extended/util/image"
	"github.com/spf13/cobra"
	"k8s.io/kube-openapi/pkg/util/sets"
	"k8s.io/kubectl/pkg/util/templates"
	k8simage "k8s.io/kubernetes/test/utils/image"
)

func NewImagesCommand() *cobra.Command {
	o := &imagesOptions{}
	cmd := &cobra.Command{
		Use:   "images",
		Short: "Gather images required for testing",
		Long: templates.LongDesc(fmt.Sprintf(`
		Creates a mapping to mirror test images to a private registry

		This command identifies the locations of all test images referenced by the test
		suite and outputs a mirror list for use with 'oc image mirror' to copy those images
		to a private registry. The list may be passed via file or standard input.

				$ openshift-tests images --to-repository private.com/test/repository > /tmp/mirror
				$ oc image mirror -f /tmp/mirror

		The 'run' and 'run-upgrade' subcommands accept '--from-repository' which will source
		required test images from your mirror.

		See the help for 'oc image mirror' for more about mirroring to disk or consult the docs
		for mirroring offline. You may use a file:// prefix in your '--to-repository', but when
		mirroring from disk to your offline repository you will have to construct the appropriate
		disk to internal registry statements yourself.

		By default, the test images are sourced from a public container image repository at
		%[1]s and are provided as-is for testing purposes only. Images are mirrored by the project
		to the public repository periodically.
		`, imagesetup.DefaultTestImageMirrorLocation)),
		PersistentPreRun: cmd.NoPrintVersion,
		SilenceUsage:     true,
		SilenceErrors:    true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := imagesetup.VerifyTestImageRepoEnvVarUnset(); err != nil {
				return err
			}

			if o.Verify {
				return imagesetup.VerifyImages()
			}

			repository := o.Repository
			var prefix string
			for _, validPrefix := range []string{"file://", "s3://"} {
				if strings.HasPrefix(repository, validPrefix) {
					repository = strings.TrimPrefix(repository, validPrefix)
					prefix = validPrefix
					break
				}
			}
			ref, err := reference.Parse(repository)
			if err != nil {
				return fmt.Errorf("--to-repository is not valid: %v", err)
			}
			if len(ref.Tag) > 0 || len(ref.ID) > 0 {
				return fmt.Errorf("--to-repository may not include a tag or image digest")
			}

			if err := imagesetup.VerifyImages(); err != nil {
				return err
			}
			lines, err := createImageMirrorForInternalImages(prefix, ref, !o.Upstream)
			if err != nil {
				return err
			}
			for _, line := range lines {
				fmt.Fprintln(os.Stdout, line)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&o.Upstream, "upstream", o.Upstream, "Retrieve images from the default upstream location")
	cmd.Flags().StringVar(&o.Repository, "to-repository", o.Repository, "A container image repository to mirror to.")
	// this is a private flag for debugging only
	cmd.Flags().BoolVar(&o.Verify, "verify", o.Verify, "Verify the contents of the image mappings")
	cmd.Flags().MarkHidden("verify")
	return cmd
}

type imagesOptions struct {
	Repository string
	Upstream   bool
	Verify     bool
}

// createImageMirrorForInternalImages returns a list of 'oc image mirror' mappings from source to
// target or returns an error. If mirrored is true the images are assumed to have already been copied
// from their upstream location into our official mirror, in the REPO:TAG format where TAG is a hash
// of the original internal name and the index of the image in the array. Otherwise the mappings will
// be set to mirror the location as defined in the test code into our official mirror, where the target
// TAG is the hash described above.
func createImageMirrorForInternalImages(prefix string, ref reference.DockerImageReference, mirrored bool) ([]string, error) {
	source := ref.Exact()
	externalImageSets := []extension.Image{}
	initialImageSets := []extensions.ImageSet{
		k8simage.GetOriginalImageConfigs(),
	}

	// If ENV is not set, the list of images should come from external binaries
	if len(os.Getenv("OPENSHIFT_SKIP_EXTERNAL_TESTS")) == 0 {
		extractionContext, extractionContextCancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer extractionContextCancel()
		cleanUpFn, externalBinaries, err := extensions.ExtractAllTestBinaries(extractionContext, 10)
		if err != nil {
			return nil, err
		}
		defer cleanUpFn()

		// List test images from all available binaries
		listContext, listContextCancel := context.WithTimeout(context.Background(), time.Minute)
		defer listContextCancel()
		imageSetsFromBinaries, err := externalBinaries.ListImages(listContext, 10)
		if err != nil {
			return nil, err
		}
		if len(imageSetsFromBinaries) == 0 {
			return nil, fmt.Errorf("no test images were reported by external binaries")
		}
		externalImageSets = imageSetsFromBinaries
	}

	// Convert external images to initial and updated image sets
	// Add mapped images to updated image set if they exist
	exceptions := image.Exceptions.List()
	updatedImageSets := []extensions.ImageSet{}
	initial := extensions.ImageSet{}
	updated := extensions.ImageSet{}
	for _, image := range externalImageSets {
		imageConfig := covertMappedImageToImageConfig(image)
		if !slices.ContainsFunc(exceptions, func(e string) bool {
			return strings.Contains(imageConfig.GetE2EImage(), e)
		}) {
			initial[k8simage.ImageID(image.Index)] = imageConfig
			if image.Mapped != nil {
				updated[k8simage.ImageID(image.Index)] = covertMappedImageToImageConfig(*image.Mapped)
			}
		}
	}
	if len(initial) > 0 {
		initialImageSets = []extensions.ImageSet{initial}
	}

	// Take the initial images coming from external binaries and remove any exceptions that might exist.
	defaultImageSets := []extensions.ImageSet{}
	for i := range initialImageSets {
		filtered := extensions.ImageSet{}
		for imageID, imageConfig := range initialImageSets[i] {
			if !slices.ContainsFunc(exceptions, func(e string) bool {
				return strings.Contains(imageConfig.GetE2EImage(), e)
			}) {
				filtered[imageID] = imageConfig
			}
		}
		if len(filtered) > 0 {
			defaultImageSets = append(defaultImageSets, filtered)
		}
	}

	// Map initial images to the target repository
	for _, img := range defaultImageSets {
		for imageID, imageConfig := range img {
			// If the imageID is in the updated image set, skip it
			if _, ok := updated[imageID]; ok {
				continue
			}
			m := map[string]k8simage.ImageID{
				imageConfig.GetE2EImage(): k8simage.ImageID(imageID),
			}
			mappedImage := map[string]string{}
			switch imageID {
			// These images are special and can't be run out of the cloud - some because they
			// are authenticated, and others because they are not real images. Tests that depend
			// on these images can't be run without access to the public internet.
			case k8simage.InvalidRegistryImage, k8simage.AgnhostPrivate, k8simage.AuthenticatedAlpine:
				mappedImage[imageConfig.GetE2EImage()] = imageConfig.GetE2EImage()
			default:
				mappedImage = image.GetMappedImages(m, source)
			}
			ref, err := reference.Parse(mappedImage[imageConfig.GetE2EImage()])
			if err != nil {
				continue
			}
			config := k8simage.Config{}
			config.SetRegistry(ref.Registry)
			config.SetName(ref.RepositoryName())
			config.SetVersion(ref.Tag)
			updated[k8simage.ImageID(imageID)] = config
		}
	}

	updatedImageSets = []extensions.ImageSet{updated}

	openshiftDefaults := image.OriginalImages()
	openshiftUpdated := image.GetMappedImages(openshiftDefaults, imagesetup.DefaultTestImageMirrorLocation)

	// if we've mirrored, then the source is going to be our repo, not upstream's
	if mirrored {
		baseRef, err := reference.Parse(imagesetup.DefaultTestImageMirrorLocation)
		if err != nil {
			return nil, fmt.Errorf("invalid default mirror location: %v", err)
		}

		// calculate the mapping of upstream images by setting defaults to baseRef
		covered := sets.NewString()
		for i := range updatedImageSets {
			for imageID, imageConfig := range updatedImageSets[i] {
				originalConfig := defaultImageSets[i][imageID]
				pullSpec := imageConfig.GetE2EImage()
				if pullSpec == originalConfig.GetE2EImage() {
					continue
				}
				if covered.Has(pullSpec) {
					continue
				}
				covered.Insert(pullSpec)
				e2eRef, err := reference.Parse(pullSpec)
				if err != nil {
					return nil, fmt.Errorf("invalid test image: %s: %v", pullSpec, err)
				}
				if len(e2eRef.Tag) == 0 {
					return nil, fmt.Errorf("invalid test image: %s: no tag", pullSpec)
				}
				imageConfig.SetRegistry(baseRef.Registry)
				imageConfig.SetName(baseRef.RepositoryName())
				imageConfig.SetVersion(e2eRef.Tag)
				defaultImageSets[i][imageID] = imageConfig
			}
		}

		// calculate the mapping for openshift images by populating openshiftUpdated
		openshiftUpdated = make(map[string]string)
		sourceMappings := image.GetMappedImages(openshiftDefaults, imagesetup.DefaultTestImageMirrorLocation)
		targetMappings := image.GetMappedImages(openshiftDefaults, source)

		for from, to := range targetMappings {
			if from == to {
				continue
			}
			if covered.Has(to) {
				continue
			}
			covered.Insert(to)
			from := sourceMappings[from]
			openshiftUpdated[from] = to
		}
	}

	covered := sets.NewString()
	var lines []string
	for i := range updatedImageSets {
		for imageID := range updatedImageSets[i] {
			a, b := defaultImageSets[i][imageID], updatedImageSets[i][imageID]
			from, to := a.GetE2EImage(), b.GetE2EImage()
			if from == to {
				continue
			}
			if covered.Has(from) {
				continue
			}
			covered.Insert(from)
			lines = append(lines, fmt.Sprintf("%s %s%s", from, prefix, to))
		}
	}

	for from, to := range openshiftUpdated {
		if from == to {
			continue
		}
		if covered.Has(from) {
			continue
		}
		covered.Insert(from)
		lines = append(lines, fmt.Sprintf("%s %s%s", from, prefix, to))
	}

	sort.Strings(lines)
	return lines, nil
}

func covertMappedImageToImageConfig(image extension.Image) k8simage.Config {
	imageConfig := k8simage.Config{}
	imageConfig.SetName(image.Name)
	imageConfig.SetVersion(image.Version)
	imageConfig.SetRegistry(image.Registry)

	return imageConfig
}

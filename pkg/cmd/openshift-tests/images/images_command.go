package images

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
	k8simage "k8s.io/kubernetes/test/utils/image"

	"github.com/openshift/library-go/pkg/image/reference"
	"github.com/openshift/origin/pkg/clioptions/imagesetup"
	"github.com/openshift/origin/pkg/cmd"
	"github.com/openshift/origin/pkg/test/extensions"
	"github.com/openshift/origin/test/extended/util/image"
	"github.com/spf13/cobra"
	"k8s.io/kube-openapi/pkg/util/sets"
	"k8s.io/kubectl/pkg/util/templates"
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

			if err := setLogLevel(o.LogLevel); err != nil {
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

			// Verify manifest lists for all images before printing
			if o.VerifyManifestLists {
				sourceImages := extractSourceImages(lines)
				if err := imagesetup.VerifyManifestLists(sourceImages, o.AllowMissingArchs); err != nil {
					return err
				}
			}
			for _, line := range lines {
				fmt.Fprintln(os.Stdout, line)
			}

			return nil
		},
	}
	cmd.Flags().BoolVar(&o.Upstream, "upstream", o.Upstream, "Retrieve images from the default upstream location")
	cmd.Flags().StringVar(&o.Repository, "to-repository", o.Repository, "A container image repository to mirror to.")
	cmd.Flags().BoolVar(&o.VerifyManifestLists, "verify-manifest-lists", o.VerifyManifestLists, "Verify that all images have multi-architecture manifest lists with required architectures")
	cmd.Flags().StringSliceVar(&o.AllowMissingArchs, "allow-missing-architectures", o.AllowMissingArchs, "Images that are allowed to have missing architectures (can be specified multiple times or comma-separated). Substring matching is supported.")
	cmd.Flags().StringVar(&o.LogLevel, "log-level", "info", "Log level for verification output (debug, info, warn, error)")
	// this is a private flag for debugging only
	cmd.Flags().BoolVar(&o.Verify, "verify", o.Verify, "Verify the contents of the image mappings")
	cmd.Flags().MarkHidden("verify")
	return cmd
}

type imagesOptions struct {
	Repository          string
	Upstream            bool
	Verify              bool
	VerifyManifestLists bool
	AllowMissingArchs   []string
	LogLevel            string
}

// createImageMirrorForInternalImages returns a list of 'oc image mirror' mappings from source to
// target or returns an error. If mirrored is true the images are assumed to have already been copied
// from their upstream location into our official mirror, in the REPO:TAG format where TAG is a hash
// of the original internal name and the index of the image in the array. Otherwise the mappings will
// be set to mirror the location as defined in the test code into our official mirror, where the target
// TAG is the hash described above.
func createImageMirrorForInternalImages(prefix string, ref reference.DockerImageReference, mirrored bool) ([]string, error) {
	source := ref.Exact()

	// initialImageSets contains all the original images discovered either from
	// internal image configs or from external test binaries.
	initialImageSets := []extensions.ImageSet{}

	// defaultImageSets contains the subset of initialImageSets after filtering
	// out images whose E2E image matches any entry in the exceptions list.
	defaultImageSets := []extensions.ImageSet{}

	// updatedImageSets contains the mapped versions of images from defaultImageSets
	// (e.g., images rewritten to point to the mirror location).
	updatedImageSets := []extensions.ImageSet{}

	// exceptions is a list of images we don't mirror temporarily due to various problems.
	exceptions := image.Exceptions.List()

	// If the ENV is set, then list the images used come from the vendored tests
	if len(os.Getenv("OPENSHIFT_SKIP_EXTERNAL_TESTS")) != 0 {
		initialImageSets = []extensions.ImageSet{
			k8simage.GetOriginalImageConfigs(),
		}

		defaultImageSets = filterExceptionsFromImageSets(initialImageSets)

		for i := range defaultImageSets {
			updatedImageSets = append(updatedImageSets, k8simage.GetMappedImageConfigs(defaultImageSets[i], ref.Exact()))
		}
	} else {
		// If ENV is not set, the list of images should come from external binaries
		// Extract all test binaries
		extractionContext, extractionContextCancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer extractionContextCancel()
		cleanUpFn, externalBinaries, _, err := extensions.ExtractAllTestBinaries(extractionContext, 10)
		if err != nil {
			return nil, err
		}
		defer cleanUpFn()

		// List test images from all available binaries
		listContext, listContextCancel := context.WithTimeout(context.Background(), time.Minute)
		defer listContextCancel()
		imagesFromBinaries, err := externalBinaries.ListImages(listContext, 10)
		if err != nil {
			return nil, err
		}
		if len(imagesFromBinaries) == 0 {
			return nil, fmt.Errorf("no test images were reported by external binaries")
		}

		for _, image := range imagesFromBinaries {
			// Add original image
			imageID := k8simage.ImageID(image.Index)
			origImageConfig := convertImageToImageConfig(image)

			initialSet := extensions.ImageSet{imageID: origImageConfig}
			initialImageSets = append(initialImageSets, initialSet)

			// Only add to default and mapped if original image passes exceptions
			if !imageMatchesExceptions(origImageConfig, exceptions) {
				defaultImageSets = append(defaultImageSets, initialSet)

				if image.Mapped != nil {
					mappedSet := extensions.ImageSet{
						imageID: convertImageToImageConfig(*image.Mapped),
					}
					updatedImageSets = append(updatedImageSets, mappedSet)
				}
			}
		}
	}

	// OpenShift defaults
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
				defaultConfig := defaultImageSets[i][imageID]
				pullSpec := imageConfig.GetE2EImage()
				if pullSpec == defaultConfig.GetE2EImage() {
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

// extractSourceImages extracts source images from mirror list lines.
// Each line is expected to be in the format: "source-image target-image"
func extractSourceImages(lines []string) []string {
	var sourceImages []string
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 1 {
			sourceImages = append(sourceImages, parts[0])
		}
	}
	return sourceImages
}

// setLogLevel configures the logrus log level based on the provided string
func setLogLevel(level string) error {
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}
	logrus.SetLevel(lvl)
	return nil
}

// filterExceptionsFromImageSets takes a list of image sets and filters out images
// whose E2E image matches any exception.Exceptions is a list of images we
// don't mirror temporarily due to various problems.
func filterExceptionsFromImageSets(initialImageSets []extensions.ImageSet) []extensions.ImageSet {
	exceptions := image.Exceptions.List()
	filteredImageSets := []extensions.ImageSet{}

	for _, imgSet := range initialImageSets {
		filtered := extensions.ImageSet{}
		for imageID, imageConfig := range imgSet {
			if !imageMatchesExceptions(imageConfig, exceptions) {
				filtered[imageID] = imageConfig
			}
		}
		if len(filtered) > 0 {
			filteredImageSets = append(filteredImageSets, filtered)
		}
	}

	return filteredImageSets
}

// imageMatchesExceptions checks if a single image matches any exception
func imageMatchesExceptions(imageConfig k8simage.Config, exceptions []string) bool {
	return slices.ContainsFunc(exceptions, func(e string) bool {
		return strings.Contains(imageConfig.GetE2EImage(), e)
	})
}

// convertImageToImageConfig converts an extension.Image to k8simage.Config
func convertImageToImageConfig(image extension.Image) k8simage.Config {
	imageConfig := k8simage.Config{}
	imageConfig.SetName(image.Name)
	imageConfig.SetVersion(image.Version)
	imageConfig.SetRegistry(image.Registry)

	return imageConfig
}

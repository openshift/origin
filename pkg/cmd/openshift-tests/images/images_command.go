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

// imageMapping pairs an image with its mirror destination.
type imageMapping struct {
	original k8simage.Config
	updated  k8simage.Config
}

// createImageMirrorForInternalImages returns a list of 'oc image mirror' mappings from source to
// target or returns an error. If mirrored is true the images are assumed to have already been copied
// from their upstream location into our official mirror, in the REPO:TAG format where TAG is a hash
// of the original internal name and the index of the image in the array. Otherwise the mappings will
// be set to mirror the location as defined in the test code into our official mirror, where the target
// TAG is the hash described above.
func createImageMirrorForInternalImages(prefix string, ref reference.DockerImageReference, mirrored bool) ([]string, error) {
	mappings, err := collectImageMappings(ref)
	if err != nil {
		return nil, err
	}
	mappings = applyExceptions(mappings, image.Exceptions.List())

	openshiftUpdated := image.GetMappedImages(image.OriginalImages(), imagesetup.DefaultTestImageMirrorLocation)
	if mirrored {
		// if we've mirrored, then the source is going to be our repo, not upstream's
		mappings, openshiftUpdated, err = remapSourcesToCommunityMirror(mappings, ref.Exact())
		if err != nil {
			return nil, err
		}
	}

	return deduplicatedMirrorLines(prefix, mappings, openshiftUpdated), nil
}

// collectImageMappings gathers the original/mapped image pairs required by the
// test suite. Images come from external binaries, or from the vendored
// Kubernetes image list when OPENSHIFT_SKIP_EXTERNAL_TESTS is set.
func collectImageMappings(ref reference.DockerImageReference) ([]imageMapping, error) {
	if len(os.Getenv("OPENSHIFT_SKIP_EXTERNAL_TESTS")) > 0 {
		originals := k8simage.GetOriginalImageConfigs()
		updated := k8simage.GetMappedImageConfigs(originals, ref.Exact())
		var mappings []imageMapping
		for id, original := range originals {
			mappings = append(mappings, imageMapping{original: original, updated: updated[id]})
		}
		return mappings, nil
	}

	// Extract all test binaries
	extractionContext, extractionContextCancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer extractionContextCancel()
	cleanUpFn, externalBinaries, _, err := extensions.ExtractAllTestBinaries(extractionContext, 10, nil, false)
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

	var mappings []imageMapping
	for _, img := range imagesFromBinaries {
		mappings = append(mappings, imageSetsForImage(img, ref))
	}
	return mappings, nil
}

// applyExceptions drops mappings whose original image matches an exception.
func applyExceptions(mappings []imageMapping, exceptions []string) []imageMapping {
	filtered := make([]imageMapping, 0, len(mappings))
	for _, m := range mappings {
		if !slices.ContainsFunc(exceptions, func(e string) bool {
			return strings.Contains(m.original.GetE2EImage(), e)
		}) {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// remapSourcesToCommunityMirror rewrites the source of already-mirrored images
// from their upstream location to the community mirror (keeping the mapped
// tag), and computes the mappings for the static Origin images.
func remapSourcesToCommunityMirror(mappings []imageMapping, source string) ([]imageMapping, map[string]string, error) {
	baseRef, err := reference.Parse(imagesetup.DefaultTestImageMirrorLocation)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid default mirror location: %v", err)
	}

	// Rewrite each mapping's source to the community mirror; every
	// destination tag is only processed once.
	covered := sets.NewString()
	for i, m := range mappings {
		pullSpec := m.updated.GetE2EImage()
		if pullSpec == m.original.GetE2EImage() {
			continue
		}
		if covered.Has(pullSpec) {
			continue
		}
		covered.Insert(pullSpec)
		e2eRef, err := reference.Parse(pullSpec)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid test image: %s: %v", pullSpec, err)
		}
		if len(e2eRef.Tag) == 0 {
			return nil, nil, fmt.Errorf("invalid test image: %s: no tag", pullSpec)
		}
		m.original.SetRegistry(baseRef.Registry)
		m.original.SetName(baseRef.RepositoryName())
		m.original.SetVersion(e2eRef.Tag)
		mappings[i] = m
	}

	// calculate the mapping for openshift images
	openshiftDefaults := image.OriginalImages()
	openshiftUpdated := make(map[string]string)
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
		openshiftUpdated[sourceMappings[from]] = to
	}

	return mappings, openshiftUpdated, nil
}

// deduplicatedMirrorLines renders sorted 'oc image mirror' mapping lines from the
// collected mappings. One source may legitimately map to several destinations,
// so only destinations are deduplicated: each destination tag may only be
// specified once in a mirror list.
func deduplicatedMirrorLines(prefix string, mappings []imageMapping, openshiftUpdated map[string]string) []string {
	coveredDestinations := sets.NewString()
	var lines []string
	for _, m := range mappings {
		from, to := m.original.GetE2EImage(), m.updated.GetE2EImage()
		if from == to {
			continue
		}
		dest := fmt.Sprintf("%s%s", prefix, to)
		if coveredDestinations.Has(dest) {
			continue
		}
		coveredDestinations.Insert(dest)
		lines = append(lines, fmt.Sprintf("%s %s", from, dest))
	}

	for from, to := range openshiftUpdated {
		if from == to {
			continue
		}
		dest := fmt.Sprintf("%s%s", prefix, to)
		if coveredDestinations.Has(dest) {
			continue
		}
		coveredDestinations.Insert(dest)
		lines = append(lines, fmt.Sprintf("%s %s", from, dest))
	}

	sort.Strings(lines)
	return lines
}

// imageSetsForImage converts an image reported by an external binary into an
// original/mapped pair. When the binary reports a Mapped tag, that tag is
// preferred over the locally computed mapping and is retargeted to the
// requested repository.
func imageSetsForImage(img extension.Image, ref reference.DockerImageReference) imageMapping {
	original := k8simage.Config{}
	original.SetName(img.Name)
	original.SetVersion(img.Version)
	original.SetRegistry(img.Registry)

	if img.Mapped == nil {
		id := k8simage.ImageID(img.Index)
		updated := k8simage.GetMappedImageConfigs(map[k8simage.ImageID]k8simage.Config{id: original}, ref.Exact())
		return imageMapping{original: original, updated: updated[id]}
	}

	// Use the binary's tag, but retarget it to the requested repository.
	updated := k8simage.Config{}
	updated.SetRegistry(ref.Registry)
	updated.SetName(ref.RepositoryName())
	updated.SetVersion(img.Mapped.Version)
	return imageMapping{original: original, updated: updated}
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

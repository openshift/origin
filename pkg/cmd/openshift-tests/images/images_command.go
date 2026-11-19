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
			lines, err := buildImageMirrorLines(prefix, ref, o.Upstream)
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

// imageMapping is one source and destination in an 'oc image mirror' list.
type imageMapping struct {
	source      string
	destination string
}

// buildImageMirrorLines sources images from the community mirror by default,
// or from the locations declared by the tests when fromUpstream is true.
func buildImageMirrorLines(destinationPrefix string, destinationRepo reference.DockerImageReference, fromUpstream bool) ([]string, error) {
	mappings, err := collectImageMappings(destinationRepo)
	if err != nil {
		return nil, err
	}
	mappings = applyExceptions(mappings, image.Exceptions.List())

	if fromUpstream {
		mappings = append(mappings, originMappingsFromUpstream()...)
	} else {
		// TODO: Reject destinations claimed by different original sources before
		// rewriting sources here; first-wins selection depends on binary completion order.
		claimedDestinations, err := rewriteSourcesToCommunityMirror(mappings)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, originMappingsFromCommunityMirror(destinationRepo.Exact(), claimedDestinations)...)
	}

	return deduplicatedMirrorLines(destinationPrefix, mappings), nil
}

// collectImageMappings gathers the source/destination pairs required by the
// test suite. Images come from external binaries, or from the vendored
// Kubernetes image list when OPENSHIFT_SKIP_EXTERNAL_TESTS is set.
func collectImageMappings(ref reference.DockerImageReference) ([]imageMapping, error) {
	if len(os.Getenv("OPENSHIFT_SKIP_EXTERNAL_TESTS")) > 0 {
		originals := k8simage.GetOriginalImageConfigs()
		updated := k8simage.GetMappedImageConfigs(originals, ref.Exact())
		var mappings []imageMapping
		for id, original := range originals {
			mapped := updated[id]
			mappings = append(mappings, imageMapping{source: original.GetE2EImage(), destination: mapped.GetE2EImage()})
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
		mappings = append(mappings, mappingForExtensionImage(img, ref))
	}
	return mappings, nil
}

// applyExceptions drops mappings whose source matches an exception.
func applyExceptions(mappings []imageMapping, exceptions []string) []imageMapping {
	filtered := make([]imageMapping, 0, len(mappings))
	for _, m := range mappings {
		if !slices.ContainsFunc(exceptions, func(e string) bool {
			return strings.Contains(m.source, e)
		}) {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// rewriteSourcesToCommunityMirror changes each source in place to the community
// mirror, using its destination tag. Rewritten mappings reserve their
// destinations even if rendering later skips an identity mapping.
func rewriteSourcesToCommunityMirror(mappings []imageMapping) (sets.String, error) {
	baseRef, err := reference.Parse(imagesetup.DefaultTestImageMirrorLocation)
	if err != nil {
		return nil, fmt.Errorf("invalid default mirror location: %v", err)
	}

	claimedDestinations := sets.NewString()
	for i := range mappings {
		m := &mappings[i]
		if m.destination == m.source {
			continue
		}
		if claimedDestinations.Has(m.destination) {
			continue
		}
		claimedDestinations.Insert(m.destination)
		e2eRef, err := reference.Parse(m.destination)
		if err != nil {
			return nil, fmt.Errorf("invalid test image: %s: %v", m.destination, err)
		}
		if len(e2eRef.Tag) == 0 {
			return nil, fmt.Errorf("invalid test image: %s: no tag", m.destination)
		}
		m.source = baseRef.Exact() + ":" + e2eRef.Tag
	}
	return claimedDestinations, nil
}

// originMappingsFromUpstream maps Origin's static images from their upstream
// sources to the community mirror.
func originMappingsFromUpstream() []imageMapping {
	var mappings []imageMapping
	for source, destination := range image.GetMappedImages(image.OriginalImages(), imagesetup.DefaultTestImageMirrorLocation) {
		mappings = append(mappings, imageMapping{source: source, destination: destination})
	}
	return mappings
}

// originMappingsFromCommunityMirror maps Origin's static images from the
// community mirror to the requested repository.
func originMappingsFromCommunityMirror(repository string, claimedDestinations sets.String) []imageMapping {
	originals := image.OriginalImages()
	sources := image.GetMappedImages(originals, imagesetup.DefaultTestImageMirrorLocation)
	destinations := image.GetMappedImages(originals, repository)
	var mappings []imageMapping
	for original, destination := range destinations {
		if original == destination {
			continue
		}
		if claimedDestinations.Has(destination) {
			continue
		}
		claimedDestinations.Insert(destination)
		mappings = append(mappings, imageMapping{source: sources[original], destination: destination})
	}
	return mappings
}

// deduplicatedMirrorLines renders sorted 'oc image mirror' mapping lines from the
// collected mappings. One source may legitimately map to several destinations,
// so only destinations are deduplicated: each destination tag may only be
// specified once in a mirror list.
func deduplicatedMirrorLines(prefix string, mappings []imageMapping) []string {
	coveredDestinations := sets.NewString()
	var lines []string
	for _, m := range mappings {
		if m.source == m.destination {
			continue
		}
		dest := prefix + m.destination
		if coveredDestinations.Has(dest) {
			continue
		}
		coveredDestinations.Insert(dest)
		lines = append(lines, m.source+" "+dest)
	}

	sort.Strings(lines)
	return lines
}

// mappingForExtensionImage uses the binary's Mapped tag when available;
// otherwise it computes the destination tag locally.
func mappingForExtensionImage(img extension.Image, ref reference.DockerImageReference) imageMapping {
	original := k8simage.Config{}
	original.SetName(img.Name)
	original.SetVersion(img.Version)
	original.SetRegistry(img.Registry)
	m := imageMapping{source: original.GetE2EImage()}

	if img.Mapped != nil {
		m.destination = ref.Exact() + ":" + img.Mapped.Version
		return m
	}

	id := k8simage.ImageID(img.Index)
	updated := k8simage.GetMappedImageConfigs(map[k8simage.ImageID]k8simage.Config{id: original}, ref.Exact())
	mapped := updated[id]
	m.destination = mapped.GetE2EImage()
	return m
}

// extractSourceImages extracts unique source images from mirror list lines.
// Each line is expected to be in the format: "source-image target-image"
func extractSourceImages(lines []string) []string {
	sourceImages := sets.NewString()
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 1 {
			sourceImages.Insert(parts[0])
		}
	}
	return sourceImages.List()
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

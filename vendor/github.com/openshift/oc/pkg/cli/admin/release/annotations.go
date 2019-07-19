package release

const (
	// This annotation is set in image-references when created with --from-release.
	annotationReleaseFromRelease = "release.openshift.io/from-release"
	// This annotation is set in image-references when created with --from-image-stream.
	annotationReleaseFromImageStream = "release.openshift.io/from-image-stream"

	// This value is set on images as LABEL to 'true' to indicate they should be
	// scanned for a /manifests/ directory to contribute to the payload.
	annotationReleaseOperator = "io.openshift.release.operator"

	// This is an internal annotation to indicate the source image was not derived
	// from an image stream or existing release but was manually specified.
	annotationReleaseOverride = "io.openshift.release.override"
	// This LABEL is set on images to indicate the manifest digest that was used
	// as the base layer for the release image (usually the cluster-version-operator).
	annotationReleaseBaseImageDigest = "io.openshift.release.base-image-digest"
	// This LABEL is a comma-delimited list of key=version pairs that can be consumed
	// by other manifests within the payload to hardcode version strings. Version must
	// be a semantic version with no build label (+ is not allowed) and key must be
	// alphanumeric characters and dashes only. The value `0.0.1-snapshot-key` in a
	// manifest will be substituted with the version value for key.
	annotationBuildVersions = "io.openshift.build.versions"

	// This LABEL is the git ref that an image was built with. Copied unmodified to
	// the image-references file.
	annotationBuildSourceRef = "io.openshift.build.commit.ref"
	// This LABEL is the full git commit hash that an image was built with. Copied
	// unmodified to the image-references file.
	annotationBuildSourceCommit = "io.openshift.build.commit.id"
	// This LABEL is the git clone location that an image was built with. Copied
	// unmodified to the image-references file.
	annotationBuildSourceLocation = "io.openshift.build.source-location"

	urlGithubPrefix = "https://github.com/"
)

package constants

// Docker image label constants
const (
	// OpenshiftNamespace is the namespace for Docker image labels used by OpenShift Origin.
	OpenshiftNamespace = "io.openshift."

	// DefaultNamespace is the default namespace for Docker image labels used and produced by S2I.
	DefaultNamespace = OpenshiftNamespace + "s2i."

	// KubernetesNamespace is the namespace for Docker image labels used by Kubernetes.
	KubernetesNamespace = "io.k8s."

	// KubernetesDescriptionLabel is the Docker image LABEL that provides an image description to Kubernetes.
	KubernetesDescriptionLabel = KubernetesNamespace + "description"

	// KubernetesDisplayNameLabel is the Docker image LABEL that provides a human-readable name for the image to Kubernetes.
	KubernetesDisplayNameLabel = KubernetesNamespace + "display-name"

	// AssembleInputFilesLabel is the Docker image LABEL that tells S2I which files wil be copied from builder to a runtime image.
	AssembleInputFilesLabel = DefaultNamespace + "assemble-input-files"

	// AssembleRuntimeUserLabel is the Docker image label that tells S2I which user should execute the assemble-runtime scripts.
	AssembleRuntimeUserLabel = DefaultNamespace + "assemble-runtime-user"

	// AssembleUserLabel is the Docker image label that tells S2I which user should execute the assemble scripts.
	AssembleUserLabel = DefaultNamespace + "assemble-user"

	buildNamespace = DefaultNamespace + "build."

	// BuildCommitRefLabel is the Docker image LABEL that S2I uses to record the source commit used to produce the S2I image.
	//
	// During a rebuild, this label is used by S2I to check out the appropriate commit from the source repository.
	BuildCommitRefLabel = buildNamespace + "commit.ref"

	// BuildImageLabel is the Docker image LABEL that S2I uses to record the builder image used to produce the S2I image.
	//
	// During a rebuild, this label is used by S2I to pull the appropriate builder image.
	BuildImageLabel = buildNamespace + "image"

	// BuildSourceLocationLabel is the Docker image LABEL that S2I uses to record the URL of the source repository used to produce the S2I image.
	//
	// During a rebuild, this label is used by S2I to clone the appropriate source code repository.
	BuildSourceLocationLabel = buildNamespace + "source-location"

	// BuildSourceContextDir is the Docker image LABEL that S2I uses to record the context directory in the source code repository to use for the  build.
	//
	// During a rebuild, this label is used by S2I to set the context directory within the source code for the S2I build.
	BuildSourceContextDirLabel = buildNamespace + "source-context-dir"

	// TODO: Deprecate BuilderVersionLabel?

	// BuilderBaseVersionLabel is the Docker image LABEL that tells S2I the version of the base image used by the builder image.
	BuilderBaseVersionLabel = OpenshiftNamespace + "builder-base-version"

	// BuilderVersionLable is the  Docker image LABEL that tells S2I the version of the builder image.
	BuilderVersionLabel = OpenshiftNamespace + "builder-version"

	// DestinationLabel is the Docker image LABEL that tells S2I where to place the artifacts (scripts, sources) in the builder image.
	DestinationLabel = DefaultNamespace + "destination"

	// ScriptsURLLabel is the Docker image LABEL that tells S2I where to look for the S2I scripts.
	// This label is also copied into the output image.
	ScriptsURLLabel = DefaultNamespace + "scripts-url"
)

// Deprecated Docker image labels
const (
	// DeprecatedScriptsURLLabel is the Docker image LABEL that previously told S2I where to look for the S2I scripts.
	//
	// DEPRECATED - use ScriptsURLLabel instead.
	DeprecatedScriptsURLLabel = "io.s2i.scripts-url"

	// DeprecatedDestinationLabel is the Docker image LABEL that previously told S2I where to place the artifacts in the builder image.
	//
	// DEPRECATED - use DestinationLabel instead.
	DeprecatedDestinationLabel = "io.s2i.destination"
)

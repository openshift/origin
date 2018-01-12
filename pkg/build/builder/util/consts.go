package util

const (

	// OriginVersion is an environment variable key that indicates the version of origin that
	// created this build definition.
	OriginVersion = "ORIGIN_VERSION"

	// AllowedUIDs is an environment variable that contains ranges of UIDs that are allowed in
	// Source builder images
	AllowedUIDs = "ALLOWED_UIDS"
	// DropCapabilities is an environment variable that contains a list of capabilities to drop when
	// executing a Source build
	DropCapabilities = "DROP_CAPS"

	// DefaultDockerLabelNamespace is the key of a Build label, whose values are build metadata.
	DefaultDockerLabelNamespace = "io.openshift."

	StatusMessageCannotCreateBuildPodSpec        = "Failed to create pod spec."
	StatusMessageCannotCreateBuildPod            = "Failed creating build pod."
	StatusMessageInvalidOutputRef                = "Output image could not be resolved."
	StatusMessageInvalidImageRef                 = "Referenced image could not be resolved."
	StatusMessageCancelBuildFailed               = "Failed to cancel build."
	StatusMessageBuildPodDeleted                 = "The pod for this build was deleted before the build completed."
	StatusMessageExceededRetryTimeout            = "Build did not complete and retrying timed out."
	StatusMessageMissingPushSecret               = "Missing push secret."
	StatusMessagePostCommitHookFailed            = "Build failed because of post commit hook."
	StatusMessagePushImageToRegistryFailed       = "Failed to push the image to the registry."
	StatusMessagePullBuilderImageFailed          = "Failed pulling builder image."
	StatusMessageFetchSourceFailed               = "Failed to fetch the input source."
	StatusMessageInvalidContextDirectory         = "The supplied context directory does not exist."
	StatusMessageCancelledBuild                  = "The build was cancelled by the user."
	StatusMessageDockerBuildFailed               = "Docker build strategy has failed."
	StatusMessageBuildPodExists                  = "The pod for this build already exists and is older than the build."
	StatusMessageNoBuildContainerStatus          = "The pod for this build has no container statuses indicating success or failure."
	StatusMessageFailedContainer                 = "The pod for this build has at least one container with a non-zero exit status."
	StatusMessageGenericBuildFailed              = "Generic Build failure - check logs for details."
	StatusMessageUnresolvableEnvironmentVariable = "Unable to resolve build environment variable reference."
	StatusMessageCannotRetrieveServiceAccount    = "Unable to look up the service account associated with this build."
)

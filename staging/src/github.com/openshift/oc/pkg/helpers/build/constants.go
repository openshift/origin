package build

const (
	// BuildConfigAnnotation is an annotation that identifies the BuildConfig that a Build was created from
	BuildConfigAnnotation = "openshift.io/build-config.name"
	// BuildConfigLabel is the key of a Build label whose value is the ID of a BuildConfig
	// on which the Build is based. NOTE: The value for this label may not contain the entire
	// BuildConfig name because it will be truncated to maximum label length.
	BuildConfigLabel = "openshift.io/build-config.name"
	// BuildConfigLabelDeprecated was used as BuildConfigLabel before adding namespaces.
	// We keep it for backward compatibility.
	BuildConfigLabelDeprecated = "buildconfig"
	// BuildJenkinsBlueOceanLogURLAnnotation is an annotation holding a link to the Jenkins build console log via the Jenkins BlueOcean UI Plugin
	BuildJenkinsBlueOceanLogURLAnnotation = "openshift.io/jenkins-blueocean-log-url"
)

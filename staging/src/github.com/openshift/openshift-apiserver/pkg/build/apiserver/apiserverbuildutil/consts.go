package apiserverbuildutil

const (
	BuildTriggerCauseGithubMsg    = "GitHub WebHook"
	BuildTriggerCauseGenericMsg   = "Generic WebHook"
	BuildTriggerCauseGitLabMsg    = "GitLab WebHook"
	BuildTriggerCauseBitbucketMsg = "Bitbucket WebHook"
)

const (
	// GitCloneContainer is the name of the container that will clone the
	// build source repository and also handle binary input content.
	GitCloneContainer = "git-clone"
)

const (
	CustomBuild = "custom-build"
	DockerBuild = "docker-build"
	StiBuild    = "sti-build"
)

var BuildContainerNames = []string{CustomBuild, StiBuild, DockerBuild}

const (
	// NoBuildLogsMessage reports that no build logs are available
	NoBuildLogsMessage = "No logs are available."
)

package api

// Synthetic authorization endpoints
const (
	DockerBuildResource          = "builds/docker"
	SourceBuildResource          = "builds/source"
	CustomBuildResource          = "builds/custom"
	JenkinsPipelineBuildResource = "builds/jenkinspipeline"

	// these are valid under the "nodes" resource
	NodeMetricsSubresource = "metrics"
	NodeStatsSubresource   = "stats"
	NodeSpecSubresource    = "spec"
	NodeLogSubresource     = "log"

	RestrictedEndpointsResource = "endpoints/restricted"
)

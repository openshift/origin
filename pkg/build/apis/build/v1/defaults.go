package v1

import "github.com/openshift/api/build/v1"

func SetDefaults_BuildConfigSpec(config *v1.BuildConfigSpec) {
	if len(config.RunPolicy) == 0 {
		config.RunPolicy = v1.BuildRunPolicySerial
	}
}

func SetDefaults_BuildSource(source *v1.BuildSource) {
	if (source != nil) && (source.Type == v1.BuildSourceBinary) && (source.Binary == nil) {
		source.Binary = &v1.BinaryBuildSource{}
	}
}

func SetDefaults_BuildStrategy(strategy *v1.BuildStrategy) {
	if (strategy != nil) && (strategy.Type == v1.DockerBuildStrategyType) && (strategy.DockerStrategy == nil) {
		strategy.DockerStrategy = &v1.DockerBuildStrategy{}
	}
}

func SetDefaults_SourceBuildStrategy(obj *v1.SourceBuildStrategy) {
	if len(obj.From.Kind) == 0 {
		obj.From.Kind = "ImageStreamTag"
	}
}

func SetDefaults_DockerBuildStrategy(obj *v1.DockerBuildStrategy) {
	if obj.From != nil && len(obj.From.Kind) == 0 {
		obj.From.Kind = "ImageStreamTag"
	}
}

func SetDefaults_CustomBuildStrategy(obj *v1.CustomBuildStrategy) {
	if len(obj.From.Kind) == 0 {
		obj.From.Kind = "ImageStreamTag"
	}
}

func SetDefaults_BuildTriggerPolicy(obj *v1.BuildTriggerPolicy) {
	if obj.Type == v1.ImageChangeBuildTriggerType && obj.ImageChange == nil {
		obj.ImageChange = &v1.ImageChangeTrigger{}
	}
}

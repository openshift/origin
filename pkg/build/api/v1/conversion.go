package v1

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	err := kapi.Scheme.AddDefaultingFuncs(
		func(strategy *BuildStrategy) {
			if (strategy != nil) && (strategy.Type == DockerBuildStrategyType) {
				//  initialize DockerStrategy to a default state if it's not set.
				if strategy.DockerStrategy == nil {
					strategy.DockerStrategy = &DockerBuildStrategy{}
				}
			}
		},
		func(obj *SourceBuildStrategy) {
			if len(obj.From.Kind) == 0 {
				obj.From.Kind = "ImageStreamTag"
			}
		},
		func(obj *DockerBuildStrategy) {
			if obj.From != nil && len(obj.From.Kind) == 0 {
				obj.From.Kind = "ImageStreamTag"
			}
		},
		func(obj *CustomBuildStrategy) {
			if len(obj.From.Kind) == 0 {
				obj.From.Kind = "ImageStreamTag"
			}
		},
		func(obj *BuildTriggerPolicy) {
			if obj.Type == ImageChangeBuildTriggerType && obj.ImageChange == nil {
				obj.ImageChange = &ImageChangeTrigger{}
			}
		},
	)
	if err != nil {
		panic(err)
	}

	// Add field conversion funcs.
	err = kapi.Scheme.AddFieldLabelConversionFunc("v1", "Build",
		func(label, value string) (string, string, error) {
			switch label {
			case "name":
				return "metadata.name", value, nil
			case "status":
				return "status", value, nil
			case "podName":
				return "podName", value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
	err = kapi.Scheme.AddFieldLabelConversionFunc("v1", "BuildConfig",
		func(label, value string) (string, string, error) {
			switch label {
			case "name":
				return "metadata.name", value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}

package build

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/golang/glog"
)

// BuildToPodLogOptions builds a PodLogOptions object out of a BuildLogOptions.
// Currently BuildLogOptions.Container and BuildLogOptions.Previous aren't used
// so they won't be copied to PodLogOptions.
func BuildToPodLogOptions(opts *BuildLogOptions) *kapi.PodLogOptions {
	return &kapi.PodLogOptions{
		Follow:       opts.Follow,
		SinceSeconds: opts.SinceSeconds,
		SinceTime:    opts.SinceTime,
		Timestamps:   opts.Timestamps,
		TailLines:    opts.TailLines,
		LimitBytes:   opts.LimitBytes,
	}
}

// PredicateFunc is testing an argument and decides does it meet some criteria or not.
// It can be used for filtering elements based on some conditions.
type PredicateFunc func(interface{}) bool

// FilterBuilds returns array of builds that satisfies predicate function.
func FilterBuilds(builds []Build, predicate PredicateFunc) []Build {
	if len(builds) == 0 {
		return builds
	}

	result := make([]Build, 0)
	for _, build := range builds {
		if predicate(build) {
			result = append(result, build)
		}
	}

	return result
}

// ByBuildConfigPredicate matches all builds that have build config annotation or label with specified value.
func ByBuildConfigPredicate(labelValue string) PredicateFunc {
	return func(arg interface{}) bool {
		return (hasBuildConfigAnnotation(arg.(Build), BuildConfigAnnotation, labelValue) ||
			hasBuildConfigLabel(arg.(Build), BuildConfigLabel, labelValue) ||
			hasBuildConfigLabel(arg.(Build), BuildConfigLabelDeprecated, labelValue))
	}
}

func hasBuildConfigLabel(build Build, labelName, labelValue string) bool {
	value, ok := build.Labels[labelName]
	return ok && value == labelValue
}

func hasBuildConfigAnnotation(build Build, annotationName, annotationValue string) bool {
	if build.Annotations == nil {
		return false
	}
	value, ok := build.Annotations[annotationName]
	return ok && value == annotationValue
}

// FindTriggerPolicy retrieves the BuildTrigger(s) of a given type from a build configuration.
// Returns nil if no matches are found.
func FindTriggerPolicy(triggerType BuildTriggerType, config *BuildConfig) (buildTriggers []BuildTriggerPolicy) {
	for _, specTrigger := range config.Spec.Triggers {
		if specTrigger.Type == triggerType {
			buildTriggers = append(buildTriggers, specTrigger)
		}
	}
	return buildTriggers
}

func HasTriggerType(triggerType BuildTriggerType, bc *BuildConfig) bool {
	matches := FindTriggerPolicy(triggerType, bc)
	return len(matches) > 0
}

// RecordStageAndStepInfo records details about each build stage and step
func RecordStageAndStepInfo(stages []StageInfo, stageName StageName, stepName StepName, startTime metav1.Time, endTime metav1.Time) []StageInfo {
	// If the stage already exists in the slice, update the DurationMilliseconds, and append the new step.
	for stageKey, stageVal := range stages {
		if stageVal.Name == stageName {
			for _, step := range stages[stageKey].Steps {
				if step.Name == stepName {
					glog.V(4).Infof("error recording build timing information, step %v already exists in stage %v", stepName, stageName)
				}
			}
			stages[stageKey].DurationMilliseconds = endTime.Time.Sub(stages[stageKey].StartTime.Time).Nanoseconds() / int64(time.Millisecond)
			if len(stages[stageKey].Steps) == 0 {
				stages[stageKey].Steps = make([]StepInfo, 0)
			}
			stages[stageKey].Steps = append(stages[stageKey].Steps, StepInfo{
				Name:                 stepName,
				StartTime:            startTime,
				DurationMilliseconds: endTime.Time.Sub(startTime.Time).Nanoseconds() / int64(time.Millisecond),
			})
			return stages
		}
	}

	// If the stageName does not exist, add it to the slice along with the new step.
	var steps []StepInfo
	steps = append(steps, StepInfo{
		Name:                 stepName,
		StartTime:            startTime,
		DurationMilliseconds: endTime.Time.Sub(startTime.Time).Nanoseconds() / int64(time.Millisecond),
	})
	stages = append(stages, StageInfo{
		Name:                 stageName,
		StartTime:            startTime,
		DurationMilliseconds: endTime.Time.Sub(startTime.Time).Nanoseconds() / int64(time.Millisecond),
		Steps:                steps,
	})
	return stages
}

// AppendStageAndStepInfo appends the step info from one stages slice into another.
func AppendStageAndStepInfo(stages []StageInfo, stagesToMerge []StageInfo) []StageInfo {
	for _, stage := range stagesToMerge {
		for _, step := range stage.Steps {
			stages = RecordStageAndStepInfo(stages, stage.Name, step.Name, step.StartTime, metav1.NewTime(step.StartTime.Add(time.Duration(step.DurationMilliseconds)*time.Millisecond)))
		}
	}
	return stages
}

// GetInputReference returns the From ObjectReference associated with the
// BuildStrategy.
func GetInputReference(strategy BuildStrategy) *kapi.ObjectReference {
	switch {
	case strategy.SourceStrategy != nil:
		return &strategy.SourceStrategy.From
	case strategy.DockerStrategy != nil:
		return strategy.DockerStrategy.From
	case strategy.CustomStrategy != nil:
		return &strategy.CustomStrategy.From
	default:
		return nil
	}
}

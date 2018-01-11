package timing

import (
	"context"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildapiv1 "github.com/openshift/api/build/v1"
)

type key int

var timingKey key

// NewContext returns a context initialised for use
func NewContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, timingKey, &[]buildapiv1.StageInfo{})
}

// fromContext returns the existing data stored in the context
func fromContext(ctx context.Context) *[]buildapiv1.StageInfo {
	return ctx.Value(timingKey).(*[]buildapiv1.StageInfo)
}

// RecordNewStep adds a new timing step to the context
func RecordNewStep(ctx context.Context, stageName buildapiv1.StageName, stepName buildapiv1.StepName, startTime metav1.Time, endTime metav1.Time) {
	stages := fromContext(ctx)
	newStages := RecordStageAndStepInfo(*stages, stageName, stepName, startTime, endTime)
	*stages = newStages
}

// GetStages returns all stages and steps currently stored in the context
func GetStages(ctx context.Context) []buildapiv1.StageInfo {
	stages := fromContext(ctx)
	return *stages
}

// RecordStageAndStepInfo records details about each build stage and step
func RecordStageAndStepInfo(stages []buildapiv1.StageInfo, stageName buildapiv1.StageName, stepName buildapiv1.StepName, startTime metav1.Time, endTime metav1.Time) []buildapiv1.StageInfo {
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
				stages[stageKey].Steps = make([]buildapiv1.StepInfo, 0)
			}
			stages[stageKey].Steps = append(stages[stageKey].Steps, buildapiv1.StepInfo{
				Name:                 stepName,
				StartTime:            startTime,
				DurationMilliseconds: endTime.Time.Sub(startTime.Time).Nanoseconds() / int64(time.Millisecond),
			})
			return stages
		}
	}

	// If the stageName does not exist, add it to the slice along with the new step.
	var steps []buildapiv1.StepInfo
	steps = append(steps, buildapiv1.StepInfo{
		Name:                 stepName,
		StartTime:            startTime,
		DurationMilliseconds: endTime.Time.Sub(startTime.Time).Nanoseconds() / int64(time.Millisecond),
	})
	stages = append(stages, buildapiv1.StageInfo{
		Name:                 stageName,
		StartTime:            startTime,
		DurationMilliseconds: endTime.Time.Sub(startTime.Time).Nanoseconds() / int64(time.Millisecond),
		Steps:                steps,
	})
	return stages
}

// AppendStageAndStepInfo appends the step info from one stages slice into another.
func AppendStageAndStepInfo(stages []buildapiv1.StageInfo, stagesToMerge []buildapiv1.StageInfo) []buildapiv1.StageInfo {
	for _, stage := range stagesToMerge {
		for _, step := range stage.Steps {
			stages = RecordStageAndStepInfo(stages, stage.Name, step.Name, step.StartTime, metav1.NewTime(step.StartTime.Add(time.Duration(step.DurationMilliseconds)*time.Millisecond)))
		}
	}
	return stages
}

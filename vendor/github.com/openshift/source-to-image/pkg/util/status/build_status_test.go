package status

import (
	"testing"
	"time"

	"github.com/openshift/source-to-image/pkg/api"
)

func TestNewFailureReason(t *testing.T) {
	failureReason := NewFailureReason(ReasonAssembleFailed, ReasonMessageAssembleFailed)

	if failureReason.Reason != ReasonAssembleFailed {
		t.Errorf("Expected reason to be: %s, got %s", ReasonAssembleFailed, failureReason.Reason)
	}

	if failureReason.Message != ReasonMessageAssembleFailed {
		t.Errorf("Expected message reason to be: %s, got %s", ReasonMessageAssembleFailed, failureReason.Message)
	}
}

func TestAddNewStage(t *testing.T) {
	buildInfo := new(api.BuildInfo)

	buildInfo.Stages = api.RecordStageAndStepInfo(buildInfo.Stages, api.StagePullImages, api.StepPullPreviousImage, time.Now(), time.Now())

	if len(buildInfo.Stages) != 1 {
		t.Errorf("Stage not added wanted 1, got %#v", len(buildInfo.Stages))
	}
}

func TestAddNewStages(t *testing.T) {
	buildInfo := new(api.BuildInfo)

	if len(buildInfo.Stages) > 0 {
		t.Errorf("Stages should be 0 but was %v instead.", len(buildInfo.Stages))
	}
	buildInfo.Stages = api.RecordStageAndStepInfo(buildInfo.Stages, api.StagePullImages, api.StepPullBuilderImage, time.Now(), time.Now())

	if len(buildInfo.Stages) != 1 {
		t.Errorf("Stages should be 1 but was %v instead.", len(buildInfo.Stages))
	}
	buildInfo.Stages = api.RecordStageAndStepInfo(buildInfo.Stages, api.StageBuild, api.StepBuildDockerImage, time.Now(), time.Now())
	if len(buildInfo.Stages) != 2 {
		t.Errorf("Stages should be 2 but was %v instead.", len(buildInfo.Stages))
	}
}

func TestAddNewStepToStage(t *testing.T) {
	buildInfo := new(api.BuildInfo)

	buildInfo.Stages = api.RecordStageAndStepInfo(buildInfo.Stages, api.StagePullImages, api.StepPullPreviousImage, time.Now(), time.Now())
	buildInfo.Stages = api.RecordStageAndStepInfo(buildInfo.Stages, api.StagePullImages, api.StepPullBuilderImage, time.Now(), time.Now())

	if len(buildInfo.Stages[0].Steps) != 2 {
		t.Errorf("Step not added in Stage, wanted 2, got %#v", len(buildInfo.Stages[0].Steps))
	}
}

func TestUpdateStageDuration(t *testing.T) {
	buildInfo := new(api.BuildInfo)

	startTime := time.Now()

	buildInfo.Stages = api.RecordStageAndStepInfo(buildInfo.Stages, api.StagePullImages, api.StepPullPreviousImage, startTime, time.Now())

	addDuration, _ := time.ParseDuration("5m")

	endTime := time.Now().Add(addDuration)

	buildInfo.Stages = api.RecordStageAndStepInfo(buildInfo.Stages, api.StagePullImages, api.StepPullBuilderImage, time.Now(), endTime)

	if buildInfo.Stages[0].DurationMilliseconds != (endTime.Sub(startTime).Nanoseconds() / int64(time.Millisecond)) {
		t.Errorf("Stage Duration was not updated, expected %#v, got %#v", (endTime.Sub(startTime).Nanoseconds() / int64(time.Millisecond)), buildInfo.Stages[0].DurationMilliseconds)
	}

}

package timing

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

func TestRecordStageAndStepInfo(t *testing.T) {
	var stages []buildapi.StageInfo

	stages = buildapi.RecordStageAndStepInfo(stages, buildapi.StageFetchInputs, buildapi.StepFetchGitSource, metav1.Now(), metav1.Now())

	if len(stages) != 1 || len(stages[0].Steps) != 1 {
		t.Errorf("There should be 1 stage and 1 step, but instead there were %v stage(s) and %v step(s).", len(stages), len(stages[0].Steps))
	}

	stages = buildapi.RecordStageAndStepInfo(stages, buildapi.StagePullImages, buildapi.StepPullBaseImage, metav1.Now(), metav1.Now())
	stages = buildapi.RecordStageAndStepInfo(stages, buildapi.StagePullImages, buildapi.StepPullInputImage, metav1.Now(), metav1.Now())

	if len(stages) != 2 || len(stages[1].Steps) != 2 {
		t.Errorf("There should be 2 stages and 2 steps under the second stage, but instead there were %v stage(s) and %v step(s).", len(stages), len(stages[1].Steps))
	}

}

func TestAppendStageAndStepInfo(t *testing.T) {
	var stages []buildapi.StageInfo
	var stagesToMerge []buildapi.StageInfo

	stages = buildapi.RecordStageAndStepInfo(stages, buildapi.StagePullImages, buildapi.StepPullBaseImage, metav1.Now(), metav1.Now())
	stages = buildapi.RecordStageAndStepInfo(stages, buildapi.StagePullImages, buildapi.StepPullInputImage, metav1.Now(), metav1.Now())

	stagesToMerge = buildapi.RecordStageAndStepInfo(stagesToMerge, buildapi.StagePushImage, buildapi.StepPushImage, metav1.Now(), metav1.Now())
	stagesToMerge = buildapi.RecordStageAndStepInfo(stagesToMerge, buildapi.StagePostCommit, buildapi.StepExecPostCommitHook, metav1.Now(), metav1.Now())

	stages = buildapi.AppendStageAndStepInfo(stages, stagesToMerge)

	if len(stages) != 3 {
		t.Errorf("There should be 3 stages, but instead there were %v stage(s).", len(stages))
	}

}

func TestTimingContextGetStages(t *testing.T) {
	ctx := NewContext(context.Background())

	RecordNewStep(ctx, buildapi.StagePullImages, buildapi.StepPullBaseImage, metav1.Now(), metav1.Now())
	RecordNewStep(ctx, buildapi.StageFetchInputs, buildapi.StepFetchGitSource, metav1.Now(), metav1.Now())
	RecordNewStep(ctx, buildapi.StagePostCommit, buildapi.StepExecPostCommitHook, metav1.Now(), metav1.Now())

	stages := GetStages(ctx)
	if len(stages) != 3 {
		t.Errorf("There should be 3 stages but instead there are %v stage(s).", len(stages))
	}
}

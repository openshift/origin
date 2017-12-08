package timing

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildapiv1 "github.com/openshift/api/build/v1"
)

func TestRecordStageAndStepInfo(t *testing.T) {
	var stages []buildapiv1.StageInfo

	stages = RecordStageAndStepInfo(stages, buildapiv1.StageFetchInputs, buildapiv1.StepFetchGitSource, metav1.Now(), metav1.Now())

	if len(stages) != 1 || len(stages[0].Steps) != 1 {
		t.Errorf("There should be 1 stage and 1 step, but instead there were %v stage(s) and %v step(s).", len(stages), len(stages[0].Steps))
	}

	stages = RecordStageAndStepInfo(stages, buildapiv1.StagePullImages, buildapiv1.StepPullBaseImage, metav1.Now(), metav1.Now())
	stages = RecordStageAndStepInfo(stages, buildapiv1.StagePullImages, buildapiv1.StepPullInputImage, metav1.Now(), metav1.Now())

	if len(stages) != 2 || len(stages[1].Steps) != 2 {
		t.Errorf("There should be 2 stages and 2 steps under the second stage, but instead there were %v stage(s) and %v step(s).", len(stages), len(stages[1].Steps))
	}

}

func TestAppendStageAndStepInfo(t *testing.T) {
	var stages []buildapiv1.StageInfo
	var stagesToMerge []buildapiv1.StageInfo

	stages = RecordStageAndStepInfo(stages, buildapiv1.StagePullImages, buildapiv1.StepPullBaseImage, metav1.Now(), metav1.Now())
	stages = RecordStageAndStepInfo(stages, buildapiv1.StagePullImages, buildapiv1.StepPullInputImage, metav1.Now(), metav1.Now())

	stagesToMerge = RecordStageAndStepInfo(stagesToMerge, buildapiv1.StagePushImage, buildapiv1.StepPushImage, metav1.Now(), metav1.Now())
	stagesToMerge = RecordStageAndStepInfo(stagesToMerge, buildapiv1.StagePostCommit, buildapiv1.StepExecPostCommitHook, metav1.Now(), metav1.Now())

	stages = AppendStageAndStepInfo(stages, stagesToMerge)

	if len(stages) != 3 {
		t.Errorf("There should be 3 stages, but instead there were %v stage(s).", len(stages))
	}

}

func TestTimingContextGetStages(t *testing.T) {
	ctx := NewContext(context.Background())

	RecordNewStep(ctx, buildapiv1.StagePullImages, buildapiv1.StepPullBaseImage, metav1.Now(), metav1.Now())
	RecordNewStep(ctx, buildapiv1.StageFetchInputs, buildapiv1.StepFetchGitSource, metav1.Now(), metav1.Now())
	RecordNewStep(ctx, buildapiv1.StagePostCommit, buildapiv1.StepExecPostCommitHook, metav1.Now(), metav1.Now())

	stages := GetStages(ctx)
	if len(stages) != 3 {
		t.Errorf("There should be 3 stages but instead there are %v stage(s).", len(stages))
	}
}

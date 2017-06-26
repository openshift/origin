package timing

import (
	"context"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type key int

var timingKey key

// NewContext returns a context initialised for use
func NewContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, timingKey, &[]buildapi.StageInfo{})
}

// fromContext returns the existing data stored in the context
func fromContext(ctx context.Context) *[]buildapi.StageInfo {
	return ctx.Value(timingKey).(*[]buildapi.StageInfo)
}

// RecordNewStep adds a new timing step to the context
func RecordNewStep(ctx context.Context, stageName buildapi.StageName, stepName buildapi.StepName, startTime metav1.Time, endTime metav1.Time) {
	stages := fromContext(ctx)
	newStages := buildapi.RecordStageAndStepInfo(*stages, stageName, stepName, startTime, endTime)
	*stages = newStages
}

// GetStages returns all stages and steps currently stored in the context
func GetStages(ctx context.Context) []buildapi.StageInfo {
	stages := fromContext(ctx)
	return *stages
}

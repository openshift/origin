package timing

import (
	"context"

	"github.com/openshift/origin/pkg/build/api"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type key int

var timingKey key

// NewContext returns a context initialised for use
func NewContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, timingKey, &[]api.StageInfo{})
}

// fromContext returns the existing data stored in the context
func fromContext(ctx context.Context) *[]api.StageInfo {
	return ctx.Value(timingKey).(*[]api.StageInfo)
}

// RecordNewStep adds a new timing step to the context
func RecordNewStep(ctx context.Context, stageName api.StageName, stepName api.StepName, startTime metav1.Time, endTime metav1.Time) {
	stages := fromContext(ctx)
	newStages := api.RecordStageAndStepInfo(*stages, stageName, stepName, startTime, endTime)
	*stages = newStages
}

// GetStages returns all stages and steps currently stored in the context
func GetStages(ctx context.Context) []api.StageInfo {
	stages := fromContext(ctx)
	return *stages
}

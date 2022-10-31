package util

import (
	"context"
	"strings"
	"time"

	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	describeKey  = "describe"
	testNameKey  = "testName"
	stepNamesKey = "stepNames"
)

func WithDescription(parent context.Context, description string) context.Context {
	return context.WithValue(parent, describeKey, description)
}

func DescriptionFrom(ctx context.Context) (string, bool) {
	description, ok := ctx.Value(describeKey).(string)
	return description, ok
}

func WithTestName(parent context.Context, testName string) context.Context {
	return context.WithValue(parent, testNameKey, testName)
}

func TestNameFrom(ctx context.Context) (string, bool) {
	testName, ok := ctx.Value(testNameKey).(string)
	return testName, ok
}

func AddStep(parent context.Context, stepName string) context.Context {
	stepNamesToSet := []string{}
	if currSteps, ok := StepsFrom(parent); ok {
		stepNamesToSet = append(stepNamesToSet, currSteps...)
	}
	stepNamesToSet = append(stepNamesToSet, stepName)
	return context.WithValue(parent, stepNamesKey, stepNamesToSet)
}

func StepsFrom(ctx context.Context) ([]string, bool) {
	stepNames, ok := ctx.Value(stepNamesKey).([]string)
	return stepNames, ok
}

func StepEnd(ctx context.Context, startTime time.Time) {
	description, _ := DescriptionFrom(ctx)
	testName, _ := TestNameFrom(ctx)
	endTime := time.Now()
	steps, _ := StepsFrom(ctx)

	e2e.Logf("TEST TIMING: %q -- for step (%v) -- %v", description+" "+testName, strings.Join(steps, "."), endTime.Sub(startTime))
}

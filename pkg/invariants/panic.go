package invariants

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

func startCollectionWithPanicProtection(ctx context.Context, invariantTest InvariantTest, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("caught panic: %v", r)
		}
	}()

	err = invariantTest.StartCollection(ctx, adminRESTConfig, recorder)
	return
}

func collectDataWithPanicProtection(ctx context.Context, invariantTest InvariantTest, storageDir string, beginning, end time.Time) (intervals monitorapi.Intervals, junit []*junitapi.JUnitTestCase, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("caught panic: %v", r)
		}
	}()

	intervals, junit, err = invariantTest.CollectData(ctx, storageDir, beginning, end)
	return
}

func constructComputedIntervalsWithPanicProtection(ctx context.Context, invariantTest InvariantTest, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (intervals monitorapi.Intervals, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("caught panic: %v", r)
		}
	}()

	intervals, err = invariantTest.ConstructComputedIntervals(ctx, startingIntervals, recordedResources, beginning, end)
	return
}

func evaluateTestsFromConstructedIntervalsWithPanicProtection(ctx context.Context, invariantTest InvariantTest, finalIntervals monitorapi.Intervals) (junits []*junitapi.JUnitTestCase, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("caught panic: %v", r)
		}
	}()

	junits, err = invariantTest.EvaluateTestsFromConstructedIntervals(ctx, finalIntervals)
	return
}

func writeContentToStorageWithPanicProtection(ctx context.Context, invariantTest InvariantTest, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("caught panic: %v", r)
		}
	}()

	err = invariantTest.WriteContentToStorage(ctx, storageDir, timeSuffix, finalIntervals, finalResourceState)
	return
}

func cleanupWithPanicProtection(ctx context.Context, invariantTest InvariantTest) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("caught panic: %v", r)
		}
	}()

	err = invariantTest.Cleanup(ctx)
	return
}

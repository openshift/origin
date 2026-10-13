package monitortestframework

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

func prepareCollectionWithPanicProtection(ctx context.Context, monitortest MonitorTest, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("caught panic: %v", r)
			logrus.Error("recovering from panic")
			fmt.Print(string(debug.Stack()))
		}
	}()

	err = monitortest.PrepareCollection(ctx, adminRESTConfig, recorder)
	return
}

func startCollectionWithPanicProtection(ctx context.Context, monitortest MonitorTest, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("caught panic: %v", r)
			logrus.Error("recovering from panic")
			fmt.Print(string(debug.Stack()))
		}
	}()

	err = monitortest.StartCollection(ctx, adminRESTConfig, recorder)
	return
}

func collectDataWithPanicProtection(ctx context.Context, monitortest MonitorTest, storageDir string, beginning, end time.Time) (intervals monitorapi.Intervals, junit []*junitapi.JUnitTestCase, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("caught panic: %v", r)
			logrus.Error("recovering from panic")
			fmt.Print(string(debug.Stack()))
		}
	}()

	intervals, junit, err = monitortest.CollectData(ctx, storageDir, beginning, end)
	return
}

func constructComputedIntervalsWithPanicProtection(ctx context.Context, monitortest MonitorTest, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (intervals monitorapi.Intervals, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("caught panic: %v", r)
			logrus.Error("recovering from panic")
			fmt.Print(string(debug.Stack()))
		}
	}()

	intervals, err = monitortest.ConstructComputedIntervals(ctx, startingIntervals, recordedResources, beginning, end)
	return
}

func evaluateTestsFromConstructedIntervalsWithPanicProtection(ctx context.Context, monitortest MonitorTest, finalIntervals monitorapi.Intervals) (junits []*junitapi.JUnitTestCase, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("caught panic: %v", r)
			logrus.Error("recovering from panic")
			fmt.Print(string(debug.Stack()))
		}
	}()

	junits, err = monitortest.EvaluateTestsFromConstructedIntervals(ctx, finalIntervals)
	return
}

func writeContentToStorageWithPanicProtection(ctx context.Context, monitortest MonitorTest, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("caught panic: %v", r)
			logrus.Error("recovering from panic")
			fmt.Print(string(debug.Stack()))
		}
	}()

	err = monitortest.WriteContentToStorage(ctx, storageDir, timeSuffix, finalIntervals, finalResourceState)
	return
}

func cleanupWithPanicProtection(ctx context.Context, monitortest MonitorTest) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("caught panic: %v", r)
			logrus.WithError(err).Error("recovering from panic")
			fmt.Print(string(debug.Stack()))
		}
	}()

	err = monitortest.Cleanup(ctx)
	return
}

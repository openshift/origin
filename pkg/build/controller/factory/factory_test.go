package factory

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kutil "k8s.io/kubernetes/pkg/util"

	buildapi "github.com/openshift/origin/pkg/build/api"
	controller "github.com/openshift/origin/pkg/controller"
)

type buildUpdater struct {
	Build *buildapi.Build
}

func (b *buildUpdater) Update(namespace string, build *buildapi.Build) error {
	b.Build = build
	return nil
}

func TestLimitedLogAndRetryFinish(t *testing.T) {
	updater := &buildUpdater{}
	err := errors.New("funky error")

	now := kutil.Now()
	retry := controller.Retry{
		Count:          0,
		StartTimestamp: kutil.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute()-31, now.Second(), now.Nanosecond(), now.Location()),
	}
	if limitedLogAndRetry(updater, 30*time.Minute)(&buildapi.Build{Status: buildapi.BuildStatus{Phase: buildapi.BuildPhaseNew}}, err, retry) {
		t.Error("Expected no more retries after reaching timeout!")
	}
	if updater.Build == nil {
		t.Fatal("BuildUpdater wasn't called!")
	}
	if updater.Build.Status.Phase != buildapi.BuildPhaseFailed {
		t.Errorf("Expected status %s, got %s!", buildapi.BuildPhaseFailed, updater.Build.Status.Phase)
	}
	if !strings.Contains(updater.Build.Status.Message, err.Error()) {
		t.Errorf("Expected message to contain %v, got %s!", err.Error(), updater.Build.Status.Message)
	}
	if updater.Build.Status.CompletionTimestamp == nil {
		t.Error("Expected CompletionTimestamp to be set!")
	}
}

func TestLimitedLogAndRetryProcessing(t *testing.T) {
	updater := &buildUpdater{}
	err := errors.New("funky error")

	now := kutil.Now()
	retry := controller.Retry{
		Count:          0,
		StartTimestamp: kutil.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute()-10, now.Second(), now.Nanosecond(), now.Location()),
	}
	if !limitedLogAndRetry(updater, 30*time.Minute)(&buildapi.Build{Status: buildapi.BuildStatus{Phase: buildapi.BuildPhaseNew}}, err, retry) {
		t.Error("Expected more retries!")
	}
	if updater.Build != nil {
		t.Fatal("BuildUpdater shouldn't be called!")
	}
}

func TestControllerRetryFunc(t *testing.T) {
	obj := &kapi.Pod{}
	obj.Name = "testpod"
	obj.Namespace = "testNS"

	testErr := fmt.Errorf("test error")
	tests := []struct {
		name       string
		retryCount int
		isFatal    func(err error) bool
		err        error
		expect     bool
	}{
		{
			name:       "maxRetries-1 retries",
			retryCount: maxRetries - 1,
			err:        testErr,
			expect:     true,
		},
		{
			name:       "maxRetries+1 retries",
			retryCount: maxRetries + 1,
			err:        testErr,
			expect:     false,
		},
		{
			name:       "isFatal returns true",
			retryCount: 0,
			err:        testErr,
			isFatal: func(err error) bool {
				if err != testErr {
					t.Errorf("Unexpected error: %v", err)
				}
				return true
			},
			expect: false,
		},
		{
			name:       "isFatal returns false",
			retryCount: 0,
			err:        testErr,
			isFatal: func(err error) bool {
				if err != testErr {
					t.Errorf("Unexpected error: %v", err)
				}
				return false
			},
			expect: true,
		},
	}

	for _, tc := range tests {
		f := retryFunc("test kind", tc.isFatal)
		result := f(obj, tc.err, controller.Retry{Count: tc.retryCount})
		if result != tc.expect {
			t.Errorf("%s: unexpected result. Expected: %v. Got: %v", tc.name, tc.expect, result)
		}
	}
}

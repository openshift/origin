package factory

import (
	"errors"
	"strings"
	"testing"
	"time"

	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
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
	retry := controller.Retry{0, kutil.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute()-31, now.Second(), now.Nanosecond(), now.Location())}
	if limitedLogAndRetry(updater, 30*time.Minute)(&buildapi.Build{Status: buildapi.BuildStatusNew}, err, retry) {
		t.Error("Expected no more retries after reaching timeout!")
	}
	if updater.Build == nil {
		t.Fatal("BuildUpdater wasn't called!")
	}
	if updater.Build.Status != buildapi.BuildStatusFailed {
		t.Errorf("Expected status %s, got %s!", buildapi.BuildStatusFailed, updater.Build.Status)
	}
	if !strings.Contains(updater.Build.Message, err.Error()) {
		t.Errorf("Expected message to contain %v, got %s!", err, updater.Build.Status)
	}
	if updater.Build.CompletionTimestamp == nil {
		t.Error("Expected CompletionTimestamp to be set!")
	}
}

func TestLimitedLogAndRetryProcessing(t *testing.T) {
	updater := &buildUpdater{}
	err := errors.New("funky error")

	now := kutil.Now()
	retry := controller.Retry{0, kutil.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute()-10, now.Second(), now.Nanosecond(), now.Location())}
	if !limitedLogAndRetry(updater, 30*time.Minute)(&buildapi.Build{Status: buildapi.BuildStatusNew}, err, retry) {
		t.Error("Expected more retries!")
	}
	if updater.Build != nil {
		t.Fatal("BuildUpdater shouldn't be called!")
	}
}

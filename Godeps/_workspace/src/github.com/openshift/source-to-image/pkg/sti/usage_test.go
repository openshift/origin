package sti

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/openshift/source-to-image/pkg/sti/api"
)

type FakeUsageHandler struct {
	cleanupCalled  bool
	setupRequired  []api.Script
	setupOptional  []api.Script
	setupError     error
	executeCommand api.Script
	executeError   error
}

func (f *FakeUsageHandler) cleanup() {
	f.cleanupCalled = true
}

func (f *FakeUsageHandler) setup(required []api.Script, optional []api.Script) error {
	f.setupRequired = required
	f.setupOptional = optional
	return f.setupError
}

func (f *FakeUsageHandler) execute(command api.Script) error {
	f.executeCommand = command
	return f.executeError
}

func newTestUsage() *Usage {
	return &Usage{
		handler: &FakeUsageHandler{},
	}
}

func TestUsage(t *testing.T) {
	u := newTestUsage()
	fh := u.handler.(*FakeUsageHandler)
	err := u.Show()
	if err != nil {
		t.Errorf("Unexpected error returned from Usage: %v", err)
	}
	if !reflect.DeepEqual(fh.setupOptional, []api.Script{}) {
		t.Errorf("setup called with unexpected optional scripts: %#v", fh.setupOptional)
	}
	if !reflect.DeepEqual(fh.setupRequired, []api.Script{api.Usage}) {
		t.Errorf("setup called with unexpected required scripts: %#v", fh.setupRequired)
	}
	if fh.executeCommand != "usage" {
		t.Errorf("execute called with unexpected command: %#v", fh.executeCommand)
	}
	if !fh.cleanupCalled {
		t.Errorf("cleanup was not called from usage.")
	}
}

func TestUsageSetupError(t *testing.T) {
	u := newTestUsage()
	fh := u.handler.(*FakeUsageHandler)
	fh.setupError = fmt.Errorf("setup error")
	err := u.Show()
	if err != fh.setupError {
		t.Errorf("Unexpected error returned from Usage: %v", err)
	}
	if fh.executeCommand != "" {
		t.Errorf("Execute called when there was a setup error.")
	}
}

func TestUsageExecuteError(t *testing.T) {
	u := newTestUsage()
	fh := u.handler.(*FakeUsageHandler)
	fh.executeError = fmt.Errorf("execute error")
	err := u.Show()
	if err != fh.executeError {
		t.Errorf("Unexpected error returned from Usage: %v", err)
	}
}

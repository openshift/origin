package sti

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
)

type FakeUsageHandler struct {
	cleanupCalled  bool
	setupRequired  []string
	setupOptional  []string
	setupError     error
	executeCommand string
	executeError   error
}

type FakeCleaner struct {
	cleanupCalled bool
}

func (f *FakeCleaner) Cleanup(*api.Config) {
	f.cleanupCalled = true
}

func (f *FakeUsageHandler) Prepare(*api.Config) error {
	return f.setupError
}

func (f *FakeUsageHandler) SetScripts(r, o []string) {
	f.setupRequired = r
	f.setupOptional = o
}

func (f *FakeUsageHandler) Execute(command string, r *api.Config) error {
	f.executeCommand = command
	return f.executeError
}

func (f *FakeUsageHandler) Download(*api.Config) error {
	return nil
}

func newTestUsage() *Usage {
	return &Usage{
		handler: &FakeUsageHandler{},
	}
}

func TestUsage(t *testing.T) {
	u := newTestUsage()
	g := &FakeCleaner{}
	u.garbage = g
	fh := u.handler.(*FakeUsageHandler)
	err := u.Show()
	if err != nil {
		t.Errorf("Unexpected error returned from Usage: %v", err)
	}
	if !reflect.DeepEqual(fh.setupOptional, []string{}) {
		t.Errorf("setup called with unexpected optional scripts: %#v", fh.setupOptional)
	}
	if !reflect.DeepEqual(fh.setupRequired, []string{api.Usage}) {
		t.Errorf("setup called with unexpected required scripts: %#v", fh.setupRequired)
	}
	if fh.executeCommand != "usage" {
		t.Errorf("execute called with unexpected command: %#v", fh.executeCommand)
	}
	if !g.cleanupCalled {
		t.Errorf("cleanup was not called from usage.")
	}
}

func TestUsageSetupError(t *testing.T) {
	u := newTestUsage()
	u.garbage = &FakeCleaner{}
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
	u.garbage = &FakeCleaner{}
	fh := u.handler.(*FakeUsageHandler)
	fh.executeError = fmt.Errorf("execute error")
	err := u.Show()
	if err != fh.executeError {
		t.Errorf("Unexpected error returned from Usage: %v", err)
	}
}

package fileobserver

import (
	"fmt"
	"os"
	"time"

	"k8s.io/klog"
)

type Observer interface {
	Run(stopChan <-chan struct{})
	HasSynced() bool
	AddReactor(reaction ReactorFn, startingFileContent map[string][]byte, files ...string) Observer
}

// ActionType define a type of action observed on the file
type ActionType int

const (
	// FileModified means the file content was modified.
	FileModified ActionType = iota

	// FileCreated means the file was just created.
	FileCreated

	// FileDeleted means the file was deleted.
	FileDeleted
)

// String returns human readable form of action taken on a file.
func (t ActionType) String(filename string) string {
	switch t {
	case FileCreated:
		return fmt.Sprintf("file %s was created", filename)
	case FileDeleted:
		return fmt.Sprintf("file %s was deleted", filename)
	case FileModified:
		return fmt.Sprintf("file %s was modified", filename)
	}
	return ""
}

// ReactorFn define a reaction function called when an observed file is modified.
type ReactorFn func(file string, action ActionType) error

// ExitOnChangeReactor provides reactor function that causes the process to exit when the change is detected.
// DEPRECATED: Using this function cause process to exit immediately without proper shutdown (context close/etc.)
//             Use the TerminateOnChangeReactor() instead.
var ExitOnChangeReactor = TerminateOnChangeReactor(func() { os.Exit(0) })

func TerminateOnChangeReactor(terminateFn func()) ReactorFn {
	return func(filename string, action ActionType) error {
		klog.Infof("Triggering shutdown because %s", action.String(filename))
		terminateFn()
		return nil
	}
}

func NewObserver(interval time.Duration) (Observer, error) {
	return &pollingObserver{
		interval: interval,
		reactors: map[string][]ReactorFn{},
		files:    map[string]fileHashAndState{},
	}, nil
}

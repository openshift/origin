package fileobserver

import (
	"fmt"
	"os"
	"time"

	"github.com/golang/glog"
)

type Observer interface {
	Run(stopChan <-chan struct{})
	AddReactor(reaction reactorFn, startingFileContent map[string][]byte, files ...string) Observer
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

// reactorFn define a reaction function called when an observed file is modified.
type reactorFn func(file string, action ActionType) error

// ExitOnChangeReactor provides reactor function that causes the process to exit when the change is detected.
var ExitOnChangeReactor reactorFn = func(filename string, action ActionType) error {
	glog.Infof("exiting because %q changed", filename)
	os.Exit(0)
	return nil
}

func NewObserver(interval time.Duration) (Observer, error) {
	return &pollingObserver{
		interval: interval,
		reactors: map[string][]reactorFn{},
		files:    map[string]string{},
	}, nil
}

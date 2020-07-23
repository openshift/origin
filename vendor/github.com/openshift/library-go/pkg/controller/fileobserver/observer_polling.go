package fileobserver

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
	"k8s.io/klog/v2"
)

type pollingObserver struct {
	interval time.Duration
	reactors map[string][]ReactorFn
	files    map[string]fileHashAndState

	reactorsMutex sync.RWMutex

	syncedMutex sync.RWMutex
	hasSynced   bool
}

// HasSynced indicates that the observer synced all observed files at least once.
func (o *pollingObserver) HasSynced() bool {
	o.syncedMutex.RLock()
	defer o.syncedMutex.RUnlock()
	return o.hasSynced
}

// AddReactor will add new reactor to this observer.
func (o *pollingObserver) AddReactor(reaction ReactorFn, startingFileContent map[string][]byte, files ...string) Observer {
	o.reactorsMutex.Lock()
	defer o.reactorsMutex.Unlock()
	for _, f := range files {
		if len(f) == 0 {
			panic(fmt.Sprintf("observed file name must not be empty (%#v)", files))
		}
		// Do not rehash existing files
		if _, exists := o.files[f]; exists {
			continue
		}
		var err error

		if startingContent, ok := startingFileContent[f]; ok {
			klog.V(3).Infof("Starting from specified content for file %q", f)
			// if empty starting content is specified, do not hash the empty string but just return it the same
			// way as calculateFileHash() does in that case.
			// in case the file exists and is empty, we don't care about the initial content anyway, because we
			// are only going to react when the file content change.
			// in case the file does not exists but empty string is specified as initial content, without this
			// the content will be hashed and reaction will trigger as if the content changed.
			if len(startingContent) == 0 {
				var fileExists bool
				if fileExists, err = isFile(f); err != nil {
					panic(fmt.Sprintf("unexpected error while adding reactor for %#v: %v", files, err))
				}
				o.files[f] = fileHashAndState{exists: fileExists}
				o.reactors[f] = append(o.reactors[f], reaction)
				continue
			}
			currentHash, emptyFile, err := calculateHash(bytes.NewBuffer(startingContent))
			if err != nil {
				panic(fmt.Sprintf("unexpected error while adding reactor for %#v: %v", files, err))
			}
			o.files[f] = fileHashAndState{exists: true, hash: currentHash, isEmpty: emptyFile}
		} else {
			klog.V(3).Infof("Adding reactor for file %q", f)
			o.files[f], err = calculateFileHash(f)
			if err != nil && !os.IsNotExist(err) {
				panic(fmt.Sprintf("unexpected error while adding reactor for %#v: %v", files, err))
			}
		}
		o.reactors[f] = append(o.reactors[f], reaction)
	}
	return o
}

func (o *pollingObserver) processReactors(stopCh <-chan struct{}) {
	err := wait.PollImmediateInfinite(o.interval, func() (bool, error) {
		select {
		case <-stopCh:
			return true, nil
		default:
		}
		o.reactorsMutex.RLock()
		defer o.reactorsMutex.RUnlock()
		for filename, reactors := range o.reactors {
			currentFileState, err := calculateFileHash(filename)
			if err != nil && !os.IsNotExist(err) {
				return false, err
			}

			lastKnownFileState := o.files[filename]
			o.files[filename] = currentFileState

			for i := range reactors {
				var action ActionType
				switch {
				case !lastKnownFileState.exists && !currentFileState.exists:
					// skip non-existing file
					continue
				case !lastKnownFileState.exists && currentFileState.exists && (len(currentFileState.hash) > 0 || currentFileState.isEmpty):
					// if we see a new file created that has content or its empty, trigger FileCreate action
					klog.Infof("Observed file %q has been created (hash=%q)", filename, currentFileState.hash)
					action = FileCreated
				case lastKnownFileState.exists && !currentFileState.exists:
					klog.Infof("Observed file %q has been deleted", filename)
					action = FileDeleted
				case lastKnownFileState.hash == currentFileState.hash:
					// skip if the hashes are the same
					continue
				case lastKnownFileState.hash != currentFileState.hash:
					klog.Infof("Observed file %q has been modified (old=%q, new=%q)", filename, lastKnownFileState.hash, currentFileState.hash)
					action = FileModified
				}
				// increment metrics counter for this file
				observerActionsMetrics.WithLabelValues(filename, action.name()).Inc()
				// execute the register reactor
				if err := reactors[i](filename, action); err != nil {
					klog.Errorf("Reactor for %q failed: %v", filename, err)
				}
			}
		}
		if !o.HasSynced() {
			o.syncedMutex.Lock()
			o.hasSynced = true
			o.syncedMutex.Unlock()
			klog.V(3).Info("File observer successfully synced")
		}
		return false, nil
	})
	if err != nil {
		klog.Fatalf("file observer failed: %v", err)
	}
}

var observerActionsMetrics = metrics.NewCounterVec(&metrics.CounterOpts{
	Subsystem:      "fileobserver",
	Name:           "action_count",
	Help:           "Counter for every observed action for all monitored files",
	StabilityLevel: metrics.ALPHA,
}, []string{"name", "filename"})

func init() {
	(&sync.Once{}).Do(func() {
		legacyregistry.MustRegister(observerActionsMetrics)
	})
}

// Run will start a new observer.
func (o *pollingObserver) Run(stopChan <-chan struct{}) {
	klog.Info("Starting file observer")
	defer klog.Infof("Shutting down file observer")
	o.processReactors(stopChan)
}

type fileHashAndState struct {
	hash    string
	exists  bool
	isEmpty bool
}

func calculateFileHash(path string) (fileHashAndState, error) {
	result := fileHashAndState{}
	if exists, err := isFile(path); !exists || err != nil {
		return result, err
	}

	f, err := os.Open(path)
	if err != nil {
		return result, err
	}
	defer f.Close()
	// at this point we know for sure the file exists and we can read its content even if that content is empty
	result.exists = true

	hash, empty, err := calculateHash(f)
	if err != nil {
		return result, err
	}

	result.hash = hash
	result.isEmpty = empty

	return result, nil
}

func calculateHash(content io.Reader) (string, bool, error) {
	hasher := sha256.New()
	written, err := io.Copy(hasher, content)
	if err != nil {
		return "", false, err
	}
	// written == 0 means the content is empty
	if written == 0 {
		return "", true, nil
	}
	return hex.EncodeToString(hasher.Sum(nil)), false, nil
}

func isFile(path string) (bool, error) {
	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	// this is fatal
	if stat.IsDir() {
		return false, fmt.Errorf("%s is a directory", path)
	}

	return true, nil
}

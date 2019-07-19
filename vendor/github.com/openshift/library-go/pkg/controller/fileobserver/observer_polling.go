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
	"k8s.io/klog"
)

type pollingObserver struct {
	interval time.Duration
	reactors map[string][]reactorFn
	files    map[string]string

	reactorsMutex sync.RWMutex
}

// AddReactor will add new reactor to this observer.
func (o *pollingObserver) AddReactor(reaction reactorFn, startingFileContent map[string][]byte, files ...string) Observer {
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
			o.files[f], err = calculateHash(bytes.NewBuffer(startingContent))
			if err != nil {
				panic(fmt.Sprintf("unexpected error while adding reactor for %#v: %v", files, err))
			}
		} else {
			klog.V(3).Infof("Adding reactor for file %q", f)
			o.files[f], err = calculateFileHash(f)
			if err != nil {
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
			currentHash, err := calculateFileHash(filename)
			if err != nil {
				return false, err
			}
			lastKnownHash := o.files[filename]

			// No file change detected
			if lastKnownHash == currentHash {
				continue
			}

			klog.Infof("Observed change: file:%s (current: %q, lastKnown: %q)", filename, currentHash, lastKnownHash)
			o.files[filename] = currentHash

			for i := range reactors {
				action := FileModified
				switch {
				case len(lastKnownHash) == 0:
					action = FileCreated
				case len(currentHash) == 0:
					action = FileDeleted
				case len(lastKnownHash) > 0:
					action = FileModified
				}

				if err := reactors[i](filename, action); err != nil {
					klog.Errorf("Reactor for %q failed: %v", filename, err)
				}
			}
		}
		return false, nil
	})
	if err != nil {
		klog.Fatalf("file observer failed: %v", err)
	}
}

// Run will start a new observer.
func (o *pollingObserver) Run(stopChan <-chan struct{}) {
	klog.Info("Starting file observer")
	defer klog.Infof("Shutting down file observer")
	o.processReactors(stopChan)
}

func calculateFileHash(path string) (string, error) {
	stat, statErr := os.Stat(path)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return "", nil
		}
		return "", statErr
	}
	if stat.IsDir() {
		return "", fmt.Errorf("you can watch only files, %s is a directory", path)
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer f.Close()
	return calculateHash(f)
}

func calculateHash(content io.Reader) (string, error) {
	hasher := sha256.New()
	if _, err := io.Copy(hasher, content); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

package fileobserver

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

type reactionRecorder struct {
	reactions map[string][]ActionType
	sync.RWMutex
}

func newReactionRecorder() *reactionRecorder {
	return &reactionRecorder{reactions: map[string][]ActionType{}}
}

func (r *reactionRecorder) get(f string) []ActionType {
	r.RLock()
	defer r.RUnlock()
	return r.reactions[f]
}

func (r *reactionRecorder) add(f string, action ActionType) {
	r.Lock()
	defer r.Unlock()
	r.reactions[f] = append(r.reactions[f], action)
}

func TestObserverSimple(t *testing.T) {
	dir, err := ioutil.TempDir("", "observer-simple-")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(dir)

	o, err := NewObserver(200 * time.Millisecond)
	if err != nil {
		t.Fatalf("observer: %v", err)
	}

	reactions := newReactionRecorder()

	testReaction := func(f string, action ActionType) error {
		reactions.add(f, action)
		return nil
	}

	testFile := filepath.Join(dir, "test-file-1")

	o.AddReactor(testReaction, nil, testFile)

	stopCh := make(chan struct{})
	defer close(stopCh)
	go o.Run(stopCh)

	fileCreateObserved := make(chan struct{})
	go func() {
		defer close(fileCreateObserved)
		if err := wait.PollImmediateUntil(300*time.Millisecond, func() (bool, error) {
			t.Logf("waiting for reaction ...")
			if len(reactions.get(testFile)) == 0 {
				return false, nil
			}
			if r := reactions.get(testFile)[0]; r != FileCreated {
				return true, fmt.Errorf("expected FileCreated, got: %#v", reactions.get(testFile))
			}
			t.Logf("recv: %#v", reactions.get(testFile))
			return true, nil
		}, stopCh); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}()

	ioutil.WriteFile(testFile, []byte("foo"), os.ModePerm)
	<-fileCreateObserved

	fileModifiedObserved := make(chan struct{})
	go func() {
		defer close(fileModifiedObserved)
		if err := wait.PollImmediateUntil(300*time.Millisecond, func() (bool, error) {
			t.Logf("waiting for reaction ...")
			if len(reactions.get(testFile)) != 2 {
				return false, nil
			}

			if r := reactions.get(testFile)[1]; r != FileModified {
				return true, fmt.Errorf("expected FileModified, got: %#v", reactions.get(testFile))
			}
			t.Logf("recv: %#v", reactions.get(testFile))
			return true, nil
		}, stopCh); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}()

	ioutil.WriteFile(testFile, []byte("bar"), os.ModePerm)
	<-fileModifiedObserved

	fileRemoveObserved := make(chan struct{})
	go func() {
		defer close(fileRemoveObserved)
		if err := wait.PollImmediateUntil(300*time.Millisecond, func() (bool, error) {
			t.Logf("waiting for reaction ...")
			if len(reactions.get(testFile)) != 3 {
				return false, nil
			}
			if r := reactions.get(testFile)[2]; r != FileDeleted {
				return true, fmt.Errorf("expected FileDeleted, got: %#v", reactions.get(testFile))
			}
			t.Logf("recv: %#v", reactions.get(testFile))
			return true, nil
		}, stopCh); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}()

	os.Remove(testFile)
	<-fileRemoveObserved
}

func TestObserverSimpleContentSpecified(t *testing.T) {
	dir, err := ioutil.TempDir("", "observer-simple-")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(dir)

	o, err := NewObserver(200 * time.Millisecond)
	if err != nil {
		t.Fatalf("observer: %v", err)
	}

	reactions := newReactionRecorder()

	testReaction := func(f string, action ActionType) error {
		reactions.add(f, action)
		return nil
	}

	testFile := filepath.Join(dir, "test-file-1")
	ioutil.WriteFile(testFile, []byte("foo"), os.ModePerm)

	o.AddReactor(
		testReaction,
		map[string][]byte{
			testFile: {},
		},
		testFile)

	stopCh := make(chan struct{})
	defer close(stopCh)
	go o.Run(stopCh)

	fileModifyObserved := make(chan struct{})
	go func() {
		defer close(fileModifyObserved)
		if err := wait.PollImmediateUntil(300*time.Millisecond, func() (bool, error) {
			t.Logf("waiting for reaction ...")
			if len(reactions.get(testFile)) == 0 {
				return false, nil
			}
			if r := reactions.get(testFile)[0]; r != FileModified {
				return true, fmt.Errorf("expected FileModified, got: %#v", reactions.get(testFile))
			}
			t.Logf("recv: %#v", reactions.get(testFile))
			return true, nil
		}, stopCh); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}()

	<-fileModifyObserved
	os.Remove(testFile)
}

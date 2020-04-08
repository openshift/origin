package fileobserver

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

func TestObserverPolling(t *testing.T) {
	type observedAction struct {
		file   string
		action ActionType
	}

	var (
		nonEmptyContent = []byte("non-empty")
		changedContent  = []byte("change")
		emptyContent    = []byte("")

		observedSingleFileCreated = func(actions []observedAction, t *testing.T) {
			if len(actions) == 0 {
				t.Errorf("no actions observed, but expected to observe created")
				return
			}
			if actions[0].action != FileCreated {
				t.Errorf("created action expected, but observed %q", actions[0].action.String(path.Base(actions[0].file)))
			}
		}

		observedSingleFileModified = func(actions []observedAction, t *testing.T) {
			if len(actions) == 0 {
				t.Errorf("no actions observed, but expected to observe modified")
				return
			}
			if actions[0].action != FileModified {
				t.Errorf("modified action expected, but observed %q", actions[0].action.String(path.Base(actions[0].file)))
			}
		}

		observedSingleFileDeleted = func(actions []observedAction, t *testing.T) {
			if len(actions) == 0 {
				t.Errorf("no actions observed, but expected to observe deleted")
				return
			}
			if actions[0].action != FileDeleted {
				t.Errorf("deleted action expected, but observed %q", actions[0].action.String(path.Base(actions[0].file)))
			}
		}

		observedNoChanges = func(actions []observedAction, t *testing.T) {
			if len(actions) != 0 {
				var result []string
				for _, a := range actions {
					result = append(result, a.action.String(path.Base(a.file)))
				}
				t.Errorf("expected to not observe any actions, but observed: %s", strings.Join(result, ","))
			}
		}

		defaultTimeout = 5 * time.Second
	)

	tests := []struct {
		name              string
		startFileContent  []byte            // the content the file is created with initially
		changeFileContent []byte            // change the file content
		deleteFile        bool              // change the file by deleting it
		startWithNoFile   bool              // start test with no file
		setInitialContent bool              // set the initial content
		initialContent    map[string][]byte // initial content to pass to observer
		timeout           time.Duration     // maximum test duration (default: 5s)
		waitForObserver   time.Duration     // duration to wait for observer to sync changes (default: 300ms)

		evaluateActions func([]observedAction, *testing.T) // func to evaluate observed actions
	}{
		{
			name:              "start with existing non-empty file with no change and initial content set",
			evaluateActions:   observedNoChanges,
			setInitialContent: true,
			startFileContent:  nonEmptyContent,
			timeout:           1 * time.Second,
		},
		{
			name:             "start with existing non-empty file with no change and no initial content set",
			evaluateActions:  observedNoChanges,
			startFileContent: nonEmptyContent,
			timeout:          1 * time.Second,
		},
		{
			name:              "start with existing non-empty file that change",
			evaluateActions:   observedSingleFileModified,
			setInitialContent: true,
			startFileContent:  nonEmptyContent,
			changeFileContent: changedContent,
		},
		{
			name:              "start with existing non-empty file and no initial content that change",
			evaluateActions:   observedSingleFileModified,
			startFileContent:  nonEmptyContent,
			changeFileContent: changedContent,
		},
		{
			name:              "start with existing empty file with no change",
			evaluateActions:   observedNoChanges,
			setInitialContent: true,
			startFileContent:  emptyContent,
			changeFileContent: emptyContent,
		},
		{
			name:              "start with existing empty file and no initial content with no change",
			evaluateActions:   observedNoChanges,
			startFileContent:  emptyContent,
			changeFileContent: emptyContent,
		},
		{
			name:              "start with existing empty file that change content",
			evaluateActions:   observedSingleFileModified,
			startFileContent:  emptyContent,
			changeFileContent: changedContent,
		},
		{
			name:              "start with existing empty file and empty initial content that change content",
			evaluateActions:   observedSingleFileModified,
			setInitialContent: true,
			startFileContent:  emptyContent,
			changeFileContent: changedContent,
		},
		{
			name:            "start with non-existing file with no change",
			evaluateActions: observedNoChanges,
			startWithNoFile: true,
		},
		{
			// This is what controllercmd.NewCommandWithContext currently does to avoid races
			name:              "start with non-existing file with no change, force no starting hashing",
			setInitialContent: true,
			startFileContent:  emptyContent,
			evaluateActions:   observedNoChanges,
			startWithNoFile:   true,
		},
		{
			name:              "start with non-existing file that is created as empty file",
			evaluateActions:   observedSingleFileCreated,
			startWithNoFile:   true,
			changeFileContent: emptyContent,
		},
		{
			name:              "start with non-existing file that is created as non-empty file",
			evaluateActions:   observedSingleFileCreated,
			startWithNoFile:   true,
			changeFileContent: nonEmptyContent,
		},
		{
			name:              "start with existing file with content that is deleted",
			evaluateActions:   observedSingleFileDeleted,
			setInitialContent: true,
			startFileContent:  nonEmptyContent,
			deleteFile:        true,
		},
		{
			name:             "start with existing file with content and not initial content set that is deleted",
			evaluateActions:  observedSingleFileDeleted,
			startFileContent: nonEmptyContent,
			deleteFile:       true,
		},
	}

	baseDir, err := ioutil.TempDir("", "observer-poll-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(baseDir)

	for _, test := range tests {
		if test.timeout == 0 {
			test.timeout = defaultTimeout
		}
		t.Run(test.name, func(t *testing.T) {
			observer, err := NewObserver(200 * time.Millisecond)
			if err != nil {
				t.Fatal(err)
			}

			testDir := filepath.Join(baseDir, t.Name())
			if err := os.MkdirAll(filepath.Join(baseDir, t.Name()), 0777); err != nil {
				t.Fatal(err)
			}

			testFile := filepath.Join(testDir, "testfile")

			if test.setInitialContent {
				test.initialContent = map[string][]byte{
					testFile: test.startFileContent,
				}
			}

			if !test.startWithNoFile {
				if err := ioutil.WriteFile(testFile, test.startFileContent, os.ModePerm); err != nil {
					t.Fatal(err)
				}
				t.Logf("created file %q with content: %q", testFile, string(test.startFileContent))
			}

			observedChan := make(chan observedAction)
			observer.AddReactor(func(file string, action ActionType) error {
				t.Logf("observed %q", action.String(path.Base(file)))
				observedChan <- observedAction{
					file:   file,
					action: action,
				}
				return nil
			}, test.initialContent, testFile)

			stopChan := make(chan struct{})

			// start observing actions
			observedActions := []observedAction{}
			var observedActionsMutex sync.Mutex
			stopObservingChan := make(chan struct{})
			go func() {
				for {
					select {
					case action := <-observedChan:
						observedActionsMutex.Lock()
						observedActions = append(observedActions, action)
						observedActionsMutex.Unlock()
					case <-stopObservingChan:
						return
					}
				}
			}()

			// start file observer
			go observer.Run(stopChan)

			// wait until file observer see the files at least once
			if err := wait.PollImmediate(10*time.Millisecond, test.timeout, func() (done bool, err error) {
				return observer.HasSynced(), nil
			}); err != nil {
				t.Errorf("failed to wait for observer to sync: %v", err)
			}
			t.Logf("starting observing changes ...")

			if test.changeFileContent != nil {
				t.Logf("writing %q ...", string(test.changeFileContent))
				if err := ioutil.WriteFile(testFile, test.changeFileContent, os.ModePerm); err != nil {
					t.Fatal(err)
				}
			}

			if test.deleteFile {
				if err := os.RemoveAll(testDir); err != nil {
					t.Fatal(err)
				}
			}

			// give observer time to observe latest events
			if test.waitForObserver == 0 {
				time.Sleep(400 * time.Millisecond)
			} else {
				time.Sleep(test.waitForObserver)
			}

			close(stopObservingChan)
			close(stopChan)

			observedActionsMutex.Lock()
			defer observedActionsMutex.Unlock()
			test.evaluateActions(observedActions, t) // evaluate observed actions
		})
	}
}

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
			testFile: []byte("bar"),
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

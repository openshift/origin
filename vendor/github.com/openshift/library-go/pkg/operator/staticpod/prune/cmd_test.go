package prune

import (
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"sort"
	"testing"

	"vbom.ml/util/sortorder"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name     string
		o        PruneOptions
		files    []string
		expected []string
	}{
		{
			name: "only deletes non-protected revisions of the specified pod",
			o: PruneOptions{
				MaxEligibleRevision: 3,
				ProtectedRevisions:  []int{3, 2},
				StaticPodName:       "test",
			},
			files:    []string{"test-1", "test-2", "test-3", "othertest-4"},
			expected: []string{"test-2", "test-3", "othertest-4"},
		},
		{
			name: "doesn't delete anything higher than highest eligible revision",
			o: PruneOptions{
				MaxEligibleRevision: 2,
				ProtectedRevisions:  []int{2},
				StaticPodName:       "test",
			},
			files:    []string{"test-1", "test-2", "test-3"},
			expected: []string{"test-2", "test-3"},
		},
		{
			name: "revision numbers do not conflict between pods when detecting protected IDs",
			o: PruneOptions{
				MaxEligibleRevision: 2,
				ProtectedRevisions:  []int{2},
				StaticPodName:       "test",
			},
			files:    []string{"test-1", "test-2", "othertest-1", "othertest-2"},
			expected: []string{"test-2", "othertest-1", "othertest-2"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testDir, err := ioutil.TempDir("", "prune-revisions-test")
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				os.Remove(testDir)
			}()

			resourceDir := path.Join(testDir, "resources")
			err = os.Mkdir(resourceDir, os.ModePerm)
			if err != nil {
				t.Error(err)
			}
			for _, file := range test.files {
				err = os.Mkdir(path.Join(resourceDir, file), os.ModePerm)
				if err != nil {
					t.Error(err)
				}
			}

			o := test.o
			o.ResourceDir = resourceDir

			err = o.Run()
			if err != nil {
				t.Error(err)
			}
			checkPruned(t, o.ResourceDir, test.expected)
		})
	}
}

func checkPruned(t *testing.T, resourceDir string, expected []string) {
	files, err := ioutil.ReadDir(resourceDir)
	if err != nil {
		t.Error(err)
	}
	actual := make([]string, 0, len(files))
	for _, file := range files {
		actual = append(actual, file.Name())
	}

	sort.Sort(sortorder.Natural(expected))
	sort.Sort(sortorder.Natural(actual))

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %+v, got %+v", expected, actual)
	}
}

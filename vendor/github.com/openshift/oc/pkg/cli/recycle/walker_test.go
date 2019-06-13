package recycle

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestStatUID(t *testing.T) {
	root, err := ioutil.TempDir("", "walker-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(root); err != nil {
			t.Fatal(err)
		}
	}()

	rootName := filepath.Base(root)

	files := map[string]testFile{
		filepath.Join(root):                  {uid: 0, dir: true},
		filepath.Join(root, "dir1"):          {uid: 1, dir: true},
		filepath.Join(root, "dir2"):          {uid: 2, dir: true},
		filepath.Join(root, "dir2/subdir"):   {uid: 2, dir: true},
		filepath.Join(root, "dir2/subfile1"): {uid: 123},
		filepath.Join(root, "dir2/subfile2"): {uid: 234},
		filepath.Join(root, "file1"):         {uid: 345},
		filepath.Join(root, "file2"):         {uid: 456},
	}
	for path, fileinfo := range files {
		if fileinfo.dir {
			if err := os.MkdirAll(path, os.FileMode(0755)); err != nil {
				t.Fatalf("Error writing dir %s\n%v", path, err)
				continue
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(path), os.FileMode(0755)); err != nil {
				t.Fatalf("Error writing dir %s\n%v", path, err)
				continue
			}
			if err := ioutil.WriteFile(path, []byte(path), os.FileMode(0755)); err != nil {
				t.Fatalf("Error writing file %s\n%v", path, err)
			}
		}
	}

	expectedActions := []testAction{
		// get root info, list files
		// no setfsuid call, because we are already uid 0
		{"lstat", rootName},
		{"readDirNames", rootName},

		// get dir1 info, become owner, list files, change back to root owner, walk dir1 (as owner of root dir, so we can rmdir dir1)
		{"lstat", "dir1"},
		{"setfsuid", 1},
		{"readDirNames", "dir1"},
		{"setfsuid", 0},
		{"walk", "dir1"},

		// get dir2 info, become owner, list files
		{"lstat", "dir2"},
		{"setfsuid", 2},
		{"readDirNames", "dir2"},
		// get subdir info, list files, walk
		// no setfsuid calls to subdir owner or back to dir2 owner, because we are already uid 2
		{"lstat", "subdir"},
		{"readDirNames", "subdir"},
		{"walk", "subdir"},
		// stat and walk subfiles, no fsuid changes needed for files
		{"lstat", "subfile1"},
		{"walk", "subfile1"},
		{"lstat", "subfile2"},
		{"walk", "subfile2"},
		// change back to root owner, walk dir2 (as owner of root dir, so we can rmdir dir2)
		{"setfsuid", 0},
		{"walk", "dir2"},

		// stat and walk files, no fsuid changes needed for files
		{"lstat", "file1"},
		{"walk", "file1"},
		{"lstat", "file2"},
		{"walk", "file2"},

		// finally, walk the root
		{"walk", rootName},
	}
	actions := []testAction{}

	w := &walker{
		fsuid: 0, // mock starting as uid 0
		walkFn: func(path string, info os.FileInfo) error {
			actions = append(actions, testAction{"walk", filepath.Base(path)})
			return nil
		},
		lstat: func(path string) (os.FileInfo, error) {
			actions = append(actions, testAction{"lstat", filepath.Base(path)})
			l, err := os.Lstat(path)
			return testFileInfoWrapper{l, files[path]}, err
		},
		getuid: func(info os.FileInfo) (int64, error) {
			return info.(testFileInfoWrapper).testData.uid, nil
		},
		setfsuid: func(uid int) error {
			actions = append(actions, testAction{"setfsuid", uid})
			return nil
		},
		readDirNames: func(path string) ([]string, error) {
			actions = append(actions, testAction{"readDirNames", filepath.Base(path)})
			return readDirNames(path)
		},
	}

	err = w.Walk(root)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	for i, action := range actions {
		if len(expectedActions) < i+1 {
			t.Errorf("%d unexpected actions: %+v", len(actions)-len(expectedActions), actions[i:])
			break
		}

		expectedAction := expectedActions[i]
		if !reflect.DeepEqual(expectedAction, action) {
			t.Errorf("%d: expected %#v\ngot                      %#v", i, expectedAction, action)
			continue
		}
	}

	if len(expectedActions) > len(actions) {
		t.Errorf("%d additional expected actions:%+v", len(expectedActions)-len(actions), expectedActions[len(actions):])
	}

}

type testFile struct {
	uid int64
	dir bool
}

type testAction struct {
	Action string
	Data   interface{}
}

type testFileInfoWrapper struct {
	os.FileInfo
	testData testFile
}

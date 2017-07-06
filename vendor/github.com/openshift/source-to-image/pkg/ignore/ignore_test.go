package ignore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/util"
)

func baseTest(t *testing.T, patterns []string, filesToDel []string, filesToKeep []string) {
	// create working dir
	workingDir, werr := util.NewFileSystem().CreateWorkingDirectory()
	if werr != nil {
		t.Errorf("problem allocating working dir: %v", werr)
	} else {
		t.Logf("working directory is %q", workingDir)
	}
	defer func() {
		// clean up test
		cleanerr := os.RemoveAll(workingDir)
		if cleanerr != nil {
			t.Errorf("problem cleaning up: %v", cleanerr)
		}
	}()

	c := &api.Config{WorkingDir: workingDir}

	// create source repo dir for .s2iignore that matches where ignore.go looks
	dpath := filepath.Join(c.WorkingDir, "upload", "src")
	derr := os.MkdirAll(dpath, 0777)
	if derr != nil {
		t.Errorf("Problem creating source repo dir %q: %v", dpath, derr)
	}

	c.WorkingSourceDir = dpath
	t.Logf("working source dir %q", dpath)

	// create s2iignore file
	ipath := filepath.Join(dpath, api.IgnoreFile)
	ifile, ierr := os.Create(ipath)
	defer ifile.Close()
	if ierr != nil {
		t.Errorf("Problem creating .s2iignore in %q: %v", ipath, ierr)
	}

	// write patterns to remove into s2ignore, but save ! exclusions
	filesToIgnore := make(map[string]string)
	for _, pattern := range patterns {
		t.Logf("storing pattern %q", pattern)
		_, serr := ifile.WriteString(pattern)

		if serr != nil {
			t.Errorf("Problem setting .s2iignore: %v", serr)
		}
		if strings.HasPrefix(pattern, "!") {
			pattern = strings.Replace(pattern, "!", "", 1)
			t.Logf("Noting ignore pattern %q", pattern)
			filesToIgnore[pattern] = pattern
		}
	}

	// create slices the store files to create, maps for files which should be deleted, files which should be kept
	filesToCreate := []string{}
	filesToDelCheck := make(map[string]string)
	for _, fileToDel := range filesToDel {
		filesToDelCheck[fileToDel] = fileToDel
		filesToCreate = append(filesToCreate, fileToDel)
	}
	filesToKeepCheck := make(map[string]string)
	for _, fileToKeep := range filesToKeep {
		filesToKeepCheck[fileToKeep] = fileToKeep
		filesToCreate = append(filesToCreate, fileToKeep)
	}

	// create files for test
	for _, fileToCreate := range filesToCreate {
		fbpath := filepath.Join(dpath, fileToCreate)

		// ensure any subdirs off working dir exist
		dirpath := filepath.Dir(fbpath)
		derr := os.MkdirAll(dirpath, 0777)
		if derr != nil && !os.IsExist(derr) {
			t.Errorf("Problem creating subdirs %q: %v", dirpath, derr)
		}
		t.Logf("Going to create file %q", fbpath)
		fbfile, fberr := os.Create(fbpath)
		if fberr != nil {
			t.Errorf("Problem creating test file: %v", fberr)
		}
		fbfile.Close()
	}

	// run ignorer algorithm
	ignorer := &DockerIgnorer{}
	ignorer.Ignore(c)

	// check if filesToDel, minus ignores, are gone, and filesToKeep are still there
	for _, fileToCheck := range filesToCreate {
		fbpath := filepath.Join(dpath, fileToCheck)
		t.Logf("Evaluating file %q from dir %q and file to check %q", fbpath, dpath, fileToCheck)

		// see if file still exists or not
		ofile, oerr := os.Open(fbpath)
		defer ofile.Close()
		var fileExists bool
		if oerr == nil {
			fileExists = true
			t.Logf("The file %q exists after Ignore was run", fbpath)
		} else {
			if os.IsNotExist(oerr) {
				t.Logf("The file %q does not exist after Ignore was run", fbpath)
				fileExists = false
			} else {
				t.Errorf("Could not verify existence of %q: %v", fbpath, oerr)
			}
		}

		_, iok := filesToIgnore[fileToCheck]
		_, kok := filesToKeepCheck[fileToCheck]
		_, dok := filesToDelCheck[fileToCheck]

		// if file present, verify it is in ignore or keep list, and not in del list
		if fileExists {
			if iok {
				t.Logf("validated ignored file is still present: %q", fileToCheck)
				continue
			}
			if kok {
				t.Logf("validated file to keep is still present: %q", fileToCheck)
				continue
			}
			if dok {
				t.Errorf("file which was cited to be deleted by caller to runTest exists: %q", fileToCheck)
				continue
			}
			// if here, something unexpected
			t.Errorf("file %q not in ignore / keep / del list !?!?!?!?", fileToCheck)
		} else {
			if dok {
				t.Logf("file which should have been deleted is in fact gone: %q", fileToCheck)
				continue
			}
			if iok {
				t.Errorf("file put into ignore list does not exist: %q", fileToCheck)
				continue
			}
			if kok {
				t.Errorf("file passed in with keep list does not exist: %q", fileToCheck)
				continue
			}
			// if here, then something unexpected happened
			t.Errorf("file %q not in ignore / keep / del list !?!?!?!?", fileToCheck)
		}
	}
}

func TestBlankLine(t *testing.T) {
	baseTest(t, []string{"foo.bar\n", "\n", "bar.baz\n"}, []string{"foo.bar", "bar.baz"}, []string{"foo.baz"})
}

func TestSingleIgnore(t *testing.T) {
	baseTest(t, []string{"foo.bar\n"}, []string{"foo.bar"}, []string{})
}

func TestWildcardIgnore(t *testing.T) {
	baseTest(t, []string{"foo.*\n"}, []string{"foo.a", "foo.b"}, []string{})
}

func TestExclusion(t *testing.T) {
	baseTest(t, []string{"foo.*\n", "!foo.a"}, []string{"foo.b"}, []string{"foo.a"})
}

func TestSubDirs(t *testing.T) {
	baseTest(t, []string{"*/foo.a\n"}, []string{"asdf/foo.a"}, []string{"foo.a"})
}

func TestBasicDelKeepMix(t *testing.T) {
	baseTest(t, []string{"foo.bar\n"}, []string{"foo.bar"}, []string{"bar.foo"})
}

/*
Per the docker user guide, with a docker ignore list of:

    LICENSE.*
    !LICENSE.md
    *.md

LICENSE.MD will NOT be kept, as *.md overrides !LICENSE.md
*/
func TestExcludeOverride(t *testing.T) {
	baseTest(t, []string{"LICENSE.*\n", "!LICENSE.md\n", "*.md"}, []string{"LICENSE.foo", "LICENSE.md"}, []string{"foo.bar"})
}

func TestExclusionWithWildcard(t *testing.T) {
	baseTest(t, []string{"*.bar\n", "!foo.*"}, []string{"boo.bar", "bar.bar"}, []string{"foo.bar"})
}

func TestHopelessExclusion(t *testing.T) {
	baseTest(t, []string{"!LICENSE.md\n", "LICENSE.*"}, []string{"LICENSE.md"}, []string{})
}

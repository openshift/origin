package dockerclient

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/docker/docker/pkg/archive"
)

func Test_archiveFromFile(t *testing.T) {
	f, err := ioutil.TempFile("", "test-tar")
	if err != nil {
		t.Fatal(err)
	}
	rc, err := archive.TarWithOptions("testdata/dir", &archive.TarOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(f, rc); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	testArchive := f.Name()
	testCases := []struct {
		file     string
		src      string
		dst      string
		excludes []string
		expect   []string
	}{
		{
			file: testArchive,
			src:  "/*",
			dst:  "test",
			expect: []string{
				"test/Dockerfile",
				"test/file",
				"test/subdir",
				"test/subdir/file2",
			},
		},
		{
			file: testArchive,
			src:  ".",
			dst:  "test",
			expect: []string{
				"test/Dockerfile",
				"test/file",
				"test/subdir",
				"test/subdir/file2",
			},
		},
		{
			file: testArchive,
			src:  "fil?",
			dst:  "test",
			expect: []string{
				"test/file",
			},
		},
		{
			file: testArchive,
			src:  "fil?",
			dst:  "",
			expect: []string{
				"file",
			},
		},
		{
			file: testArchive,
			src:  "subdir",
			dst:  "",
			expect: []string{
				"file2",
			},
		},
		{
			file: testArchive,
			src:  "subdir/",
			dst:  "",
			expect: []string{
				"file2",
			},
		},
		{
			file: testArchive,
			src:  "subdir/",
			dst:  "test/",
			expect: []string{
				"test",
				"test/file2",
			},
		},
		{
			file: testArchive,
			src:  "subdir/file?",
			dst:  "test/",
			expect: []string{
				"test/file2",
			},
		},
		{
			file: testArchive,
			src:  "subdi?",
			dst:  "test",
			expect: []string{
				"test/subdir",
				"test/subdir/file2",
			},
		},
		{
			file: testArchive,
			src:  "subdi?",
			dst:  "test/",
			expect: []string{
				"test/subdir",
				"test/subdir/file2",
			},
		},
		{
			file:     testArchive,
			src:      "subdi?",
			dst:      "test/",
			excludes: []string{"**/file*"},
			expect: []string{
				"test/subdir",
			},
		},
		{
			file:     testArchive,
			src:      ".",
			dst:      "",
			excludes: []string{"unknown"},
			expect: []string{
				"Dockerfile",
				"file",
				"subdir",
				"subdir/file2",
			},
		},
		{
			file:     testArchive,
			src:      ".",
			dst:      "",
			excludes: []string{"subdir"},
			expect: []string{
				"Dockerfile",
				"file",
			},
		},
		{
			file:     testArchive,
			src:      ".",
			dst:      "",
			excludes: []string{"file"},
			expect: []string{
				"Dockerfile",
				"subdir",
				"subdir/file2",
			},
		},
		{
			file:     testArchive,
			src:      ".",
			dst:      "",
			excludes: []string{"*/file2"},
			expect: []string{
				"Dockerfile",
				"file",
				"subdir",
			},
		},
	}
	for i := range testCases {
		testCase := testCases[i]
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			r, c, err := archiveFromFile(
				testCase.file,
				testCase.src,
				testCase.dst,
				testCase.excludes,
			)
			if err != nil {
				t.Fatal(err)
			}
			tr := tar.NewReader(r)
			var found []string
			for {
				h, err := tr.Next()
				if err != nil {
					if err == io.EOF {
						break
					}
					t.Fatal(err)
				}
				found = append(found, h.Name)
			}
			c.Close()
			sort.Strings(found)
			if !reflect.DeepEqual(testCase.expect, found) {
				t.Errorf("unexpected files:\n%v\n%v", testCase.expect, found)
			}
		})
	}
}

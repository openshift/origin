package dockerclient

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/docker/docker/pkg/archive"
	"github.com/pkg/errors"
)

type testDirectoryCheck map[string]bool

func (c testDirectoryCheck) IsDirectory(path string) (bool, error) {
	if c == nil {
		return false, nil
	}

	isDir, ok := c[path]
	if !ok {
		return false, fmt.Errorf("no path defined for %s", path)
	}
	return isDir, nil
}

type archiveGenerator struct {
	Headers []*tar.Header
}

func newArchiveGenerator() *archiveGenerator {
	return &archiveGenerator{}
}

func (g *archiveGenerator) File(name string) *archiveGenerator {
	g.Headers = append(g.Headers, &tar.Header{Name: name, Size: 1})
	return g
}

func (g *archiveGenerator) Dir(name string) *archiveGenerator {
	g.Headers = append(g.Headers, &tar.Header{Name: name, Typeflag: tar.TypeDir})
	return g
}

func (g *archiveGenerator) Reader() io.Reader {
	pr, pw := io.Pipe()
	go func() {
		err := func() error {
			w := tar.NewWriter(pw)
			for _, h := range g.Headers {
				if err := w.WriteHeader(h); err != nil {
					return err
				}
				if h.Typeflag&tar.TypeDir == tar.TypeDir {
					continue
				}
				for i := int64(0); i < h.Size; i++ {
					if _, err := w.Write([]byte{byte(i)}); err != nil {
						return err
					}
				}
			}
			return w.Close()
		}()
		pw.CloseWithError(err)
	}()
	return pr
}

// errors.Cause() plus unwrapping go1.13-style wrapped errors
func unwrapError(err error) error {
	type unwrapper interface {
		Unwrap() error
	}
	if errors.Cause(err) == nil {
		return nil
	}
	unwrappable, ok := errors.Cause(err).(unwrapper)
	for ok && err != nil {
		err = errors.Cause(unwrappable.Unwrap())
		unwrappable, ok = err.(unwrapper)
	}
	return err
}

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
		gen      *archiveGenerator
		src      string
		closeErr error
		dst      string
		excludes []string
		expect   []string
		check    map[string]bool
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
		{
			file:     testArchive,
			src:      "subdir/no-such-file",
			closeErr: os.ErrNotExist,
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
				testDirectoryCheck(testCase.check),
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
			closeErr := c.Close()
			if unwrapError(testCase.closeErr) != unwrapError(closeErr) {
				t.Fatalf("expected error %q, got %q", unwrapError(testCase.closeErr), unwrapError(closeErr))
			}
			sort.Strings(found)
			if !reflect.DeepEqual(testCase.expect, found) {
				t.Errorf("unexpected files:\n%v\n%v", testCase.expect, found)
			}
		})
	}
}

func Test_archiveFromContainer(t *testing.T) {
	testCases := []struct {
		gen      *archiveGenerator
		src      string
		closeErr error
		dst      string
		excludes []string
		expect   []string
		path     string
		check    map[string]bool
	}{
		{
			gen:  newArchiveGenerator().File("file").Dir("test").File("test/file2"),
			src:  "/*",
			dst:  "test",
			path: "/",
			expect: []string{
				"test/file",
				"test/test",
				"test/test/file2",
			},
		},
		{
			gen:  newArchiveGenerator().File("file").Dir("test").File("test/file2"),
			src:  "/",
			dst:  "test",
			path: "/",
			expect: []string{
				"test/file",
				"test/test",
				"test/test/file2",
			},
		},
		{
			gen:  newArchiveGenerator().File("file").Dir("test").File("test/file2"),
			src:  ".",
			dst:  "test",
			path: ".",
			expect: []string{
				"test/file",
				"test/test",
				"test/test/file2",
			},
		},
		{
			gen:  newArchiveGenerator().File("file").Dir("test").File("test/file2"),
			src:  ".",
			dst:  "test/",
			path: ".",
			expect: []string{
				"test/file",
				"test/test",
				"test/test/file2",
			},
		},
		{
			gen:  newArchiveGenerator().File("file").Dir("test").File("test/file2"),
			src:  ".",
			dst:  "/test",
			path: ".",
			expect: []string{
				"/test/file",
				"/test/test",
				"/test/test/file2",
			},
		},
		{
			gen:  newArchiveGenerator().File("file").Dir("test").File("test/file2"),
			src:  ".",
			dst:  "/test/",
			path: ".",
			expect: []string{
				"/test/file",
				"/test/test",
				"/test/test/file2",
			},
		},
		{
			gen:  newArchiveGenerator().File("b/file").Dir("b/test").File("b/test/file2"),
			src:  "/a/b/",
			dst:  "/b",
			path: "/a/b",
			expect: []string{
				"/b/file",
				"/b/test",
				"/b/test/file2",
			},
		},
		{
			gen:  newArchiveGenerator().File("/b/file").Dir("/b/test").File("/b/test/file2"),
			src:  "/a/b/*",
			dst:  "/b",
			path: "/a/b",
			expect: []string{
				"/b/file",
				"/b/test",
				"/b/test/file2",
			},
		},

		// DownloadFromContainer returns tar archive paths prefixed with a slash when
		// the base directory is the root
		{
			gen:  newArchiveGenerator().File("/a").Dir("/b").File("/b/1"),
			src:  "/a",
			dst:  "/",
			path: "/",
			expect: []string{
				"/a",
			},
		},
		{
			gen:  newArchiveGenerator().File("/a").Dir("/b").File("/b/1"),
			src:  "/a",
			dst:  "/a",
			path: "/",
			expect: []string{
				"/a",
			},
		},
		{
			gen:  newArchiveGenerator().Dir("b/").File("b/1").File("b/2"),
			src:  "/a/b/",
			dst:  "/b/",
			path: "/a/b",
			expect: []string{
				"/b",
				"/b/1",
				"/b/2",
			},
		},
		{
			gen:      newArchiveGenerator().Dir("").File("b"),
			src:      "/a/b",
			closeErr: os.ErrNotExist,
			dst:      "/a",
			path:     "/a",
			expect:   nil,
		},
		{
			gen:      newArchiveGenerator().File("b"),
			src:      "/a/b",
			closeErr: os.ErrNotExist,
			dst:      "/a",
			check:    map[string]bool{"/a": true},
			path:     "/a",
			expect:   nil,
		},
		{
			gen:  newArchiveGenerator().Dir("a/").File("a/b"),
			src:  "/a/b",
			dst:  "/a",
			path: "/a",
			expect: []string{
				"/a",
			},
		},
		{
			gen:    newArchiveGenerator().Dir("./a").File("./a/b"),
			src:    "a",
			dst:    "/a",
			path:   ".",
			expect: []string{"/a/b"},
		},
		{
			gen:      newArchiveGenerator().Dir("/a").File("/a/b"),
			src:      "/a/c",
			path:     "/a",
			closeErr: os.ErrNotExist,
		},
		{
			gen:      newArchiveGenerator().Dir("/a").File("/a/b"),
			src:      "/a/c*",
			path:     "/a",
			closeErr: os.ErrNotExist,
		},
	}
	for i := range testCases {
		testCase := testCases[i]
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			rc, path, err := archiveFromContainer(
				testCase.gen.Reader(),
				testCase.src,
				testCase.dst,
				testCase.excludes,
				testDirectoryCheck(testCase.check),
			)
			if err != nil {
				t.Fatal(err)
			}
			if filepath.Clean(path) != testCase.path {
				t.Errorf("unexpected path: %s != %s", filepath.Clean(path), testCase.path)
			}
			tr := tar.NewReader(rc)
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
			closeErr := rc.Close()
			if unwrapError(testCase.closeErr) != unwrapError(closeErr) {
				t.Fatalf("expected error %q, got %q", unwrapError(testCase.closeErr), unwrapError(closeErr))
			}
			sort.Strings(found)
			if !reflect.DeepEqual(testCase.expect, found) {
				t.Errorf("unexpected files:\n%v\n%v", testCase.expect, found)
			}
		})
	}
}

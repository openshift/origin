package dockerclient

import (
	"fmt"
	"reflect"
	"testing"
)

func TestCalcCopyInfo(t *testing.T) {
	nilErr := func(err error) bool { return err == nil }
	tests := []struct {
		origPath       string
		rootPath       string
		dstPath        string
		allowWildcards bool
		errFn          func(err error) bool
		paths          map[string]struct{}
		excludes       []string
		rebaseNames    map[string]string
	}{
		{
			origPath:       "subdir/*",
			rootPath:       "testdata/dir",
			allowWildcards: true,
			errFn:          nilErr,
			paths:          map[string]struct{}{"subdir/file2": {}},
		},
		{
			origPath:       "*",
			rootPath:       "testdata/dir",
			allowWildcards: true,
			errFn:          nilErr,
			paths: map[string]struct{}{
				"Dockerfile": {},
				"file":       {},
				"subdir":     {},
			},
		},
		{
			origPath:       ".",
			rootPath:       "testdata/dir",
			allowWildcards: true,
			errFn:          nilErr,
			paths: map[string]struct{}{
				"Dockerfile": {},
				"file":       {},
				"subdir":     {},
			},
		},
		{
			origPath:       "/.",
			rootPath:       "testdata/dir",
			allowWildcards: true,
			errFn:          nilErr,
			paths: map[string]struct{}{
				"Dockerfile": {},
				"file":       {},
				"subdir":     {},
			},
		},
		{
			origPath:       "subdir/",
			rootPath:       "testdata/dir",
			allowWildcards: true,
			errFn:          nilErr,
			paths: map[string]struct{}{
				"subdir/": {},
			},
		},
		{
			origPath:       "subdir",
			rootPath:       "testdata/dir",
			allowWildcards: true,
			errFn:          nilErr,
			paths: map[string]struct{}{
				"subdir": {},
			},
		},
		{
			origPath:       ".",
			dstPath:        "copy",
			rootPath:       "testdata/dir",
			allowWildcards: true,
			errFn:          nilErr,
			paths: map[string]struct{}{
				"file":       {},
				"Dockerfile": {},
				"subdir":     {},
			},
			rebaseNames: map[string]string{
				"file":       "copy/file",
				"Dockerfile": "copy/Dockerfile",
				"subdir":     "copy/subdir",
			},
		},
		{
			origPath:       ".",
			dstPath:        "copy",
			rootPath:       "testdata/singlefile",
			allowWildcards: true,
			errFn:          nilErr,
			paths: map[string]struct{}{
				"Dockerfile": {},
			},
			rebaseNames: map[string]string{
				"Dockerfile": "copy/Dockerfile",
			},
		},
		{
			origPath:       "existing/",
			dstPath:        ".",
			rootPath:       "testdata/overlapdir",
			allowWildcards: true,
			errFn:          nilErr,
			paths: map[string]struct{}{
				"existing/": {},
			},
			rebaseNames: map[string]string{
				"existing": ".",
			},
		},
		{
			origPath:       "existing",
			dstPath:        ".",
			rootPath:       "testdata/overlapdir",
			allowWildcards: true,
			errFn:          nilErr,
			paths: map[string]struct{}{
				"existing": {},
			},
			rebaseNames: map[string]string{
				"existing": ".",
			},
		},
		{
			origPath:       "existing",
			dstPath:        "/",
			rootPath:       "testdata/overlapdir",
			allowWildcards: true,
			errFn:          nilErr,
			paths: map[string]struct{}{
				"existing": {},
			},
			rebaseNames: map[string]string{
				"existing": "/",
			},
		},
		{
			origPath:       "subdir/.",
			rootPath:       "testdata/dir",
			allowWildcards: true,
			errFn:          nilErr,
			paths: map[string]struct{}{
				"subdir/": {},
			},
		},
		{
			origPath:       "testdata/dir/subdir/.",
			rootPath:       "",
			allowWildcards: true,
			errFn:          nilErr,
			paths: map[string]struct{}{
				"testdata/dir/subdir/": {},
			},
		},
		{
			origPath:       "subdir/",
			rootPath:       "testdata/dir",
			allowWildcards: true,
			errFn:          nilErr,
			paths: map[string]struct{}{
				"subdir/": {},
			},
		},
		{
			origPath:       "subdir/",
			rootPath:       "testdata/dir",
			allowWildcards: true,
			errFn:          nilErr,
			paths: map[string]struct{}{
				"subdir/": {},
			},
			dstPath: "test/",
			rebaseNames: map[string]string{
				"subdir": "test",
			},
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			infos, err := CalcCopyInfo(test.origPath, test.rootPath, test.allowWildcards)
			if !test.errFn(err) {
				t.Fatalf("unexpected error: %v", err)
			}
			if err != nil {
				return
			}
			expect := make(map[string]struct{})
			for k := range test.paths {
				expect[k] = struct{}{}
			}
			for _, info := range infos {
				if _, ok := expect[info.Path]; ok {
					delete(expect, info.Path)
				} else {
					t.Errorf("did not expect path %s", info.Path)
				}
			}
			if len(expect) > 0 {
				t.Errorf("did not see paths: %#v", expect)
			}

			options := archiveOptionsFor(infos, test.dstPath, test.excludes)
			if !reflect.DeepEqual(test.rebaseNames, options.RebaseNames) {
				t.Errorf("rebase names did not match:\n%#v\n%#v", test.rebaseNames, options.RebaseNames)
			}
		})
	}
}

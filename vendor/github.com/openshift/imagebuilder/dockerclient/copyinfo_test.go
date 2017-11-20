package dockerclient

import (
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
				"subdir/": "test/",
			},
		},
	}

	for i, test := range tests {
		infos, err := CalcCopyInfo(test.origPath, test.rootPath, false, test.allowWildcards)
		if !test.errFn(err) {
			t.Errorf("%d: unexpected error: %v", i, err)
			continue
		}
		if err != nil {
			continue
		}
		expect := make(map[string]struct{})
		for k := range test.paths {
			expect[k] = struct{}{}
		}
		for _, info := range infos {
			if _, ok := expect[info.Path]; ok {
				delete(expect, info.Path)
			} else {
				t.Errorf("%d: did not expect path %s", i, info.Path)
			}
		}
		if len(expect) > 0 {
			t.Errorf("%d: did not see paths: %#v", i, expect)
		}

		options := archiveOptionsFor(infos, test.dstPath, test.excludes)
		if !reflect.DeepEqual(test.rebaseNames, options.RebaseNames) {
			t.Errorf("%d: rebase names did not match: %#v", i, options.RebaseNames)
		}
	}
}

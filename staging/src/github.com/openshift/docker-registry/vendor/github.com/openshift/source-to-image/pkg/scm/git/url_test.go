package git

import (
	"net/url"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

type parseTest struct {
	rawurl         string
	expectedGitURL *URL
	expectedError  bool
}

func TestParse(t *testing.T) {
	var tests []parseTest

	switch runtime.GOOS {
	case "windows":
		tests = append(tests,
			parseTest{
				rawurl:        "file://relative/path",
				expectedError: true,
			},
			parseTest{
				rawurl:        "file:///relative/path",
				expectedError: true,
			},
			parseTest{
				rawurl: "file:///c:/absolute/path?query#fragment",
				expectedGitURL: &URL{
					URL: url.URL{
						Scheme:   "file",
						Path:     "/c:/absolute/path",
						RawQuery: "query",
						Fragment: "fragment",
					},
					Type: URLTypeURL,
				},
			},
		)

	default:
		tests = append(tests,
			parseTest{
				rawurl:        "file://relative/path",
				expectedError: true,
			},
			parseTest{
				rawurl: "file:///absolute/path?query#fragment",
				expectedGitURL: &URL{
					URL: url.URL{
						Scheme:   "file",
						Path:     "/absolute/path",
						RawQuery: "query",
						Fragment: "fragment",
					},
					Type: URLTypeURL,
				},
			},
		)
	}

	tests = append(tests,
		// http://
		parseTest{
			rawurl: "http://user:pass@github.com:443/user/repo.git?query#fragment",
			expectedGitURL: &URL{
				URL: url.URL{
					Scheme:   "http",
					User:     url.UserPassword("user", "pass"),
					Host:     "github.com:443",
					Path:     "/user/repo.git",
					RawQuery: "query",
					Fragment: "fragment",
				},
				Type: URLTypeURL,
			},
		},
		parseTest{
			rawurl: "http://user@1.2.3.4:443/repo?query#fragment",
			expectedGitURL: &URL{
				URL: url.URL{
					Scheme:   "http",
					User:     url.User("user"),
					Host:     "1.2.3.4:443",
					Path:     "/repo",
					RawQuery: "query",
					Fragment: "fragment",
				},
				Type: URLTypeURL,
			},
		},
		parseTest{
			rawurl: "http://[::ffff:1.2.3.4]:443",
			expectedGitURL: &URL{
				URL: url.URL{
					Scheme: "http",
					Host:   "[::ffff:1.2.3.4]:443",
				},
				Type: URLTypeURL,
			},
		},
		parseTest{
			rawurl: "http://github.com/openshift/origin",
			expectedGitURL: &URL{
				URL: url.URL{
					Scheme: "http",
					Host:   "github.com",
					Path:   "/openshift/origin",
				},
				Type: URLTypeURL,
			},
		},

		// transport::opaque
		parseTest{
			rawurl: "http::http://github.com/openshift/origin",
			expectedGitURL: &URL{
				URL: url.URL{
					Scheme: "http",
					Opaque: ":http://github.com/openshift/origin",
				},
				Type: URLTypeURL,
			},
		},

		// git@host ...
		parseTest{
			rawurl: "user@github.com:/user/repo.git#fragment",
			expectedGitURL: &URL{
				URL: url.URL{
					User:     url.User("user"),
					Host:     "github.com",
					Path:     "/user/repo.git",
					Fragment: "fragment",
				},
				Type: URLTypeSCP,
			},
		},
		parseTest{
			rawurl: "user@github.com:user/repo.git#fragment",
			expectedGitURL: &URL{
				URL: url.URL{
					User:     url.User("user"),
					Host:     "github.com",
					Path:     "user/repo.git",
					Fragment: "fragment",
				},
				Type: URLTypeSCP,
			},
		},
		parseTest{
			rawurl: "user@1.2.3.4:repo#fragment",
			expectedGitURL: &URL{
				URL: url.URL{
					User:     url.User("user"),
					Host:     "1.2.3.4",
					Path:     "repo",
					Fragment: "fragment",
				},
				Type: URLTypeSCP,
			},
		},
		parseTest{
			rawurl: "[::ffff:1.2.3.4]:",
			expectedGitURL: &URL{
				URL: url.URL{
					Host: "[::ffff:1.2.3.4]",
				},
				Type: URLTypeSCP,
			},
		},
		parseTest{
			rawurl: "git@github.com:openshift/origin",
			expectedGitURL: &URL{
				URL: url.URL{
					User: url.User("git"),
					Host: "github.com",
					Path: "openshift/origin",
				},
				Type: URLTypeSCP,
			},
		},

		// path ...
		parseTest{
			rawurl: "/absolute#fragment",
			expectedGitURL: &URL{
				URL: url.URL{
					Path:     "/absolute",
					Fragment: "fragment",
				},
				Type: URLTypeLocal,
			},
		},
		parseTest{
			rawurl: "relative#fragment",
			expectedGitURL: &URL{
				URL: url.URL{
					Path:     "relative",
					Fragment: "fragment",
				},
				Type: URLTypeLocal,
			},
		},
	)

	for _, test := range tests {
		url, err := Parse(test.rawurl)
		if test.expectedError != (err != nil) {
			t.Errorf("%s: Parse() returned err: %v", test.rawurl, err)
		}
		if err != nil {
			continue
		}

		if !reflect.DeepEqual(url, test.expectedGitURL) {
			t.Errorf("%s: Parse() returned\n\t%#v\nWanted\n\t%#v", test.rawurl, url, test.expectedGitURL)
		}

		if url.String() != test.rawurl {
			t.Errorf("%s: String() returned %s", test.rawurl, url.String())
		}

		if url.StringNoFragment() != strings.SplitN(test.rawurl, "#", 2)[0] {
			t.Errorf("%s: StringNoFragment() returned %s", test.rawurl, url.StringNoFragment())
		}
	}
}

func TestStringNoFragment(t *testing.T) {
	u := MustParse("part#fragment")
	if u.StringNoFragment() != "part" {
		t.Errorf("StringNoFragment() returned %s", u.StringNoFragment())
	}
	if !reflect.DeepEqual(u, MustParse("part#fragment")) {
		t.Errorf("StringNoFragment() modified its argument")
	}
}

type localPathTest struct {
	url      *URL
	expected string
}

func TestLocalPath(t *testing.T) {
	var tests []localPathTest

	switch runtime.GOOS {
	case "windows":
		tests = append(tests,
			localPathTest{
				url:      MustParse("file:///c:/foo/bar"),
				expected: `c:\foo\bar`,
			},
			localPathTest{
				url:      MustParse(`c:\foo\bar`),
				expected: `c:\foo\bar`,
			},
			localPathTest{
				url:      MustParse(`\foo\bar`),
				expected: `\foo\bar`,
			},
			localPathTest{
				url:      MustParse(`foo\bar`),
				expected: `foo\bar`,
			},
			localPathTest{
				url:      MustParse(`foo`),
				expected: `foo`,
			},
		)

	default:
		tests = append(tests,
			localPathTest{
				url:      MustParse("file:///foo/bar"),
				expected: "/foo/bar",
			},
			localPathTest{
				url:      MustParse("/foo/bar"),
				expected: "/foo/bar",
			},
		)
	}

	for i, test := range tests {
		if test.url.LocalPath() != test.expected {
			t.Errorf("%d: LocalPath() returned %s", i, test.url.LocalPath())
		}
	}
}

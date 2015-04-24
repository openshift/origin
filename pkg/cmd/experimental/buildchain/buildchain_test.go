package buildchain

import (
	"reflect"
	"sort"
	"testing"

	buildapi "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func TestFindRepoDeps(t *testing.T) {
	tests := []struct {
		name                 string
		repo                 string
		tag                  string
		all                  bool
		candidates           []buildapi.BuildConfig
		expectedTreeSize     int
		expectedRootChildren int
		expectedRootEdges    int
		expectedErr          error
	}{
		{
			name:                 "docker-image-references-test",
			repo:                 "default/start",
			tag:                  imageapi.DefaultImageTag,
			all:                  false,
			candidates:           dockerImageReferencesList(),
			expectedTreeSize:     6,
			expectedRootChildren: 2,
			expectedRootEdges:    2,
			expectedErr:          nil,
		},
		{
			name:                 "single-namespace-test",
			repo:                 "default/start",
			tag:                  "other",
			all:                  true,
			candidates:           singleNamespaceList(),
			expectedTreeSize:     5,
			expectedRootChildren: 2,
			expectedRootEdges:    2,
			expectedErr:          nil,
		},
		{
			name:                 "multiple-namespaces-test",
			repo:                 "test/test-repo",
			tag:                  "atag",
			all:                  true,
			candidates:           multipleNamespacesList(),
			expectedTreeSize:     3,
			expectedRootChildren: 2,
			expectedRootEdges:    2,
			expectedErr:          nil,
		},
	}

	for _, test := range tests {
		root, err := findRepoDeps(test.repo, test.tag, test.all, test.candidates)
		if err != test.expectedErr {
			t.Errorf("%s: Invalid error: Expected %v, got %v", test.name, test.expectedErr, err)
		}

		gotTreeSize := treeSize(root)
		if test.expectedTreeSize != gotTreeSize {
			t.Errorf("%s: Invalid tree size: Expected %d, got %d", test.name, test.expectedTreeSize, gotTreeSize)
		}

		rootChildren := len(root.Children)
		if test.expectedRootChildren != rootChildren {
			t.Errorf("%s: Invalid root(%s) children amount: Expected %d, got %d", test.name, test.repo, test.expectedRootChildren, rootChildren)
		}

		rootEdges := len(root.Edges)
		if test.expectedRootEdges != rootEdges {
			t.Errorf("%s: Invalid root(%s) edges amount: Expected %d, got %d", test.name, test.repo, test.expectedRootEdges, rootEdges)
		}
	}
}

func TestGetRepos(t *testing.T) {
	tests := []struct {
		name       string
		configList []buildapi.BuildConfig
		expected   map[string][]string
	}{
		{
			name:       "1st getRepo test",
			configList: dockerImageReferencesList(),
			expected: map[string][]string{
				"default/another-repo": {"outputtag"},
				"default/start":        {imageapi.DefaultImageTag},
				"default/test-repo":    {"atag"},
			},
		},
		{
			name:       "2nd getRepo test",
			configList: singleNamespaceList(),
			expected: map[string][]string{
				"default/another-repo": {"outputtag"},
				"default/start":        {imageapi.DefaultImageTag, "tip", "other"},
				"default/test-repo":    {"atag", "release", imageapi.DefaultImageTag},
			},
		},
		{
			name:       "3rd getRepo test",
			configList: []buildapi.BuildConfig{},
			expected:   map[string][]string{},
		},
	}

	for _, test := range tests {
		repos := getRepos(test.configList)
		for repo, tags := range repos {
			expectedTags, ok := test.expected[repo]
			if !ok {
				t.Errorf("%s: Image repository not found: %s", test.name, repo)
			}

			sort.Strings(expectedTags)
			sort.Strings(tags)
			if !reflect.DeepEqual(expectedTags, tags) {
				t.Errorf("invalid tags: Expected %v, got %v", expectedTags, tags)
			}
		}
	}
}

func TestParseTag(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedRest string
		expectedTag  string
		expectedErr  error
	}{
		{
			name:         "1st parseTag test",
			input:        "centos",
			expectedRest: "centos",
			expectedTag:  imageapi.DefaultImageTag,
			expectedErr:  nil,
		},
		{
			name:         "2nd parseTag test",
			input:        "os/centos:14",
			expectedRest: "os/centos",
			expectedTag:  "14",
			expectedErr:  nil,
		},
		{
			name:         "3rd parseTag test",
			input:        "other/centos:07",
			expectedRest: "other/centos",
			expectedTag:  "07",
			expectedErr:  nil,
		},
		{
			name:         "4th parseTag test",
			input:        "test:for:error",
			expectedRest: "",
			expectedTag:  "",
			expectedErr:  invalidRepoTagErr,
		},
	}

	for _, test := range tests {
		rest, tag, err := parseTag(test.input)
		if tag != test.expectedTag {
			t.Errorf("%s: invalid tag, expected %s, got %s", test.name, test.expectedTag, tag)
		}
		if rest != test.expectedRest {
			t.Errorf("%s: invalid rest of input, expected %s, got %s", test.name, test.expectedRest, rest)
		}
		if err != test.expectedErr {
			t.Errorf("%s: invalid error, expected %v, got %v", test.name, test.expectedErr, err)
		}
	}
}

func TestJoin(t *testing.T) {
	tests := []struct {
		name           string
		namespace      string
		repo           string
		expectedOutput string
	}{
		{
			name:           "1st join test",
			namespace:      "default",
			repo:           "centos",
			expectedOutput: "default/centos",
		},
		{
			name:           "2nd join test",
			namespace:      "testing",
			repo:           "playground",
			expectedOutput: "testing/playground",
		},
		{
			name:           "3rd join test",
			namespace:      "other",
			repo:           "another",
			expectedOutput: "other/another",
		},
	}

	for _, test := range tests {
		fullName := join(test.namespace, test.repo)
		if fullName != test.expectedOutput {
			t.Errorf("%s: invalid output, expected %s, got %s", test.name, test.expectedOutput, fullName)
		}
	}
}

func TestSplit(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		expectedNamespace string
		expectedName      string
		expectedErr       error
	}{
		{
			name:              "1st split test",
			input:             "default/centos",
			expectedNamespace: "default",
			expectedName:      "centos",
			expectedErr:       nil,
		},
		{
			name:              "2nd split test",
			input:             "testing/playground",
			expectedNamespace: "testing",
			expectedName:      "playground",
			expectedErr:       nil,
		},
		{
			name:              "3rd split test",
			input:             "other/another/yay",
			expectedNamespace: "",
			expectedName:      "",
			expectedErr:       invalidRepoErr,
		},
	}

	for _, test := range tests {
		namespace, name, err := split(test.input)
		if namespace != test.expectedNamespace {
			t.Errorf("%s: invalid namespace, expected %s, got %s", test.name, test.expectedNamespace, namespace)
		}
		if name != test.expectedName {
			t.Errorf("%s: invalid name, expected %s, got %s", test.name, test.expectedName, name)
		}
		if err != test.expectedErr {
			t.Errorf("%s: invalid error, expected %v, got %v", test.name, test.expectedErr, err)
		}
	}
}

func TestValidDOT(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedOutput string
	}{
		{
			name:           "1st validDOT test",
			input:          "default/centos",
			expectedOutput: "default/centos",
		},
		{
			name:           "2nd validDOT test",
			input:          "playground",
			expectedOutput: "playground",
		},
		{
			name:           "3rd validDOT test",
			input:          "THE-OTHER-WAY_AROUND",
			expectedOutput: "THE_OTHER_WAY_AROUND",
		},
	}

	for _, test := range tests {
		validated := validDOT(test.input)
		if validated != test.expectedOutput {
			t.Errorf("%s: invalid DOT output, expected %s, got %s", test.name, test.expectedOutput, validated)
		}
	}
}

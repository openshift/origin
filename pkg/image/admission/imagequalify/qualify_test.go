package imagequalify_test

import (
	"bytes"
	"testing"

	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/image/admission/imagequalify"
	"github.com/openshift/origin/pkg/image/admission/imagequalify/api"
)

type testcase struct {
	image    string
	expected string
}

func parseQualifyRules(rules []api.ImageQualifyRule) (*api.ImageQualifyConfig, error) {
	config, err := configapilatest.WriteYAML(&api.ImageQualifyConfig{
		Rules: rules,
	})

	if err != nil {
		return nil, err
	}

	return imagequalify.ReadConfig(bytes.NewReader(config))
}

func testQualify(t *testing.T, rules []api.ImageQualifyRule, tests []testcase) {
	t.Helper()

	config, err := parseQualifyRules(rules)
	if err != nil {
		t.Fatalf("failed to parse rules: %v", err)
	}

	for i, tc := range tests {
		name, err := imagequalify.QualifyImage(tc.image, config.Rules)
		if err != nil {
			t.Fatalf("test #%v: unexpected error: %s", i, err)
		}
		if tc.expected != name {
			t.Errorf("test #%v: expected %q, got %q", i, tc.expected, name)
		}
	}
}

func TestQualifyNoRules(t *testing.T) {
	rules := []api.ImageQualifyRule{}

	tests := []testcase{{
		image:    "busybox",
		expected: "busybox",
	}, {
		image:    "repo/busybox",
		expected: "repo/busybox",
	}}

	testQualify(t, rules, tests)
}

func TestQualifyImageNoMatch(t *testing.T) {
	rules := []api.ImageQualifyRule{{
		Pattern: "busybox",
		Domain:  "production.io",
	}, {
		Pattern: "busybox:v1*",
		Domain:  "v1.io",
	}, {
		Pattern: "busybox:*",
		Domain:  "next.io",
	}}

	tests := []testcase{{
		image:    "nginx",
		expected: "nginx",
	}, {
		image:    "nginx:latest",
		expected: "nginx:latest",
	}, {
		image:    "repo/nginx",
		expected: "repo/nginx",
	}, {
		image:    "repo/nginx:latest",
		expected: "repo/nginx:latest",
	}}

	testQualify(t, rules, tests)
}

func TestQualifyRepoAndImageAndTagsWithWildcard(t *testing.T) {
	rules := []api.ImageQualifyRule{{
		Pattern: "repo/busybox",
		Domain:  "production.io",
	}, {
		Pattern: "repo/busybox:v1*",
		Domain:  "v1.io",
	}, {
		Pattern: "repo/busybox:*",
		Domain:  "next.io",
	}}

	tests := []testcase{{
		image:    "busybox",
		expected: "busybox",
	}, {
		image:    "busybox:latest",
		expected: "busybox:latest",
	}, {
		image:    "repo/busybox",
		expected: "production.io/repo/busybox",
	}, {
		image:    "repo/busybox:v1.2.3",
		expected: "v1.io/repo/busybox:v1.2.3",
	}, {
		image:    "repo/busybox:latest",
		expected: "next.io/repo/busybox:latest",
	}}

	testQualify(t, rules, tests)
}

func TestQualifyNoRepoWithImageWildcard(t *testing.T) {
	rules := []api.ImageQualifyRule{{
		Pattern: "*",
		Domain:  "default.io",
	}}

	tests := []testcase{{
		image:    "nginx",
		expected: "default.io/nginx",
	}, {
		image:    "repo/nginx",
		expected: "repo/nginx",
	}}

	testQualify(t, rules, tests)
}

func TestQualifyRepoAndImageWildcard(t *testing.T) {
	rules := []api.ImageQualifyRule{{
		Pattern: "*/*",
		Domain:  "repo.io",
	}, {
		Pattern: "*",
		Domain:  "default.io",
	}}

	tests := []testcase{{
		image:    "nginx",
		expected: "default.io/nginx",
	}, {
		image:    "repo/nginx",
		expected: "repo.io/repo/nginx",
	}}

	testQualify(t, rules, tests)
}

func TestQualifyWildcards(t *testing.T) {
	rules := []api.ImageQualifyRule{{
		Pattern: "*/*:*",
		Domain:  "first.io",
	}, {
		Pattern: "*/*",
		Domain:  "second.io",
	}, {
		Pattern: "*",
		Domain:  "third.io",
	}}

	tests := []testcase{{
		image:    "busybox",
		expected: "third.io/busybox",
	}, {
		image:    "busybox:latest",
		expected: "third.io/busybox:latest",
	}, {
		image:    "nginx",
		expected: "third.io/nginx",
	}, {
		image:    "repo/busybox:latest",
		expected: "first.io/repo/busybox:latest",
	}, {
		image:    "repo/busybox",
		expected: "second.io/repo/busybox",
	}, {
		image:    "repo/nginx",
		expected: "second.io/repo/nginx",
	}, {
		image:    "nginx",
		expected: "third.io/nginx",
	}}

	testQualify(t, rules, tests)
}

func TestQualifyRepoWithWildcards(t *testing.T) {
	rules := []api.ImageQualifyRule{{
		Pattern: "*/*:*",
		Domain:  "first.io",
	}, {
		Pattern: "*/*",
		Domain:  "second.io",
	}, {
		Pattern: "*",
		Domain:  "third.io",
	}, {
		Pattern: "a*/*",
		Domain:  "a.io",
	}, {
		Pattern: "b*/*",
		Domain:  "b.io",
	}, {
		Pattern: "a*/*:*",
		Domain:  "a-with-tag.io",
	}, {
		Pattern: "b*/*:*",
		Domain:  "b-with-tag.io",
	}}

	tests := []testcase{{
		image:    "abc/nginx",
		expected: "a.io/abc/nginx",
	}, {
		image:    "bcd/nginx",
		expected: "b.io/bcd/nginx",
	}, {
		image:    "nginx",
		expected: "third.io/nginx",
	}, {
		image:    "repo/nginx",
		expected: "second.io/repo/nginx",
	}, {
		image:    "repo/nginx:latest",
		expected: "first.io/repo/nginx:latest",
	}, {
		image:    "abc/nginx:1.0",
		expected: "a-with-tag.io/abc/nginx:1.0",
	}, {
		image:    "bcd/nginx:1.0",
		expected: "b-with-tag.io/bcd/nginx:1.0",
	}}

	testQualify(t, rules, tests)
}

func TestQualifyTagsWithWildcards(t *testing.T) {
	rules := []api.ImageQualifyRule{{
		Pattern: "a*/*:*v*",
		Domain:  "v3.io",
	}, {
		Pattern: "a*/*:*v2*",
		Domain:  "v2.io",
	}, {
		Pattern: "a*/*:*v1*",
		Domain:  "v1.io",
	}}

	tests := []testcase{{
		image:    "abc/nginx",
		expected: "abc/nginx",
	}, {
		image:    "bcd/nginx",
		expected: "bcd/nginx",
	}, {
		image:    "abc/nginx:v1.0",
		expected: "v1.io/abc/nginx:v1.0",
	}, {
		image:    "abc/nginx:v2.0",
		expected: "v2.io/abc/nginx:v2.0",
	}, {
		image:    "abc/nginx:v0",
		expected: "v3.io/abc/nginx:v0",
	}, {
		image:    "abc/nginx:latest",
		expected: "abc/nginx:latest",
	}}

	testQualify(t, rules, tests)
}

func TestQualifyImagesAlreadyQualified(t *testing.T) {
	rules := []api.ImageQualifyRule{{
		Pattern: "foo",
		Domain:  "foo.com",
	}}

	tests := []testcase{{
		image:    "foo.com/foo",
		expected: "foo.com/foo",
	}}

	testQualify(t, rules, tests)
}

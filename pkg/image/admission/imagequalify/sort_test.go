package imagequalify_test

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/image/admission/imagequalify"
	"github.com/openshift/origin/pkg/image/admission/imagequalify/api"
)

func patternsFromRules(rules []api.ImageQualifyRule) string {
	var bb bytes.Buffer

	for i := range rules {
		bb.WriteString(rules[i].Pattern)
		bb.WriteString(" ")
	}

	return strings.TrimSpace(bb.String())
}

func parseTestSortPatterns(input string) (*api.ImageQualifyConfig, error) {
	rules := []api.ImageQualifyRule{}

	for i, word := range strings.Fields(input) {
		rules = append(rules, api.ImageQualifyRule{
			Pattern: word,
			Domain:  fmt.Sprintf("domain%v.com", i),
		})
	}

	serializedConfig, serializationErr := configapilatest.WriteYAML(&api.ImageQualifyConfig{
		Rules: rules,
	})

	if serializationErr != nil {
		return nil, serializationErr
	}

	return imagequalify.ReadConfig(bytes.NewReader(serializedConfig))
}

func TestSort(t *testing.T) {
	var testcases = []struct {
		description string
		input       string
		expected    string
	}{{
		description: "default order is ascending",
		input:       "a b c",
		expected:    "c b a",
	}, {
		description: "explicit patterns come before wildcard patterns",
		input:       "a b c *b *a *c",
		expected:    "c b a *c *b *a",
	}, {
		description: "tags are ordered first",
		input:       "a:latest b:latest a b",
		expected:    "b:latest a:latest b a",
	}, {
		description: "tags that are wildcards come last",
		input:       "a:* b:* a b a:latest b:latest",
		expected:    "b:latest a:latest b a b:* a:*",
	}, {
		description: "digests that have wildcards come last",
		input:       "a b@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff b@sha256:* b a@sha256:* a@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		expected:    "b@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff a@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff b a b@sha256:* a@sha256:*",
	}, {
		description: "longer patterns sort first",
		input:       "a a/b a/b/c b b/c b/c/d/e b/c/d",
		expected:    "b/c/d/e b/c/d a/b/c b/c a/b b a",
	}, {
		description: "longer patterns with tags appear before other tags",
		input:       "a a:latest a/b:latest a/b a/b/c a/b/c:latest",
		expected:    "a/b/c:latest a/b/c a/b:latest a/b a:latest a",
	}, {
		description: "longer patterns with a digest appear before others with a digest",
		input:       "a a:latest@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff a/b:latest@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff a/b a/b/c a/b/c:latest@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		expected:    "a/b/c:latest@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff a/b/c a/b:latest@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff a/b a:latest@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff a",
	}, {
		description: "tags ordering is most explicit to least specific",
		input:       "busybox:* busybox:v1.2.3* a busybox:v1.2* b busybox:v1.2.3 busybox busybox:v1* c nginx busybox:v1 busybox:v1.2",
		expected:    "busybox:v1.2.3 busybox:v1.2 busybox:v1 nginx c busybox b a busybox:v1.2.3* busybox:v1.2* busybox:v1* busybox:*",
	}, {
		description: "wildcards with tags list after explicit patterns at the same depth",
		input:       "* */* */*/* *:latest */*:latest */*/*:latest a/b/c:latest",
		expected:    "a/b/c:latest */*/*:latest */*/* */*:latest */* *:latest *",
	}, {
		description: "longer wildcards with tags and digest list first",
		input:       "* */*/*:latest */* */*/* *:latest */*:latest@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		expected:    "*/*/*:latest */*/* */*:latest@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff */* *:latest *",
	}, {
		description: "pathalogical wildcards",
		input:       "* */* */*/*/* */*/*",
		expected:    "*/*/*/* */*/* */* *",
	}}

	for i, tc := range testcases {
		config, err := parseTestSortPatterns(tc.input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		actualPatterns := patternsFromRules(config.Rules)

		if !reflect.DeepEqual(tc.expected, actualPatterns) {
			t.Errorf("test #%v: %s: expected [%s], got [%s]", i, tc.description, tc.expected, actualPatterns)
		}
	}

}

package util

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

func checkSortExpectation(data, suffixOrder []string) error {
	if len(data) != len(suffixOrder) {
		return fmt.Errorf("sorted data length %d did not match expected length %d", len(data), len(suffixOrder))
	}

	for idx, suffix := range suffixOrder {
		if !strings.HasSuffix(data[idx], suffix) {
			return fmt.Errorf("sorted data %s at index %d did not match expectation %s", data[idx], idx, suffix)
		}
	}

	return nil
}

func shuffle(data []string) []string {
	shuffled := make([]string, len(data))
	copy(shuffled, data)

	for idx := range shuffled {
		k := rand.Intn(idx + 1)
		shuffled[idx], shuffled[k] = shuffled[k], shuffled[idx]
	}

	return shuffled
}

func TestSortMapPaths(t *testing.T) {
	tests := []struct {
		name        string
		paths       []string
		pattern     string
		expectation []string
	}{
		{
			name: "simple",
			paths: []string{
				`a\.test\.org a:a`,
				`d\.test\.org d:d`,
				`f\.test\.org f:f`,
				`c\.test\.org c:c`,
				`e\.test\.org e:e`,
				`b\.test\.org b:b`,
			},
			pattern:     "",
			expectation: []string{"f:f", "e:e", "d:d", "c:c", "b:b", "a:a"},
		},
		{
			name: "simple with pattern",
			paths: []string{
				`a\.test\.org a:a`,
				`d\.test\.org d:d`,
				`zzz\.zzz.test\.org zzz:zzz`,
				`f\.test\.org f:f`,
				`zzz\.test\.org zzz:too`,
				`c\.test\.org c:c`,
				`e\.test\.org e:e`,
				`b\.test\.org b:b`,
			},
			pattern:     `zzz`,
			expectation: []string{"f:f", "e:e", "d:d", "c:c", "b:b", "a:a", "zzz:zzz", "zzz:too"},
		},
		{
			name: "simple with template used pattern",
			paths: []string{
				`a\.test\.org a:a`,
				`d\.test\.org d:d`,
				`f\.test\.org f:f`,
				`^[^\.]*\.test\.org aces:wild`,
				`c\.test\.org c:c`,
				`e\.test\.org e:e`,
				`b\.test\.org b:b`,
			},
			pattern:     `^[^\.]*\.`,
			expectation: []string{"f:f", "e:e", "d:d", "c:c", "b:b", "a:a", "aces:wild"},
		},
		{
			name: "simple with multiple template used patterns",
			paths: []string{
				`a\.test\.org a:a`,
				`d\.test\.org d:d`,
				`f\.test\.org f:f`,
				`^[^\.]*\.test\.org aces:wild`,
				`c\.test\.org c:c`,
				`^[^\.]*\.trey\.test\.org wild:threes`,
				`^[^\.]*\.tens\.test\.org wild:tens`,
				`e\.test\.org e:e`,
				`^[^\.]*\.deuces\.test\.org deuces:wild`,
				`b\.test\.org b:b`,
			},
			pattern:     `^[^\.]*\.`,
			expectation: []string{"f:f", "e:e", "d:d", "c:c", "b:b", "a:a", "wild:threes", "aces:wild", "wild:tens", "deuces:wild"},
		},
		{
			name: "data without wildcards",
			paths: []string{
				`^reencrypt\.header\.test(:[0-9]+)?(/.*)?$ be_secure:default:reencrypt`,
				`^annotated\.reencrypt\.header\.test(:[0-9]+)?(/.*)?$ be_secure:default:annotated-reencrypt`,
				`^3hello\.127\.0\.0\.1\.nip\.io(:[0-9]+)?(/.*)?$ be_edge_http:zzztest:hello03`,
				`^www\.example2\.com(:[0-9]+)?(/.*)?$ be_edge_http:default:example-route`,
				`^redirect\.blueprints\.org(:[0-9]+)?(/.*)?$ be_edge_http:blueprints:blueprint-redirect`,
				`^annotated\.blueprints\.org(:[0-9]+)?(/.*)?$ be_edge_http:blueprints:blueprint-annotated`,
				`^something\.edge\.header\.test(:[0-9]+)?(/.*)?$ be_edge_http:default:wildcard-redirect-to-https`,
				`^allow-http\.header\.test(:[0-9]+)?(/.*)?$ be_edge_http:default:allow`,
				`^edge2\.header\.test(:[0-9]+)?(/.*)?$ be_edge_http:default:edge2`,
				`^annotated-ok\.header\.test(:[0-9]+)?(/.*)?$ be_edge_http:default:annotated-ok`,
				`^annotated\.header\.test(:[0-9]+)?(/.*)?$ be_edge_http:default:annotated`,
				`^reencrypt\.blueprints\.org(:[0-9]+)?(/.*)?$ be_secure:blueprints:blueprint-reencrypt`,
				`^edge1\.header\.test(:[0-9]+)?(/.*)?$ be_edge_http:default:edge1`,
			},
			pattern: `^[^\.]*\.`,
			expectation: []string{
				"be_edge_http:default:example-route",
				"be_edge_http:default:wildcard-redirect-to-https",
				"be_secure:default:reencrypt",
				"be_secure:blueprints:blueprint-reencrypt",
				"be_edge_http:blueprints:blueprint-redirect",
				"be_edge_http:default:edge2",
				"be_edge_http:default:edge1",
				"be_secure:default:annotated-reencrypt",
				"be_edge_http:default:annotated",
				"be_edge_http:blueprints:blueprint-annotated",
				"be_edge_http:default:annotated-ok",
				"be_edge_http:default:allow",
				"be_edge_http:zzztest:hello03",
			},
		},
		{
			name: "data with wildcards",
			paths: []string{
				`^reencrypt\.header\.test(:[0-9]+)?(/.*)?$ be_secure:default:reencrypt`,
				`^[^\.]*\.bar\.wildcard\.test(:[0-9]+)?(/.*)?$ be_edge_http:bar:wildcard-test`,
				`^annotated\.reencrypt\.header\.test(:[0-9]+)?(/.*)?$ be_secure:default:annotated-reencrypt`,
				`^3hello\.127\.0\.0\.1\.nip\.io(:[0-9]+)?(/.*)?$ be_edge_http:zzztest:hello03`,
				`^www\.example2\.com(:[0-9]+)?(/.*)?$ be_edge_http:default:example-route`,
				`^[^\.]*\.wild\.io(:[0-9]+)?/a/bc/def(/.*)?$ be_edge_http:default:wildcard-path-route`,
				`^redirect\.blueprints\.org(:[0-9]+)?(/.*)?$ be_edge_http:blueprints:blueprint-redirect`,
				`^annotated\.blueprints\.org(:[0-9]+)?(/.*)?$ be_edge_http:blueprints:blueprint-annotated`,
				`^something\.edge\.header\.test(:[0-9]+)?(/.*)?$ be_edge_http:default:wildcard-redirect-to-https`,
				`^allow-http\.header\.test(:[0-9]+)?(/.*)?$ be_edge_http:default:allow`,
				`^[^\.]*\.foo\.wildcard\.test(:[0-9]+)?(/.*)?$ be_edge_http:default:foo-wildcard-test`,
				`^edge2\.header\.test(:[0-9]+)?(/.*)?$ be_edge_http:default:edge2`,
				`^annotated-ok\.header\.test(:[0-9]+)?(/.*)?$ be_edge_http:default:annotated-ok`,
				`^[^\.]*\.wildcard\.test(:[0-9]+)?(/.*)?$ be_edge_http:default:wildcard-test`,
				`^annotated\.header\.test(:[0-9]+)?(/.*)?$ be_edge_http:default:annotated`,
				`^reencrypt\.blueprints\.org(:[0-9]+)?(/.*)?$ be_secure:blueprints:blueprint-reencrypt`,
				`^edge1\.header\.test(:[0-9]+)?(/.*)?$ be_edge_http:default:edge1`,
			},
			pattern: `^[^\.]*\.`,
			expectation: []string{
				"be_edge_http:default:example-route",
				"be_edge_http:default:wildcard-redirect-to-https",
				"be_secure:default:reencrypt",
				"be_secure:blueprints:blueprint-reencrypt",
				"be_edge_http:blueprints:blueprint-redirect",
				"be_edge_http:default:edge2",
				"be_edge_http:default:edge1",
				"be_secure:default:annotated-reencrypt",
				"be_edge_http:default:annotated",
				"be_edge_http:blueprints:blueprint-annotated",
				"be_edge_http:default:annotated-ok",
				"be_edge_http:default:allow",
				"be_edge_http:zzztest:hello03",
				"be_edge_http:default:wildcard-test",
				"be_edge_http:default:wildcard-path-route",
				"be_edge_http:default:foo-wildcard-test",
				"be_edge_http:bar:wildcard-test",
			},
		},
	}

	for _, tc := range tests {
		for i := 0; i < 25; i++ {
			data := shuffle(tc.paths)
			SortMapPaths(data, tc.pattern)

			err := checkSortExpectation(data, tc.expectation)
			if err != nil {
				t.Errorf("%s: data check expectation failed: %v", tc.name, err)
			}
		}
	}
}

func TestSortMapPathWithWildcards(t *testing.T) {
	testData := []string{
		`^api-stg\.127\.0\.0\.1\.nip\.io(:[0-9]+)?(/.*)?$ stg:api-route`,
		`^api-prod\.127\.0\.0\.1\.nip\.io(:[0-9]+)?(/.*)?$ prod:api-route`,
		`^[^\.]*\.127\.0\.0\.1\.nip\.io(:[0-9]+)?(/.*)?$ prod:wildcard-route`,
		`^3dev\.127\.0\.0\.1\.nip\.io(:[0-9]+)?(/.*)?$ dev:api-route`,
		`^api-prod\.127\.0\.0\.1\.nip\.io(:[0-9]+)?/x/y/z(/.*)?$ prod:api-path-route`,
		`^3app-admin\.127\.0\.0\.1\.nip\.io(:[0-9]+)?(/.*)?$ dev:admin-route`,
		`^[^\.]*\.foo\.127\.0\.0\.1\.nip\.io(:[0-9]+)?(/.*)?$ devel2:foo-wildcard-route`,
		`^zzz-production\.wildcard\.test(:[0-9]+)?/x/y/z(/.*)?$ test:api-route`,
		`^backend-app\.127\.0\.0\.1\.nip\.io(:[0-9]+)?(/.*)?$ prod:backend-route`,
		`^[^\.]*\.foo\.wildcard\.test(:[0-9]+)?(/.*)?$ devel2:foo-wildcard-test`,
	}

	expectedBackendOrder := []string{
		"test:api-route",
		"prod:backend-route",
		"stg:api-route",
		"prod:api-path-route",
		"prod:api-route",
		"dev:api-route",
		"dev:admin-route",
		"devel2:foo-wildcard-test",
		"devel2:foo-wildcard-route",
		"prod:wildcard-route",
	}

	data := SortMapPaths(testData, `^[^\.]*`)

	err := checkSortExpectation(data, expectedBackendOrder)
	if err != nil {
		t.Errorf("TestSortMapPathWithWildcards sort expectation failed: %v", err)
	}
}

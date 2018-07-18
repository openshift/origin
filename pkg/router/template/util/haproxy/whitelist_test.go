package haproxy

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	utilrand "k8s.io/apimachinery/pkg/util/rand"
)

func generateTestData(n int) []string {
	cidrs := make([]string, 0)
	prefix := fmt.Sprintf("%d.%d.%d", utilrand.IntnRange(1, 254), utilrand.IntnRange(1, 254), utilrand.IntnRange(1, 254))
	for i := 1; i <= n; i++ {
		if i%254 == 0 {
			prefix = fmt.Sprintf("%d.%d.%d", utilrand.IntnRange(1, 254), utilrand.IntnRange(1, 254), utilrand.IntnRange(1, 254))
		}

		cidr := fmt.Sprintf("%s.%d", prefix, (i%254)+1)
		if i%10 == 0 {
			cidr = fmt.Sprintf("%s/24", cidr)
		}
		cidrs = append(cidrs, cidr)
	}

	return cidrs
}

func TestValidateWhiteList(t *testing.T) {
	tests := []struct {
		name        string
		data        []string
		expectation []string
	}{
		{
			name:        "empty list",
			data:        []string{},
			expectation: []string{},
		},
		{
			name:        "blanks",
			data:        []string{"", "  ", "", "   ", " "},
			expectation: []string{},
		},
		{
			name:        "one ip",
			data:        []string{"1.2.3.4"},
			expectation: []string{"1.2.3.4"},
		},
		{
			name:        "onesie",
			data:        []string{"172.16.32.1/24"},
			expectation: []string{"172.16.32.1/24"},
		},
		{
			name:        "duo",
			data:        []string{"172.16.32.1/24", "10.1.2.3"},
			expectation: []string{"172.16.32.1/24", "10.1.2.3"},
		},
		{
			name:        "interleaved blank entries",
			data:        []string{"172.16.32.1/24", "", "1.2.3.4", "", "5.6.7.8", ""},
			expectation: []string{"172.16.32.1/24", "1.2.3.4", "5.6.7.8"},
		},
	}

	for _, tc := range tests {
		values, ok := ValidateWhiteList(strings.Join(tc.data, " "))
		if !reflect.DeepEqual(tc.expectation, values) {
			t.Errorf("%s: expected validated data %+v, got %+v", tc.name, tc.expectation, values)
		}
		flagExpectation := len(tc.expectation) <= 61
		if ok != flagExpectation {
			t.Errorf("%s: expected flag %+v, got %+v", tc.name, flagExpectation, ok)
		}
	}

	limitsTest := []int{9, 10, 16, 32, 60, 61, 62, 63, 64, 128, 253, 254, 255, 256, 512, 1024}
	for _, v := range limitsTest {
		name := fmt.Sprintf("limits-test-%d", v)
		data := generateTestData(v)
		values, ok := ValidateWhiteList(strings.Join(data, " "))
		if !reflect.DeepEqual(data, values) {
			t.Errorf("%s: expected validated data %+v, got %+v", name, data, values)
		}
		expectation := len(data) <= 61
		if ok != expectation {
			t.Errorf("%s: expected flag %+v, got %+v", name, expectation, ok)
		}
	}
}

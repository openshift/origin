package url

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTestsToScript(t *testing.T) {
	tests := []*Test{
		Expect("GET", "https://www.google.com"),
	}
	fmt.Println(testsToScript(tests))
}

func TestURL_Through(t *testing.T) {
	testcases := []struct {
		name    string
		through string
		wants   string
	}{
		{
			name:    "IPv4",
			through: "10.1.1.5",
			wants:   "10.1.1.5",
		},
		{
			name:    "IPv6",
			through: "fe80::cafe",
			wants:   "[fe80::cafe]",
		},
		{
			name:    "IPv6 already bracketed",
			through: "[fe80::cafe]",
			wants:   "[fe80::cafe]",
		},
		{
			name:    "Host",
			through: "example.com",
			wants:   "example.com",
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ut := Expect("GET", "http://www.google.com/").Through(tc.through)
			assert.Equal(t, tc.wants, ut.ProxyHost)
		})
	}
}

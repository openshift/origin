package util

import (
	"fmt"
	"strings"
	"testing"

	"github.com/docker/engine-api/types/container"
)

func TestSafeForLoggingContainerConfig(t *testing.T) {
	c := &container.Config{
		Env: []string{
			"http_proxy=http://user:password@hostname.com",
			"https_proxy=https://user:password@hostname.com",
			"HTTP_PROXY=HTTP://user:password@hostname.com",
			"HTTPS_PROXY=HTTPS://user:password@hostname.com",
		},
	}
	orig := fmt.Sprintf("%+v", *c)

	s := fmt.Sprintf("%+v", *SafeForLoggingContainerConfig(c))
	if strings.Contains(s, "user:password") {
		t.Errorf("expected %s to not contain credentials", s)
	}

	if !strings.Contains(s, "redacted") {
		t.Errorf("expected %s to be redacted", s)
	}

	// make sure the original object was not changed
	s = fmt.Sprintf("%+v", *c)
	if !(s == orig) {
		t.Errorf("expected original %s to be unchanged, got %s", orig, s)
	}

}

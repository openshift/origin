package google

import (
	"testing"

	"github.com/openshift/origin/pkg/auth/oauth/external"
)

func TestGoogle(t *testing.T) {
	_ = external.Provider(NewProvider("", ""))
}

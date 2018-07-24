package ocscheme

import (
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/origin/pkg/api/install"
	"github.com/openshift/origin/pkg/api/legacy"
)

// ReadingInternalScheme contains:
// 1. internal upstream and downstream types
// 2. external groupified
// 3. external non-groupified
var ReadingInternalScheme = runtime.NewScheme()

func init() {
	install.InstallInternalOpenShift(ReadingInternalScheme)
	install.InstallInternalKube(ReadingInternalScheme)
	legacy.InstallInternalLegacyAll(ReadingInternalScheme)
}

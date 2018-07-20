package ocscheme

import (
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/origin/pkg/api/install"
)

// PrintingInternalScheme contains:
// 1. internal upstream and downstream types
// 2. external groupified
var PrintingInternalScheme = runtime.NewScheme()

func init() {
	install.InstallInternalOpenShift(PrintingInternalScheme)
	install.InstallInternalKube(PrintingInternalScheme)
}

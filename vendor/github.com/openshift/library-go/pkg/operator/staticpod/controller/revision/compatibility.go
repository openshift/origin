package revision

import (
	"github.com/openshift/library-go/pkg/operator/revisioncontroller"
)

// RevisionResource is an type alias to keep source code compatibility for old
// consumers of this type when it was just used for static pods.
type RevisionResource = revisioncontroller.RevisionResource

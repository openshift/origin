package install

import (
	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/openshift/origin/pkg/build/controller/build/defaults/api"
	"github.com/openshift/origin/pkg/build/controller/build/defaults/api/v1"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

const importPrefix = "github.com/openshift/origin/pkg/build/controller/build/defaults/api"

var accessor = meta.NewAccessor()

// availableVersions lists all known external versions for this group from most preferred to least preferred
var availableVersions = []schema.GroupVersion{v1.SchemeGroupVersion}

func init() {
	if err := enableVersions(availableVersions); err != nil {
		panic(err)
	}
}

// TODO: enableVersions should be centralized rather than spread in each API
// group.
// We can combine registered.RegisterVersions, registered.EnableVersions and
// registered.RegisterGroup once we have moved enableVersions there.
func enableVersions(externalVersions []schema.GroupVersion) error {
	addVersionsToScheme(externalVersions...)
	return nil
}

func addVersionsToScheme(externalVersions ...schema.GroupVersion) {
	// add the internal version to Scheme
	api.AddToScheme(configapi.Scheme)
	// add the enabled external versions to Scheme
	for _, v := range externalVersions {
		switch v {
		case v1.SchemeGroupVersion:
			v1.AddToScheme(configapi.Scheme)

		default:
			glog.Errorf("Version %s is not known, so it will not be added to the Scheme.", v)
			continue
		}
	}
}

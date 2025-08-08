package scheme

import (
	"github.com/openshift/api"
)

func init() {
	api.InstallKube(Scheme)
	api.Install(Scheme)
}

package util

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/openshift/origin/pkg/build/api"
)

// GenerateBuild creates a new build based on a given BuildConfig. Optionally a SourceRevision for the new
// build can be specified
func GenerateBuild(bc *api.BuildConfig, r *api.SourceRevision) *api.Build {
	return &api.Build{
		Parameters: api.BuildParameters{
			Source:   bc.Parameters.Source,
			Strategy: bc.Parameters.Strategy,
			Output:   bc.Parameters.Output,
			Revision: r,
		},
		ObjectMeta: kapi.ObjectMeta{
			Labels: map[string]string{api.BuildConfigLabel: bc.Name},
		},
	}
}

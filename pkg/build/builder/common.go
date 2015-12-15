package builder

import (
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/generate/git"
)

const OriginalSourceURLAnnotationKey = "openshift.io/original-source-url"

// KeyValue can be used to build ordered lists of key-value pairs.
type KeyValue struct {
	Key   string
	Value string
}

// GitClient performs git operations
type GitClient interface {
	CloneWithOptions(dir string, url string, opts git.CloneOptions) error
	Checkout(dir string, ref string) error
	ListRemote(url string, args ...string) (string, string, error)
	GetInfo(location string) (*git.SourceInfo, []error)
}

// buildInfo returns a slice of KeyValue pairs with build metadata to be
// inserted into Docker images produced by build.
func buildInfo(build *api.Build) []KeyValue {
	kv := []KeyValue{
		{"OPENSHIFT_BUILD_NAME", build.Name},
		{"OPENSHIFT_BUILD_NAMESPACE", build.Namespace},
	}
	if build.Spec.Source.Git != nil {
		sourceURL := build.Spec.Source.Git.URI
		if originalURL, ok := build.Annotations[OriginalSourceURLAnnotationKey]; ok {
			sourceURL = originalURL
		}
		kv = append(kv, KeyValue{"OPENSHIFT_BUILD_SOURCE", sourceURL})
		if build.Spec.Source.Git.Ref != "" {
			kv = append(kv, KeyValue{"OPENSHIFT_BUILD_REFERENCE", build.Spec.Source.Git.Ref})
		}
		if build.Spec.Revision != nil && build.Spec.Revision.Git != nil && build.Spec.Revision.Git.Commit != "" {
			kv = append(kv, KeyValue{"OPENSHIFT_BUILD_COMMIT", build.Spec.Revision.Git.Commit})
		}
	}
	if build.Spec.Strategy.SourceStrategy != nil {
		env := build.Spec.Strategy.SourceStrategy.Env
		for _, e := range env {
			kv = append(kv, KeyValue{e.Name, e.Value})
		}
	}
	return kv
}

func updateBuildRevision(c client.BuildInterface, build *api.Build, sourceInfo *git.SourceInfo) {
	if build.Spec.Revision != nil {
		return
	}
	build.Spec.Revision = &api.SourceRevision{
		Git: &api.GitSourceRevision{
			Commit:  sourceInfo.CommitID,
			Message: sourceInfo.Message,
			Author: api.SourceControlUser{
				Name:  sourceInfo.AuthorName,
				Email: sourceInfo.AuthorEmail,
			},
			Committer: api.SourceControlUser{
				Name:  sourceInfo.CommitterName,
				Email: sourceInfo.CommitterEmail,
			},
		},
	}

	// Reset ResourceVersion to avoid a conflict with other updates to the build
	build.ResourceVersion = ""

	glog.V(4).Infof("Setting build revision to %#v", build.Spec.Revision.Git)
	_, err := c.UpdateDetails(build)
	if err != nil {
		glog.Warningf("An error occurred saving build revision: %v", err)
	}
}

package util

import (
	"strings"

	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/api"
)

// GenerateOutputImageLabels generate the labels based on the source repository
// informations.
func GenerateOutputImageLabels(info *api.SourceInfo, config *api.Config) map[string]string {
	result := map[string]string{}

	if len(config.Description) > 0 {
		result[api.KubernetesNamespace+"description"] = config.Description
	}

	if len(config.DisplayName) > 0 {
		result[api.KubernetesNamespace+"display-name"] = config.DisplayName
	} else {
		result[api.KubernetesNamespace+"display-name"] = config.Tag
	}

	if info == nil {
		glog.V(3).Infof("Unable to fetch source informations, the output image labels will not be set")
		return result
	}

	addBuildLabel(result, "image", config.BuilderImage)
	addBuildLabel(result, "commit.author", info.Author)
	addBuildLabel(result, "commit.date", info.Date)
	addBuildLabel(result, "commit.id", info.CommitID)
	addBuildLabel(result, "commit.ref", info.Ref)
	addBuildLabel(result, "commit.message", info.Message)
	addBuildLabel(result, "source-location", info.Location)
	addBuildLabel(result, "source-context-dir", config.ContextDir)

	return result
}

// addBuildLabel adds a new "io.openshift.s2i.build.*" label into map when the
// value of this label is not empty
func addBuildLabel(to map[string]string, key, value string) {
	if len(value) == 0 {
		return
	}
	to[strings.Join([]string{api.DefaultNamespace, "build.", key}, "")] = value
}

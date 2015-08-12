package util

import (
	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/api"
)

// GenerateOutputImageLabels generate the labels based on the s2i Config
// and source repository informations.
func GenerateOutputImageLabels(info *api.SourceInfo, config *api.Config) map[string]string {
	labels := map[string]string{}
	namespace := api.DefaultNamespace
	if len(config.LabelNamespace) > 0 {
		namespace = config.LabelNamespace
	}

	labels = GenerateLabelsFromConfig(labels, config, namespace)
	labels = GenerateLabelsFromSourceInfo(labels, info, namespace)
	return labels
}

// GenerateLabelsFromConfig generate the labels based on build s2i Config
func GenerateLabelsFromConfig(labels map[string]string, config *api.Config, namespace string) map[string]string {
	if len(config.Description) > 0 {
		labels[api.KubernetesNamespace+"description"] = config.Description
	}

	if len(config.DisplayName) > 0 {
		labels[api.KubernetesNamespace+"display-name"] = config.DisplayName
	} else {
		labels[api.KubernetesNamespace+"display-name"] = config.Tag
	}

	addBuildLabel(labels, "image", config.BuilderImage, namespace)
	addBuildLabel(labels, "source-context-dir", config.ContextDir, namespace)
	return labels
}

// GenerateLabelsFromSourceInfo generate the labels based on the source repository
// informations.
func GenerateLabelsFromSourceInfo(labels map[string]string, info *api.SourceInfo, namespace string) map[string]string {
	if info == nil {
		glog.V(3).Infof("Unable to fetch source informations, the output image labels will not be set")
		return labels
	}

	addBuildLabel(labels, "commit.author", info.Author, namespace)
	addBuildLabel(labels, "commit.date", info.Date, namespace)
	addBuildLabel(labels, "commit.id", info.CommitID, namespace)
	addBuildLabel(labels, "commit.ref", info.Ref, namespace)
	addBuildLabel(labels, "commit.message", info.Message, namespace)
	addBuildLabel(labels, "source-location", info.Location, namespace)
	return labels
}

// addBuildLabel adds a new "*.build.*" label into map when the
// value of this label is not empty
func addBuildLabel(to map[string]string, key, value, namespace string) {
	if len(value) == 0 {
		return
	}
	to[namespace+"build."+key] = value
}

package v1

import (
	"k8s.io/api/core/v1"

	newer "github.com/openshift/origin/pkg/image/apis/image"
)

func SetDefaults_ImageImportSpec(obj *ImageImportSpec) {
	if obj.To == nil {
		if ref, err := newer.ParseDockerImageReference(obj.From.Name); err == nil {
			if len(ref.Tag) > 0 {
				obj.To = &v1.LocalObjectReference{Name: ref.Tag}
			}
		}
	}
}

func SetDefaults_TagReferencePolicy(obj *TagReferencePolicy) {
	if len(obj.Type) == 0 {
		obj.Type = SourceTagReferencePolicy
	}
}

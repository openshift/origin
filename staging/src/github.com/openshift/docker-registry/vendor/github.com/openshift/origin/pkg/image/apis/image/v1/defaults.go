package v1

import (
	newer "github.com/openshift/origin/pkg/image/apis/image"
	"k8s.io/kubernetes/pkg/api/v1"
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

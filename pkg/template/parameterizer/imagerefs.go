package parameterizer

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/api/meta"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

var (
	parameterRegex                = regexp.MustCompile("\\$\\{[^}]+\\}")
	parameterNameInvalidCharRegex = regexp.MustCompile("[^a-zA-Z0-9_]")

	// ImageRefParameterizer is a Parameterizer for image references in runtime objects
	ImageRefParameterizer Parameterizer = &imageRefParameterizer{}
)

// ImageRefParameterizer is a parameterizer for image references in a template
type imageRefParameterizer struct{}

func makeValidParameterName(value string) string {
	return strings.ToUpper(parameterNameInvalidCharRegex.ReplaceAllString(value, "_"))
}

func imageStreamTagParamName(value string) string {
	return fmt.Sprintf("%s_IMAGE_STREAM_TAG", makeValidParameterName(imageStreamTagImageStream(value)))
}

func imageStreamImageParamName(value string) string {
	return fmt.Sprintf("%s_IMAGE_STREAM_IMAGE", makeValidParameterName(imageStreamImageImageStream(value)))
}

func dockerImageRefParamName(value string) string {
	ref, err := imageapi.ParseDockerImageReference(value)
	if err != nil {
		return "IMAGE_REF"
	}
	return fmt.Sprintf("%s_IMAGE_REF", makeValidParameterName(ref.Name))
}

func namespaceParamName(name string) string {
	return fmt.Sprintf("%s_NS", makeValidParameterName(name))
}

func formatParameter(name string) string {
	return fmt.Sprintf("${%s}", name)
}

func imageStreamTagImageStream(ref string) string {
	name, _, err := imageapi.ParseImageStreamTagName(ref)
	if err != nil {
		return ""
	}
	return name
}

func imageStreamImageImageStream(ref string) string {
	name, _, err := imageapi.ParseImageStreamImageName(ref)
	if err != nil {
		return ""
	}
	return name
}

func includesImageStream(name string, objs []runtime.Object) bool {
	for _, obj := range objs {
		if is, isImageStream := obj.(*imageapi.ImageStream); isImageStream {
			if is.Name == name {
				return true
			}
		}
	}
	return false
}

func isParameter(value string) bool {
	return parameterRegex.MatchString(value)
}

func parameterizeImageStreamImage(ref *kapi.ObjectReference, objs []runtime.Object, params Params) {
	parameterizeNs := false
	if isParameter(ref.Name) {
		return
	}
	if len(ref.Namespace) > 0 {
		parameterizeNs = true
	}
	if !parameterizeNs && includesImageStream(imageStreamImageImageStream(ref.Name), objs) {
		return
	}
	paramName := params.AddParam(makeParameter(imageStreamImageParamName(ref.Name), ref.Name))
	ref.Name = formatParameter(paramName)
	if parameterizeNs {
		nsParamName := params.AddParam(makeParameter(namespaceParamName(paramName), ref.Namespace))
		ref.Namespace = formatParameter(nsParamName)
	}
}

func parameterizeImageStreamTag(ref *kapi.ObjectReference, objs []runtime.Object, params Params) {
	parameterizeNs := false
	if isParameter(ref.Name) {
		return
	}
	if len(ref.Namespace) > 0 {
		parameterizeNs = true
	}
	if !parameterizeNs && includesImageStream(imageStreamTagImageStream(ref.Name), objs) {
		return
	}
	paramName := params.AddParam(makeParameter(imageStreamTagParamName(ref.Name), ref.Name))
	ref.Name = formatParameter(paramName)
	if parameterizeNs {
		nsParamName := params.AddParam(makeParameter(namespaceParamName(paramName), ref.Namespace))
		ref.Namespace = formatParameter(nsParamName)
	}
}

func parameterizeDockerImageRef(ref *kapi.ObjectReference, objs []runtime.Object, params Params) {
	if isParameter(ref.Name) {
		return
	}
	paramName := params.AddParam(makeParameter(dockerImageRefParamName(ref.Name), ref.Name))
	ref.Name = formatParameter(paramName)
}

func parameterizeBuildConfig(bc *buildapi.BuildConfig, objs []runtime.Object, params Params) {
	for _, trigger := range bc.Spec.Triggers {
		if trigger.ImageChange == nil || trigger.ImageChange.From == nil || trigger.ImageChange.From.Kind != "ImageStreamTag" {
			continue
		}
		parameterizeImageStreamTag(trigger.ImageChange.From, objs, params)
	}
	if bc.Spec.Output.To != nil && !isParameter(bc.Spec.Output.To.Name) {
		parameterizeRef(bc.Spec.Output.To, objs, params)
	}
	parameterizeRuntimeObject(bc, objs, params)
}

func parameterizeDeploymentConfig(dc *deployapi.DeploymentConfig, objs []runtime.Object, params Params) {
	triggerContainers := sets.NewString()
	for _, trigger := range dc.Spec.Triggers {
		if trigger.ImageChangeParams == nil || trigger.ImageChangeParams.From.Kind != "ImageStreamTag" {
			continue
		}
		parameterizeImageStreamTag(&trigger.ImageChangeParams.From, objs, params)
		triggerContainers.Insert(trigger.ImageChangeParams.ContainerNames...)
	}
	if dc.Spec.Template == nil {
		return
	}
	parameterizeContainer := func(container *kapi.Container) {
		// Skip if the image value will be set by a trigger or is already parameterized
		if triggerContainers.Has(container.Name) {
			return
		}
		ref := &kapi.ObjectReference{
			Name: container.Image,
			Kind: "DockerImage",
		}
		parameterizeDockerImageRef(ref, objs, params)
		container.Image = ref.Name
	}
	for i := range dc.Spec.Template.Spec.Containers {
		parameterizeContainer(&dc.Spec.Template.Spec.Containers[i])
	}
	for i := range dc.Spec.Template.Spec.InitContainers {
		parameterizeContainer(&dc.Spec.Template.Spec.InitContainers[i])
	}
}

func parameterizeRef(ref *kapi.ObjectReference, objs []runtime.Object, params Params) {
	switch ref.Kind {
	case "ImageStreamImage":
		parameterizeImageStreamImage(ref, objs, params)
	case "ImageStreamTag":
		parameterizeImageStreamTag(ref, objs, params)
	case "DockerImage":
		parameterizeDockerImageRef(ref, objs, params)
	}
}

func parameterizeRuntimeObject(object runtime.Object, objs []runtime.Object, params Params) {
	m, err := meta.GetImageReferenceMutator(object)
	if err != nil {
		return // Object has no image refs to mutate
	}
	m.Mutate(func(ref *kapi.ObjectReference) error {
		parameterizeRef(ref, objs, params)
		return nil
	})
}

// Parameterize generates parameters for the given object and returns
func (i *imageRefParameterizer) Parameterize(object runtime.Object, objs []runtime.Object, params Params) error {
	switch t := object.(type) {
	case *deployapi.DeploymentConfig:
		parameterizeDeploymentConfig(t, objs, params)
	case *buildapi.BuildConfig:
		parameterizeBuildConfig(t, objs, params)
	default:
		parameterizeRuntimeObject(t, objs, params)
	}
	return nil
}

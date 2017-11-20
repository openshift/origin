package build

import (
	"fmt"

	"k8s.io/api/core/v1"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

// buildPodCreationStrategy is used by the build controller to
// create a build pod based on a build strategy
type buildPodCreationStrategy interface {
	CreateBuildPod(build *buildapi.Build) (*v1.Pod, error)
}

type typeBasedFactoryStrategy struct {
	dockerBuildStrategy buildPodCreationStrategy
	sourceBuildStrategy buildPodCreationStrategy
	customBuildStrategy buildPodCreationStrategy
}

func (f *typeBasedFactoryStrategy) CreateBuildPod(build *buildapi.Build) (*v1.Pod, error) {
	var pod *v1.Pod
	var err error
	switch {
	case build.Spec.Strategy.DockerStrategy != nil:
		pod, err = f.dockerBuildStrategy.CreateBuildPod(build)
	case build.Spec.Strategy.SourceStrategy != nil:
		pod, err = f.sourceBuildStrategy.CreateBuildPod(build)
	case build.Spec.Strategy.CustomStrategy != nil:
		pod, err = f.customBuildStrategy.CreateBuildPod(build)
	case build.Spec.Strategy.JenkinsPipelineStrategy != nil:
		return nil, fmt.Errorf("creating a build pod for Build %s/%s with the JenkinsPipeline strategy is not supported", build.Namespace, build.Name)
	default:
		return nil, fmt.Errorf("no supported build strategy defined for Build %s/%s", build.Namespace, build.Name)
	}

	if pod != nil {
		if pod.Annotations == nil {
			pod.Annotations = map[string]string{}
		}
		pod.Annotations[buildapi.BuildAnnotation] = build.Name
	}
	return pod, err
}

package build

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// buildPodCreationStrategy is used by the build controller to
// create a build pod based on a build strategy
type buildPodCreationStrategy interface {
	CreateBuildPod(build *buildapi.Build) (*kapi.Pod, error)
}

type typeBasedFactoryStrategy struct {
	dockerBuildStrategy buildPodCreationStrategy
	sourceBuildStrategy buildPodCreationStrategy
	customBuildStrategy buildPodCreationStrategy
}

func (f *typeBasedFactoryStrategy) CreateBuildPod(build *buildapi.Build) (*kapi.Pod, error) {
	var pod *kapi.Pod
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

package git

import (
	"os"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/openshift/origin/pkg/resourcewatch/observe"
)

func Sink(log logr.Logger) (observe.ObservationSink, error) {
	gitStorage, err := gitInitStorage()
	if err != nil {
		log.Error(err, "Failed to create git storage")
		return nil, err
	}

	return func(log logr.Logger, resourceC <-chan *observe.ResourceObservation) chan struct{} {
		finished := make(chan struct{})
		go func() {
			defer close(finished)
			for observation := range resourceC {
				gitWrite(gitStorage, observation)
			}
		}()
		return finished
	}, nil
}

func gitInitStorage() (*GitStorage, error) {
	repositoryPath := "/repository"
	if repositoryPathEnv := os.Getenv("REPOSITORY_PATH"); len(repositoryPathEnv) > 0 {
		repositoryPath = repositoryPathEnv
	}
	return NewGitStorage(repositoryPath)
}

func gitWrite(gitStorage *GitStorage, observation *observe.ResourceObservation) {
	gvr := schema.GroupVersionResource{
		Group:    observation.Group,
		Version:  observation.Version,
		Resource: observation.Resource,
	}
	switch observation.ObservationType {
	case observe.ObservationTypeAdd:
		gitStorage.OnAdd(gvr, observation.Object)
	case observe.ObservationTypeUpdate:
		gitStorage.OnUpdate(gvr, observation.OldObject, observation.Object)
	case observe.ObservationTypeDelete:
		gitStorage.OnDelete(gvr, observation.Object)
	}
}

package meta

import (
	"k8s.io/kubernetes/pkg/util/validation/field"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

type buildSpecMutator struct {
	spec *buildapi.CommonSpec
	path *field.Path
}

func (m *buildSpecMutator) Mutate(fn ImageReferenceMutateFunc) field.ErrorList {
	var errs field.ErrorList
	for i, image := range m.spec.Source.Images {
		if err := fn(&image.From); err != nil {
			errs = append(errs, field.InternalError(m.path.Child("source", "images").Index(i).Child("from", "name"), err))
			continue
		}
	}
	if s := m.spec.Strategy.CustomStrategy; s != nil {
		if err := fn(&s.From); err != nil {
			errs = append(errs, field.InternalError(m.path.Child("strategy", "customStrategy", "from", "name"), err))
		}
	}
	if s := m.spec.Strategy.DockerStrategy; s != nil {
		if s.From != nil {
			if err := fn(s.From); err != nil {
				errs = append(errs, field.InternalError(m.path.Child("strategy", "dockerStrategy", "from", "name"), err))
			}
		}
	}
	if s := m.spec.Strategy.SourceStrategy; s != nil {
		if err := fn(&s.From); err != nil {
			errs = append(errs, field.InternalError(m.path.Child("strategy", "sourceStrategy", "from", "name"), err))
		}
	}
	return errs
}

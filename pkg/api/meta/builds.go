package meta

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

type buildSpecMutator struct {
	spec   *buildapi.CommonSpec
	path   *field.Path
	output bool
}

// NewBuildMutator returns an ImageReferenceMutator that includes the output field.
func NewBuildMutator(build *buildapi.Build) ImageReferenceMutator {
	return &buildSpecMutator{
		spec:   &build.Spec.CommonSpec,
		path:   field.NewPath("spec"),
		output: true,
	}
}

func (m *buildSpecMutator) Mutate(fn ImageReferenceMutateFunc) field.ErrorList {
	var errs field.ErrorList
	for i, image := range m.spec.Source.Images {
		if err := fn(&image.From); err != nil {
			errs = append(errs, fieldErrorOrInternal(err, m.path.Child("source", "images").Index(i).Child("from", "name")))
			continue
		}
	}
	if s := m.spec.Strategy.CustomStrategy; s != nil {
		if err := fn(&s.From); err != nil {
			errs = append(errs, fieldErrorOrInternal(err, m.path.Child("strategy", "customStrategy", "from", "name")))
		}
	}
	if s := m.spec.Strategy.DockerStrategy; s != nil {
		if s.From != nil {
			if err := fn(s.From); err != nil {
				errs = append(errs, fieldErrorOrInternal(err, m.path.Child("strategy", "dockerStrategy", "from", "name")))
			}
		}
	}
	if s := m.spec.Strategy.SourceStrategy; s != nil {
		if err := fn(&s.From); err != nil {
			errs = append(errs, fieldErrorOrInternal(err, m.path.Child("strategy", "sourceStrategy", "from", "name")))
		}
	}
	if m.output {
		if s := m.spec.Output.To; s != nil {
			if err := fn(s); err != nil {
				errs = append(errs, fieldErrorOrInternal(err, m.path.Child("output", "to")))
			}
		}
	}
	return errs
}

func fieldErrorOrInternal(err error, path *field.Path) *field.Error {
	if ferr, ok := err.(*field.Error); ok {
		if len(ferr.Field) == 0 {
			ferr.Field = path.String()
		}
		return ferr
	}
	if errors.IsNotFound(err) {
		return field.NotFound(path, err)
	}
	return field.InternalError(path, err)
}

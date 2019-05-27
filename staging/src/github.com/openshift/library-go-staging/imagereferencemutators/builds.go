package imagereferencemutators

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"

	buildv1 "github.com/openshift/api/build/v1"
)

type buildSpecMutator struct {
	spec    *buildv1.CommonSpec
	oldSpec *buildv1.CommonSpec
	path    *field.Path
	output  bool
}

// NewBuildMutator returns an ImageReferenceMutator that includes the output field.
func NewBuildMutator(build *buildv1.Build) ImageReferenceMutator {
	return &buildSpecMutator{
		spec:   &build.Spec.CommonSpec,
		path:   field.NewPath("spec"),
		output: true,
	}
}

func hasIdenticalImageSourceObjectReference(spec *buildv1.CommonSpec, ref corev1.ObjectReference) bool {
	if spec == nil {
		return false
	}
	for i := range spec.Source.Images {
		if spec.Source.Images[i].From == ref {
			return true
		}
	}
	return false
}

func hasIdenticalStrategyFrom(spec, oldSpec *buildv1.CommonSpec) bool {
	if oldSpec == nil {
		return false
	}
	switch {
	case spec.Strategy.CustomStrategy != nil:
		if oldSpec.Strategy.CustomStrategy != nil {
			return spec.Strategy.CustomStrategy.From == oldSpec.Strategy.CustomStrategy.From
		}
	case spec.Strategy.DockerStrategy != nil:
		if oldSpec.Strategy.DockerStrategy != nil {
			return hasIdenticalObjectReference(spec.Strategy.DockerStrategy.From, oldSpec.Strategy.DockerStrategy.From)
		}
	case spec.Strategy.SourceStrategy != nil:
		if oldSpec.Strategy.SourceStrategy != nil {
			return spec.Strategy.SourceStrategy.From == oldSpec.Strategy.SourceStrategy.From
		}
	}
	return false
}

func hasIdenticalObjectReference(ref, oldRef *corev1.ObjectReference) bool {
	if ref == nil || oldRef == nil {
		return false
	}
	return *ref == *oldRef
}

func (m *buildSpecMutator) Mutate(fn ImageReferenceMutateFunc) field.ErrorList {
	var errs field.ErrorList
	for i := range m.spec.Source.Images {
		if hasIdenticalImageSourceObjectReference(m.oldSpec, m.spec.Source.Images[i].From) {
			continue
		}
		if err := fn(&m.spec.Source.Images[i].From); err != nil {
			errs = append(errs, fieldErrorOrInternal(err, m.path.Child("source", "images").Index(i).Child("from", "name")))
			continue
		}
	}
	if !hasIdenticalStrategyFrom(m.spec, m.oldSpec) {
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
	}
	if m.output {
		if s := m.spec.Output.To; s != nil {
			if m.oldSpec == nil || m.oldSpec.Output.To == nil || !hasIdenticalObjectReference(s, m.oldSpec.Output.To) {
				if err := fn(s); err != nil {
					errs = append(errs, fieldErrorOrInternal(err, m.path.Child("output", "to")))
				}
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

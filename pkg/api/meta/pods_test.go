package meta

import (
	"reflect"
	"testing"

	kapiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	_ "github.com/openshift/origin/pkg/api/install"
)

func TestResourcesToCheck(t *testing.T) {
	known := knownResourceKinds()
	detected := resourcesToCheck
	for _, k := range detected {
		if _, isKnown := known[k]; !isKnown {
			t.Errorf("Unknown resource kind %s contains a PodSpec", (&k).String())
			continue
		}
		delete(known, k)
	}
	if len(known) > 0 {
		t.Errorf("These known kinds were not detected to have a PodSpec: %#v", known)
	}
}

var podSpecType = reflect.TypeOf(kapi.PodSpec{})

func hasPodSpec(visited map[reflect.Type]bool, t reflect.Type) bool {
	if visited[t] {
		return false
	}
	visited[t] = true

	switch t.Kind() {
	case reflect.Struct:
		if t == podSpecType {
			return true
		}
		for i := 0; i < t.NumField(); i++ {
			if hasPodSpec(visited, t.Field(i).Type) {
				return true
			}
		}
	case reflect.Array, reflect.Slice, reflect.Chan, reflect.Map, reflect.Ptr:
		return hasPodSpec(visited, t.Elem())
	}
	return false
}

func internalGroupVersions() []schema.GroupVersion {
	groupVersions := legacyscheme.Registry.EnabledVersions()
	groups := map[string]struct{}{}
	for _, gv := range groupVersions {
		groups[gv.Group] = struct{}{}
	}
	result := []schema.GroupVersion{}
	for group := range groups {
		result = append(result, schema.GroupVersion{Group: group, Version: runtime.APIVersionInternal})
	}
	return result
}

func isList(t reflect.Type) bool {
	if t.Kind() != reflect.Struct {
		return false
	}

	_, hasListMeta := t.FieldByName("ListMeta")
	return hasListMeta
}

func kindsWithPodSpecs() []schema.GroupKind {
	result := []schema.GroupKind{}
	for _, gv := range internalGroupVersions() {
		knownTypes := legacyscheme.Scheme.KnownTypes(gv)
		for kind, knownType := range knownTypes {
			if !isList(knownType) && hasPodSpec(map[reflect.Type]bool{}, knownType) {
				result = append(result, schema.GroupKind{Group: gv.Group, Kind: kind})
			}
		}
	}

	return result
}

func knownResourceKinds() map[schema.GroupKind]struct{} {
	result := map[schema.GroupKind]struct{}{}
	for _, ka := range resourcesToCheck {
		result[ka] = struct{}{}
	}
	return result
}

func Test_podSpecMutator_Mutate(t *testing.T) {
	imageRef := func(name string) *kapi.ObjectReference {
		ref := imageRefValue(name)
		return &ref
	}

	type fields struct {
		spec    *kapi.PodSpec
		oldSpec *kapi.PodSpec
		path    *field.Path
	}
	type args struct {
		fn ImageReferenceMutateFunc
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		want     field.ErrorList
		wantSpec *kapi.PodSpec
	}{
		{
			name:   "no-op",
			fields: fields{spec: &kapi.PodSpec{}},
		},
		{
			name: "passes init container reference",
			fields: fields{spec: &kapi.PodSpec{
				InitContainers: []kapi.Container{
					{Name: "1", Image: "test"},
				},
			}},
			args: args{fn: func(ref *kapi.ObjectReference) error {
				if !reflect.DeepEqual(ref, imageRef("test")) {
					t.Errorf("unexpected ref: %#v", ref)
				}
				return nil
			}},
			wantSpec: &kapi.PodSpec{
				InitContainers: []kapi.Container{
					{Name: "1", Image: "test"},
				},
			},
		},
		{
			name: "passes container reference",
			fields: fields{spec: &kapi.PodSpec{
				Containers: []kapi.Container{
					{Name: "1", Image: "test"},
				},
			}},
			args: args{fn: func(ref *kapi.ObjectReference) error {
				if !reflect.DeepEqual(ref, imageRef("test")) {
					t.Errorf("unexpected ref: %#v", ref)
				}
				return nil
			}},
			wantSpec: &kapi.PodSpec{
				Containers: []kapi.Container{
					{Name: "1", Image: "test"},
				},
			},
		},

		{
			name: "mutates reference",
			fields: fields{spec: &kapi.PodSpec{
				InitContainers: []kapi.Container{
					{Name: "1", Image: "test"},
				},
				Containers: []kapi.Container{
					{Name: "2", Image: "test-2"},
				},
			}},
			args: args{fn: func(ref *kapi.ObjectReference) error {
				if ref.Name == "test-2" {
					ref.Name = "test-3"
				}
				return nil
			}},
			wantSpec: &kapi.PodSpec{
				InitContainers: []kapi.Container{
					{Name: "1", Image: "test"},
				},
				Containers: []kapi.Container{
					{Name: "2", Image: "test-3"},
				},
			},
		},
		{
			name: "mutates only changed references",
			fields: fields{
				spec: &kapi.PodSpec{
					InitContainers: []kapi.Container{
						{Name: "1", Image: "test"},
					},
					Containers: []kapi.Container{
						{Name: "2", Image: "test-2"},
					},
				},
				oldSpec: &kapi.PodSpec{
					InitContainers: []kapi.Container{
						{Name: "1", Image: "test-1"},
					},
					Containers: []kapi.Container{
						{Name: "2", Image: "test-2"},
					},
				},
			},
			args: args{fn: func(ref *kapi.ObjectReference) error {
				if ref.Name != "test" {
					t.Errorf("did not expect to be called for existing reference")
				}
				ref.Name = "test-3"
				return nil
			}},
			wantSpec: &kapi.PodSpec{
				InitContainers: []kapi.Container{
					{Name: "1", Image: "test-3"},
				},
				Containers: []kapi.Container{
					{Name: "2", Image: "test-2"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &podSpecMutator{
				spec:    tt.fields.spec,
				oldSpec: tt.fields.oldSpec,
				path:    tt.fields.path,
			}
			if tt.wantSpec == nil {
				tt.wantSpec = &kapi.PodSpec{}
			}
			if got := m.Mutate(tt.args.fn); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildSpecMutator.Mutate() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(tt.wantSpec, tt.fields.spec) {
				t.Errorf("buildSpecMutator.Mutate() spec = %v, want %v", tt.fields.spec, tt.wantSpec)
			}
		})
	}
}

func Test_podSpecV1Mutator_Mutate(t *testing.T) {
	type fields struct {
		spec    *kapiv1.PodSpec
		oldSpec *kapiv1.PodSpec
		path    *field.Path
	}
	type args struct {
		fn ImageReferenceMutateFunc
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		want     field.ErrorList
		wantSpec *kapiv1.PodSpec
	}{
		{
			name:   "no-op",
			fields: fields{spec: &kapiv1.PodSpec{}},
		},
		{
			name: "passes init container reference",
			fields: fields{spec: &kapiv1.PodSpec{
				InitContainers: []kapiv1.Container{
					{Name: "1", Image: "test"},
				},
			}},
			args: args{fn: func(ref *kapi.ObjectReference) error {
				if !reflect.DeepEqual(ref, imageRef("test")) {
					t.Errorf("unexpected ref: %#v", ref)
				}
				return nil
			}},
			wantSpec: &kapiv1.PodSpec{
				InitContainers: []kapiv1.Container{
					{Name: "1", Image: "test"},
				},
			},
		},
		{
			name: "passes container reference",
			fields: fields{spec: &kapiv1.PodSpec{
				Containers: []kapiv1.Container{
					{Name: "1", Image: "test"},
				},
			}},
			args: args{fn: func(ref *kapi.ObjectReference) error {
				if !reflect.DeepEqual(ref, imageRef("test")) {
					t.Errorf("unexpected ref: %#v", ref)
				}
				return nil
			}},
			wantSpec: &kapiv1.PodSpec{
				Containers: []kapiv1.Container{
					{Name: "1", Image: "test"},
				},
			},
		},

		{
			name: "mutates reference",
			fields: fields{spec: &kapiv1.PodSpec{
				InitContainers: []kapiv1.Container{
					{Name: "1", Image: "test"},
				},
				Containers: []kapiv1.Container{
					{Name: "2", Image: "test-2"},
				},
			}},
			args: args{fn: func(ref *kapi.ObjectReference) error {
				if ref.Name == "test-2" {
					ref.Name = "test-3"
				}
				return nil
			}},
			wantSpec: &kapiv1.PodSpec{
				InitContainers: []kapiv1.Container{
					{Name: "1", Image: "test"},
				},
				Containers: []kapiv1.Container{
					{Name: "2", Image: "test-3"},
				},
			},
		},
		{
			name: "mutates only changed references",
			fields: fields{
				spec: &kapiv1.PodSpec{
					InitContainers: []kapiv1.Container{
						{Name: "1", Image: "test"},
					},
					Containers: []kapiv1.Container{
						{Name: "2", Image: "test-2"},
					},
				},
				oldSpec: &kapiv1.PodSpec{
					InitContainers: []kapiv1.Container{
						{Name: "1", Image: "test-1"},
					},
					Containers: []kapiv1.Container{
						{Name: "2", Image: "test-2"},
					},
				},
			},
			args: args{fn: func(ref *kapi.ObjectReference) error {
				if ref.Name != "test" {
					t.Errorf("did not expect to be called for existing reference")
				}
				ref.Name = "test-3"
				return nil
			}},
			wantSpec: &kapiv1.PodSpec{
				InitContainers: []kapiv1.Container{
					{Name: "1", Image: "test-3"},
				},
				Containers: []kapiv1.Container{
					{Name: "2", Image: "test-2"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &podSpecV1Mutator{
				spec:    tt.fields.spec,
				oldSpec: tt.fields.oldSpec,
				path:    tt.fields.path,
			}
			if tt.wantSpec == nil {
				tt.wantSpec = &kapiv1.PodSpec{}
			}
			if got := m.Mutate(tt.args.fn); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildSpecMutator.Mutate() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(tt.wantSpec, tt.fields.spec) {
				t.Errorf("buildSpecMutator.Mutate() spec = %v, want %v", tt.fields.spec, tt.wantSpec)
			}
		})
	}
}

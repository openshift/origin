package internalversion

import (
	"reflect"
	"testing"

	kapiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	_ "github.com/openshift/origin/pkg/api/install"
)

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

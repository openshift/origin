package referencemutator

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Test_podSpecV1Mutator_Mutate(t *testing.T) {
	type fields struct {
		spec    *corev1.PodSpec
		oldSpec *corev1.PodSpec
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
		wantSpec *corev1.PodSpec
	}{
		{
			name:   "no-op",
			fields: fields{spec: &corev1.PodSpec{}},
		},
		{
			name: "passes init container reference",
			fields: fields{spec: &corev1.PodSpec{
				InitContainers: []corev1.Container{
					{Name: "1", Image: "test"},
				},
			}},
			args: args{fn: func(ref *corev1.ObjectReference) error {
				if !reflect.DeepEqual(ref, imageRef("test")) {
					t.Errorf("unexpected ref: %#v", ref)
				}
				return nil
			}},
			wantSpec: &corev1.PodSpec{
				InitContainers: []corev1.Container{
					{Name: "1", Image: "test"},
				},
			},
		},
		{
			name: "passes container reference",
			fields: fields{spec: &corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "1", Image: "test"},
				},
			}},
			args: args{fn: func(ref *corev1.ObjectReference) error {
				if !reflect.DeepEqual(ref, imageRef("test")) {
					t.Errorf("unexpected ref: %#v", ref)
				}
				return nil
			}},
			wantSpec: &corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "1", Image: "test"},
				},
			},
		},

		{
			name: "mutates reference",
			fields: fields{spec: &corev1.PodSpec{
				InitContainers: []corev1.Container{
					{Name: "1", Image: "test"},
				},
				Containers: []corev1.Container{
					{Name: "2", Image: "test-2"},
				},
			}},
			args: args{fn: func(ref *corev1.ObjectReference) error {
				if ref.Name == "test-2" {
					ref.Name = "test-3"
				}
				return nil
			}},
			wantSpec: &corev1.PodSpec{
				InitContainers: []corev1.Container{
					{Name: "1", Image: "test"},
				},
				Containers: []corev1.Container{
					{Name: "2", Image: "test-3"},
				},
			},
		},
		{
			name: "mutates only changed references",
			fields: fields{
				spec: &corev1.PodSpec{
					InitContainers: []corev1.Container{
						{Name: "1", Image: "test"},
					},
					Containers: []corev1.Container{
						{Name: "2", Image: "test-2"},
					},
				},
				oldSpec: &corev1.PodSpec{
					InitContainers: []corev1.Container{
						{Name: "1", Image: "test-1"},
					},
					Containers: []corev1.Container{
						{Name: "2", Image: "test-2"},
					},
				},
			},
			args: args{fn: func(ref *corev1.ObjectReference) error {
				if ref.Name != "test" {
					t.Errorf("did not expect to be called for existing reference")
				}
				ref.Name = "test-3"
				return nil
			}},
			wantSpec: &corev1.PodSpec{
				InitContainers: []corev1.Container{
					{Name: "1", Image: "test-3"},
				},
				Containers: []corev1.Container{
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
				tt.wantSpec = &corev1.PodSpec{}
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

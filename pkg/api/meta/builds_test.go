package meta

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/apis/core"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func imageRef(name string) *kapi.ObjectReference {
	ref := imageRefValue(name)
	return &ref
}
func imageRefValue(name string) kapi.ObjectReference {
	return kapi.ObjectReference{Kind: "DockerImage", Name: name}
}

func Test_buildSpecMutator_Mutate(t *testing.T) {
	type fields struct {
		spec    *buildapi.CommonSpec
		oldSpec *buildapi.CommonSpec
		path    *field.Path
		output  bool
	}
	type args struct {
		fn ImageReferenceMutateFunc
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		want     field.ErrorList
		wantSpec *buildapi.CommonSpec
	}{
		{
			name:   "no-op",
			fields: fields{spec: &buildapi.CommonSpec{}},
		},
		{
			name: "passes reference",
			fields: fields{spec: &buildapi.CommonSpec{
				Strategy: buildapi.BuildStrategy{
					DockerStrategy: &buildapi.DockerBuildStrategy{From: imageRef("test")},
				},
			}},
			args: args{fn: func(ref *kapi.ObjectReference) error {
				if !reflect.DeepEqual(ref, imageRef("test")) {
					t.Errorf("unexpected ref: %#v", ref)
				}
				return nil
			}},
			wantSpec: &buildapi.CommonSpec{
				Strategy: buildapi.BuildStrategy{
					DockerStrategy: &buildapi.DockerBuildStrategy{From: imageRef("test")},
				},
			},
		},
		{
			name: "mutates docker reference",
			fields: fields{spec: &buildapi.CommonSpec{
				Strategy: buildapi.BuildStrategy{
					DockerStrategy: &buildapi.DockerBuildStrategy{From: imageRef("test")},
				},
			}},
			args: args{fn: func(ref *kapi.ObjectReference) error {
				ref.Name = "test-2"
				return nil
			}},
			wantSpec: &buildapi.CommonSpec{
				Strategy: buildapi.BuildStrategy{
					DockerStrategy: &buildapi.DockerBuildStrategy{From: imageRef("test-2")},
				},
			},
		},
		{
			name: "mutates source reference",
			fields: fields{spec: &buildapi.CommonSpec{
				Strategy: buildapi.BuildStrategy{
					SourceStrategy: &buildapi.SourceBuildStrategy{From: imageRefValue("test")},
				},
			}},
			args: args{fn: func(ref *kapi.ObjectReference) error {
				ref.Name = "test-2"
				return nil
			}},
			wantSpec: &buildapi.CommonSpec{
				Strategy: buildapi.BuildStrategy{
					SourceStrategy: &buildapi.SourceBuildStrategy{From: imageRefValue("test-2")},
				},
			},
		},
		{
			name: "mutates custom reference",
			fields: fields{spec: &buildapi.CommonSpec{
				Strategy: buildapi.BuildStrategy{
					CustomStrategy: &buildapi.CustomBuildStrategy{From: imageRefValue("test")},
				},
			}},
			args: args{fn: func(ref *kapi.ObjectReference) error {
				ref.Name = "test-2"
				return nil
			}},
			wantSpec: &buildapi.CommonSpec{
				Strategy: buildapi.BuildStrategy{
					CustomStrategy: &buildapi.CustomBuildStrategy{From: imageRefValue("test-2")},
				},
			},
		},
		{
			name: "mutates image source references",
			fields: fields{spec: &buildapi.CommonSpec{
				Source: buildapi.BuildSource{Images: []buildapi.ImageSource{
					{From: imageRefValue("test-1")},
					{From: imageRefValue("test-2")},
					{From: imageRefValue("test-3")},
				}},
			}},
			args: args{fn: func(ref *kapi.ObjectReference) error {
				if ref.Name == "test-2" {
					ref.Name = "test-4"
				}
				return nil
			}},
			wantSpec: &buildapi.CommonSpec{
				Source: buildapi.BuildSource{Images: []buildapi.ImageSource{
					{From: imageRefValue("test-1")},
					{From: imageRefValue("test-4")},
					{From: imageRefValue("test-3")},
				}},
			},
		},
		{
			name: "mutates only changed references",
			fields: fields{
				spec: &buildapi.CommonSpec{
					Source: buildapi.BuildSource{Images: []buildapi.ImageSource{
						{From: imageRefValue("test-1")},
						{From: imageRefValue("test-2")},
						{From: imageRefValue("test-3")},
					}},
				},
				oldSpec: &buildapi.CommonSpec{
					Source: buildapi.BuildSource{Images: []buildapi.ImageSource{
						{From: imageRefValue("test-1")},
						{From: imageRefValue("test-3")},
					}},
				},
			},
			args: args{fn: func(ref *kapi.ObjectReference) error {
				if ref.Name != "test-2" {
					t.Errorf("did not expect to be called for existing reference")
				}
				ref.Name = "test-4"
				return nil
			}},
			wantSpec: &buildapi.CommonSpec{
				Source: buildapi.BuildSource{Images: []buildapi.ImageSource{
					{From: imageRefValue("test-1")},
					{From: imageRefValue("test-4")},
					{From: imageRefValue("test-3")},
				}},
			},
		},
		{
			name: "skips when docker reference unchanged",
			fields: fields{
				spec: &buildapi.CommonSpec{
					Strategy: buildapi.BuildStrategy{
						DockerStrategy: &buildapi.DockerBuildStrategy{From: imageRef("test")},
					},
				},
				oldSpec: &buildapi.CommonSpec{
					Strategy: buildapi.BuildStrategy{
						DockerStrategy: &buildapi.DockerBuildStrategy{From: imageRef("test")},
					},
				},
			},
			args: args{fn: func(ref *kapi.ObjectReference) error {
				t.Errorf("should not have called mutator")
				return nil
			}},
			wantSpec: &buildapi.CommonSpec{
				Strategy: buildapi.BuildStrategy{
					DockerStrategy: &buildapi.DockerBuildStrategy{From: imageRef("test")},
				},
			},
		},
		{
			name: "skips when custom reference unchanged",
			fields: fields{
				spec: &buildapi.CommonSpec{
					Strategy: buildapi.BuildStrategy{
						CustomStrategy: &buildapi.CustomBuildStrategy{From: imageRefValue("test")},
					},
				},
				oldSpec: &buildapi.CommonSpec{
					Strategy: buildapi.BuildStrategy{
						CustomStrategy: &buildapi.CustomBuildStrategy{From: imageRefValue("test")},
					},
				},
			},
			args: args{fn: func(ref *kapi.ObjectReference) error {
				t.Errorf("should not have called mutator")
				return nil
			}},
			wantSpec: &buildapi.CommonSpec{
				Strategy: buildapi.BuildStrategy{
					CustomStrategy: &buildapi.CustomBuildStrategy{From: imageRefValue("test")},
				},
			},
		},
		{
			name: "skips when source reference unchanged",
			fields: fields{
				spec: &buildapi.CommonSpec{
					Strategy: buildapi.BuildStrategy{
						SourceStrategy: &buildapi.SourceBuildStrategy{From: imageRefValue("test")},
					},
				},
				oldSpec: &buildapi.CommonSpec{
					Strategy: buildapi.BuildStrategy{
						SourceStrategy: &buildapi.SourceBuildStrategy{From: imageRefValue("test")},
					},
				},
			},
			args: args{fn: func(ref *kapi.ObjectReference) error {
				t.Errorf("should not have called mutator")
				return nil
			}},
			wantSpec: &buildapi.CommonSpec{
				Strategy: buildapi.BuildStrategy{
					SourceStrategy: &buildapi.SourceBuildStrategy{From: imageRefValue("test")},
				},
			},
		},
		{
			name: "skips when source reference unchanged",
			fields: fields{
				spec: &buildapi.CommonSpec{
					Strategy: buildapi.BuildStrategy{
						SourceStrategy: &buildapi.SourceBuildStrategy{
							From: imageRefValue("test"),
						},
					},
				},
				oldSpec: &buildapi.CommonSpec{
					Strategy: buildapi.BuildStrategy{
						SourceStrategy: &buildapi.SourceBuildStrategy{
							From: imageRefValue("test"),
						},
					},
				},
			},
			args: args{fn: func(ref *kapi.ObjectReference) error {
				t.Errorf("should not have called mutator")
				return nil
			}},
			wantSpec: &buildapi.CommonSpec{
				Strategy: buildapi.BuildStrategy{
					SourceStrategy: &buildapi.SourceBuildStrategy{
						From: imageRefValue("test"),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &buildSpecMutator{
				spec:    tt.fields.spec,
				oldSpec: tt.fields.oldSpec,
				path:    tt.fields.path,
				output:  tt.fields.output,
			}
			if tt.wantSpec == nil {
				tt.wantSpec = &buildapi.CommonSpec{}
			}
			if got := m.Mutate(tt.args.fn); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildSpecMutator.Mutate() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(tt.wantSpec, tt.fields.spec) {
				t.Errorf("buildSpecMutator.Mutate() spec = %#v, want %#v", tt.fields.spec, tt.wantSpec)
			}
		})
	}
}

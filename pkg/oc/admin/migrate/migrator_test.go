package migrate

import (
	"fmt"
	"net/http"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/kubectl/resource"
)

func TestIsNotFoundForInfo(t *testing.T) {
	type args struct {
		info *resource.Info
		err  error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "nil err does not match",
			args: args{
				info: nil,
				err:  nil,
			},
			want: false,
		},
		{
			name: "simple not found match",
			args: args{
				info: &resource.Info{
					Mapping: &meta.RESTMapping{
						GroupVersionKind: schema.GroupVersionKind{
							Group: "group1",
							Kind:  "kind1",
						},
					},
					Name: "name1",
				},
				err: errors.NewNotFound(schema.GroupResource{
					Group:    "group1",
					Resource: "kind1", // this is the kind
				},
					"name1",
				),
			},
			want: true,
		},
		{
			name: "simple not found match from generic 404 response",
			args: args{
				info: &resource.Info{
					Mapping: &meta.RESTMapping{
						GroupVersionKind: schema.GroupVersionKind{
							Group: "group1",
							Kind:  "kind1",
						},
					},
					Name: "name1",
				},
				err: errors.NewGenericServerResponse(
					http.StatusNotFound,
					"",
					schema.GroupResource{
						Group:    "group1",
						Resource: "kind1", // this is the kind
					},
					"name1",
					"",
					0,
					false,
				),
			},
			want: true,
		},
		{
			name: "simple not match from generic 400 response",
			args: args{
				info: &resource.Info{
					Mapping: &meta.RESTMapping{
						GroupVersionKind: schema.GroupVersionKind{
							Group: "group1",
							Kind:  "kind1",
						},
					},
					Name: "name1",
				},
				err: errors.NewGenericServerResponse(
					http.StatusBadRequest,
					"",
					schema.GroupResource{
						Group:    "group1",
						Resource: "kind1", // this is the kind
					},
					"name1",
					"",
					0,
					false,
				),
			},
			want: false,
		},
		{
			name: "different status error does not match",
			args: args{
				info: &resource.Info{
					Mapping: &meta.RESTMapping{
						GroupVersionKind: schema.GroupVersionKind{
							Group: "group1",
							Kind:  "kind1",
						},
					},
					Name: "name1",
				},
				err: errors.NewAlreadyExists(schema.GroupResource{
					Group:    "group1",
					Resource: "kind1", // this is the kind
				},
					"name1",
				),
			},
			want: false,
		},
		{
			name: "non status error does not match",
			args: args{
				info: &resource.Info{
					Mapping: &meta.RESTMapping{
						GroupVersionKind: schema.GroupVersionKind{
							Group: "group1",
							Kind:  "kind1",
						},
					},
					Name: "name1",
				},
				err: fmt.Errorf("%v",
					schema.GroupVersionKind{
						Group: "group1",
						Kind:  "kind1",
					},
				),
			},
			want: false,
		},
		{
			name: "case-insensitive not found match",
			args: args{
				info: &resource.Info{
					Mapping: &meta.RESTMapping{
						GroupVersionKind: schema.GroupVersionKind{
							Group: "GROUPA",
							Kind:  "KINDB",
						},
					},
					Name: "NOTname",
				},
				err: errors.NewNotFound(schema.GroupResource{
					Group:    "groupA",
					Resource: "Kindb", // this is the kind
				},
					"notNAME",
				),
			},
			want: true,
		},
		{
			name: "case-insensitive not found match from generic 404 response",
			args: args{
				info: &resource.Info{
					Mapping: &meta.RESTMapping{
						GroupVersionKind: schema.GroupVersionKind{
							Group: "ThisGroup",
							Kind:  "HasKinds",
						},
					},
					Name: "AndAName",
				},
				err: errors.NewGenericServerResponse(
					http.StatusNotFound,
					"",
					schema.GroupResource{
						Group:    "thisgroup",
						Resource: "haskinds", // this is the kind
					},
					"andaname",
					"",
					0,
					false,
				),
			},
			want: true,
		},
		{
			name: "case-insensitive not found match, no group in info",
			args: args{
				info: &resource.Info{
					Mapping: &meta.RESTMapping{
						GroupVersionKind: schema.GroupVersionKind{
							Kind: "KINDB",
						},
					},
					Name: "NOTname",
				},
				err: errors.NewNotFound(schema.GroupResource{
					Group:    "groupA",
					Resource: "Kindb", // this is the kind
				},
					"notNAME",
				),
			},
			want: true,
		},
		{
			name: "case-insensitive not found match, no group in error",
			args: args{
				info: &resource.Info{
					Mapping: &meta.RESTMapping{
						GroupVersionKind: schema.GroupVersionKind{
							Group: "GROUPA",
							Kind:  "KINDB",
						},
					},
					Name: "NOTname",
				},
				err: errors.NewNotFound(schema.GroupResource{
					Resource: "Kindb", // this is the kind
				},
					"notNAME",
				),
			},
			want: true,
		},
		{
			name: "case-insensitive not match due to different groups",
			args: args{
				info: &resource.Info{
					Mapping: &meta.RESTMapping{
						GroupVersionKind: schema.GroupVersionKind{
							Group: "group1",
							Kind:  "KINDB",
						},
					},
					Name: "NOTname",
				},
				err: errors.NewNotFound(schema.GroupResource{
					Group:    "group2",
					Resource: "Kindb", // this is the kind
				},
					"notNAME",
				),
			},
			want: false,
		},
		{
			name: "case-insensitive not found match from generic 404 response, no group in info",
			args: args{
				info: &resource.Info{
					Mapping: &meta.RESTMapping{
						GroupVersionKind: schema.GroupVersionKind{
							Kind: "HasKinds",
						},
					},
					Name: "AndAName",
				},
				err: errors.NewGenericServerResponse(
					http.StatusNotFound,
					"",
					schema.GroupResource{
						Group:    "thisgroup",
						Resource: "haskinds", // this is the kind
					},
					"andaname",
					"",
					0,
					false,
				),
			},
			want: true,
		},
		{
			name: "case-insensitive not found match from generic 404 response, no group in error",
			args: args{
				info: &resource.Info{
					Mapping: &meta.RESTMapping{
						GroupVersionKind: schema.GroupVersionKind{
							Group: "ThisGroup",
							Kind:  "HasKinds",
						},
					},
					Name: "AndAName",
				},
				err: errors.NewGenericServerResponse(
					http.StatusNotFound,
					"",
					schema.GroupResource{
						Resource: "haskinds", // this is the kind
					},
					"andaname",
					"",
					0,
					false,
				),
			},
			want: true,
		},
		{
			name: "case-insensitive not match from generic 404 response due to different groups",
			args: args{
				info: &resource.Info{
					Mapping: &meta.RESTMapping{
						GroupVersionKind: schema.GroupVersionKind{
							Group: "thingA",
							Kind:  "HasKinds",
						},
					},
					Name: "AndAName",
				},
				err: errors.NewGenericServerResponse(
					http.StatusNotFound,
					"",
					schema.GroupResource{
						Group:    "thingB",
						Resource: "haskinds", // this is the kind
					},
					"andaname",
					"",
					0,
					false,
				),
			},
			want: false,
		},
		{
			name: "case-insensitive match due to different kinds but same resource",
			args: args{
				info: &resource.Info{
					Mapping: &meta.RESTMapping{
						Resource: "KIND2",
						GroupVersionKind: schema.GroupVersionKind{
							Group: "group1",
							Kind:  "kind1",
						},
					},
					Name: "NOTname",
				},
				err: errors.NewNotFound(schema.GroupResource{
					Group:    "GROUP1",
					Resource: "kind2", // this is the kind
				},
					"notNAME",
				),
			},
			want: true,
		},
		{
			name: "case-insensitive match due to different resource but same kinds",
			args: args{
				info: &resource.Info{
					Mapping: &meta.RESTMapping{
						Resource: "kind1",
						GroupVersionKind: schema.GroupVersionKind{
							Group: "group1",
							Kind:  "KIND2",
						},
					},
					Name: "NOTname",
				},
				err: errors.NewNotFound(schema.GroupResource{
					Group:    "GROUP1",
					Resource: "kind2", // this is the kind
				},
					"notNAME",
				),
			},
			want: true,
		},
		{
			name: "case-insensitive not match due to different resource and different kinds",
			args: args{
				info: &resource.Info{
					Mapping: &meta.RESTMapping{
						Resource: "kind1",
						GroupVersionKind: schema.GroupVersionKind{
							Group: "group1",
							Kind:  "kind3",
						},
					},
					Name: "NOTname",
				},
				err: errors.NewNotFound(schema.GroupResource{
					Group:    "GROUP1",
					Resource: "kind2", // this is the kind
				},
					"notNAME",
				),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNotFoundForInfo(tt.args.info, tt.args.err); got != tt.want {
				t.Errorf("isNotFoundForInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}

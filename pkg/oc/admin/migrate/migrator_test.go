package migrate

import (
	"flag"
	"fmt"
	"net/http"
	"runtime"
	"sync"
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

// TestResourceVisitor_Visit is used to check for race conditions
func TestResourceVisitor_Visit(t *testing.T) {
	// set glog levels so we write to Out
	flag.CommandLine.Lookup("v").Value.Set("2")
	flag.CommandLine.Lookup("stderrthreshold").Value.Set("INFO")

	type fields struct {
		Out      mapWriter
		Builder  testBuilder
		SaveFn   *countSaveFn
		PrintFn  MigrateActionFunc
		FilterFn MigrateFilterFunc
		DryRun   bool
		Workers  int
	}
	type args struct {
		fn MigrateVisitFunc
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "migrate storage race detection",
			fields: fields{
				Out:      make(mapWriter),       // detect writes via multiple goroutines
				Builder:  testBuilder(5000),     // send a specific amount of infos to SaveFn
				SaveFn:   new(countSaveFn),      // make sure we process all resources
				PrintFn:  nil,                   // must be nil to use SaveFn
				FilterFn: nil,                   // we want no filtering
				DryRun:   false,                 // must be false to use SaveFn
				Workers:  32 * runtime.NumCPU(), // same as migrate storage
			},
			args: args{
				fn: AlwaysRequiresMigration, // same as migrate storage
			},
			wantErr: false, // should never error
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &ResourceVisitor{
				Out:      tt.fields.Out,
				Builder:  tt.fields.Builder,
				SaveFn:   tt.fields.SaveFn.save,
				PrintFn:  tt.fields.PrintFn,
				FilterFn: tt.fields.FilterFn,
				DryRun:   tt.fields.DryRun,
				Workers:  tt.fields.Workers,
			}
			// how many infos are we expected to process
			expectedInfos := int(tt.fields.Builder)
			// countSaveFn will spawn one goroutine per info it sees
			tt.fields.SaveFn.w.Add(expectedInfos)
			// process the infos
			if err := o.Visit(tt.args.fn); (err != nil) != tt.wantErr {
				t.Errorf("ResourceVisitor.Visit() error = %v, wantErr %v", err, tt.wantErr)
			}
			// wait for all countSaveFn goroutines to finish
			tt.fields.SaveFn.w.Wait()
			// check that we saw the correct amount of infos throughout
			writes := len(tt.fields.Out) - 1 // minus one for the summary output
			saves := tt.fields.SaveFn.n
			if expectedInfos != writes || expectedInfos != saves {
				t.Errorf("ResourceVisitor.Visit() incorrect counts seen, expectedInfos=%d writes=%d saves=%d out=%v",
					expectedInfos, writes, saves, tt.fields.Out)
			}
		})
	}
}

// mapWriter is an io.Writer that is guaranteed to panic if accessed via multiple goroutines at the same time
type mapWriter map[int]string

func (m mapWriter) Write(p []byte) (n int, err error) {
	l := len(m)      // makes it easy to track how many times Write is called
	m[l] = string(p) // string for debugging
	return len(p), nil
}

// countSaveFn is used to build a MigrateActionFunc (SaveFn) that records how many times it was called
// goroutine safe
type countSaveFn struct {
	w sync.WaitGroup
	m sync.Mutex
	n int
}

func (c *countSaveFn) save(_ *resource.Info, _ Reporter) error {
	// do not block workers on the mutex, we do not want to accidentally serialize our code in a way that masks race conditions
	go func() {
		c.m.Lock()
		c.n++
		c.m.Unlock()
		c.w.Done()
	}()
	return nil
}

// testBuilder emits a resource.Visitor that calls resource.VisitorFunc n times
type testBuilder int

func (t testBuilder) Visitor(_ ...resource.ErrMatchFunc) (resource.Visitor, error) {
	infos := make(resource.InfoListVisitor, t) // the resource.VisitorFunc will be called t times
	for i := range infos {
		infos[i] = &resource.Info{Mapping: &meta.RESTMapping{}} // just enough to prevent NPEs
	}
	return infos, nil
}

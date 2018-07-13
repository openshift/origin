package migrate

import (
	"flag"
	"runtime"
	"sync"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/resource"
)

// TestResourceVisitor_Visit is used to check for race conditions
func TestResourceVisitor_Visit(t *testing.T) {
	// get log level flag for glog
	verbosity := flag.CommandLine.Lookup("v").Value
	// save its original value
	origVerbosity := verbosity.String()
	// set log level high enough so we write to ResourceOptions.Out on each success
	verbosity.Set("1")
	// restore the original flag value when we return
	defer func() {
		verbosity.Set(origVerbosity)
	}()

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

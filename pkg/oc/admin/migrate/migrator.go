package migrate

import (
	"fmt"
	"io"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/kubectl"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	kprinters "k8s.io/kubernetes/pkg/printers"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

// MigrateVisitFunc is invoked for each returned object, and may return a
// Reporter that can contain info to be used by save.
type MigrateVisitFunc func(info *resource.Info) (Reporter, error)

// MigrateActionFunc is expected to persist the altered info.Object. The
// Reporter returned from Visit is passed to this function and may be used
// to carry additional information about what to save on an object.
type MigrateActionFunc func(info *resource.Info, reporter Reporter) error

// MigrateFilterFunc can return false to skip an item, or an error.
type MigrateFilterFunc func(info *resource.Info) (bool, error)

// Reporter indicates whether a resource requires migration.
type Reporter interface {
	// Changed returns true if the resource requires migration.
	Changed() bool
}

// ReporterBool implements the Reporter interface for a boolean.
type ReporterBool bool

func (r ReporterBool) Changed() bool {
	return bool(r)
}

func AlwaysRequiresMigration(_ *resource.Info) (Reporter, error) {
	return ReporterBool(true), nil
}

// NotChanged is a Reporter returned by operations that are guaranteed to be read-only
var NotChanged = ReporterBool(false)

// ResourceOptions assists in performing migrations on any object that
// can be retrieved via the API.
type ResourceOptions struct {
	In          io.Reader
	Out, ErrOut io.Writer

	AllNamespaces bool
	Include       []string
	Filenames     []string
	Confirm       bool
	Output        string
	FromKey       string
	ToKey         string

	OverlappingResources []sets.String
	DefaultExcludes      []schema.GroupResource

	Builder   *resource.Builder
	SaveFn    MigrateActionFunc
	PrintFn   MigrateActionFunc
	FilterFn  MigrateFilterFunc
	DryRun    bool
	Summarize bool
}

func (o *ResourceOptions) Bind(c *cobra.Command) {
	c.Flags().StringVarP(&o.Output, "output", "o", o.Output, "Output the modified objects instead of saving them, valid values are 'yaml' or 'json'")
	kcmdutil.AddNoHeadersFlags(c)
	c.Flags().StringSliceVar(&o.Include, "include", o.Include, "Resource types to migrate. Passing --filename will override this flag.")
	c.Flags().BoolVar(&o.AllNamespaces, "all-namespaces", true, "Migrate objects in all namespaces. Defaults to true.")
	c.Flags().BoolVar(&o.Confirm, "confirm", false, "If true, all requested objects will be migrated. Defaults to false.")

	c.Flags().StringVar(&o.FromKey, "from-key", o.FromKey, "If specified, only migrate items with a key (namespace/name or name) greater than or equal to this value")
	c.Flags().StringVar(&o.ToKey, "to-key", o.ToKey, "If specified, only migrate items with a key (namespace/name or name) less than this value")

	usage := "Filename, directory, or URL to docker-compose.yml file to use"
	kubectl.AddJsonFilenameFlag(c, &o.Filenames, usage)
	c.MarkFlagRequired("filename")
}

func (o *ResourceOptions) Complete(f *clientcmd.Factory, c *cobra.Command) error {
	switch {
	case len(o.Output) > 0:
		printer, err := f.PrinterForCommand(c, false, nil, kprinters.PrintOptions{})
		if err != nil {
			return err
		}
		first := true
		o.PrintFn = func(info *resource.Info, _ Reporter) error {
			obj, err := info.Mapping.ConvertToVersion(info.Object, info.Mapping.GroupVersionKind.GroupVersion())
			if err != nil {
				return err
			}
			// TODO: PrintObj is not correct for YAML - it should inject document separators itself
			if o.Output == "yaml" && !first {
				fmt.Fprintln(o.Out, "---")
			}
			first = false
			printer.PrintObj(obj, o.Out)
			return nil
		}
		o.DryRun = true
	case o.Confirm:
		o.DryRun = false
	default:
		o.DryRun = true
	}

	namespace, explicitNamespace, err := f.DefaultNamespace()
	if err != nil {
		return err
	}
	allNamespaces := !explicitNamespace && o.AllNamespaces

	if len(o.FromKey) > 0 || len(o.ToKey) > 0 {
		o.FilterFn = func(info *resource.Info) (bool, error) {
			var key string
			if info.Mapping.Scope.Name() == meta.RESTScopeNameNamespace {
				key = info.Namespace + "/" + info.Name
			} else {
				if !allNamespaces {
					return false, nil
				}
				key = info.Name
			}
			if len(o.FromKey) > 0 && o.FromKey > key {
				return false, nil
			}
			if len(o.ToKey) > 0 && o.ToKey <= key {
				return false, nil
			}
			return true, nil
		}
	}

	oclient, _, err := f.Clients()
	if err != nil {
		return err
	}
	mapper, _ := f.Object()

	resourceNames := sets.NewString()
	for i, s := range o.Include {
		if resourceNames.Has(s) {
			continue
		}
		if s != "*" {
			resourceNames.Insert(s)
			break
		}

		all, err := clientcmd.FindAllCanonicalResources(oclient.Discovery(), mapper)
		if err != nil {
			return fmt.Errorf("could not calculate the list of available resources: %v", err)
		}
		exclude := sets.NewString()
		for _, gr := range o.DefaultExcludes {
			if len(o.OverlappingResources) > 0 {
				for _, others := range o.OverlappingResources {
					if !others.Has(gr.String()) {
						continue
					}
					exclude.Insert(others.List()...)
					break
				}
			}
			exclude.Insert(gr.String())
		}
		candidate := sets.NewString()
		for _, gr := range all {
			// if the user specifies a resource that matches resource or resource+group, skip it
			if resourceNames.Has(gr.Resource) || resourceNames.Has(gr.String()) || exclude.Has(gr.String()) {
				continue
			}
			candidate.Insert(gr.String())
		}
		candidate.Delete(exclude.List()...)
		include := candidate
		if len(o.OverlappingResources) > 0 {
			include = sets.NewString()
			for _, k := range candidate.List() {
				reduce := k
				for _, others := range o.OverlappingResources {
					if !others.Has(k) {
						continue
					}
					reduce = others.List()[0]
					break
				}
				include.Insert(reduce)
			}
		}
		glog.V(4).Infof("Found the following resources from the server: %v", include.List())
		last := o.Include[i+1:]
		o.Include = append([]string{}, o.Include[:i]...)
		o.Include = append(o.Include, include.List()...)
		o.Include = append(o.Include, last...)
		break
	}

	o.Builder = f.NewBuilder(true).
		AllNamespaces(allNamespaces).
		FilenameParam(false, &resource.FilenameOptions{Recursive: false, Filenames: o.Filenames}).
		ContinueOnError().
		DefaultNamespace().
		RequireObject(true).
		SelectAllParam(true).
		Flatten()
	if !allNamespaces {
		o.Builder.NamespaceParam(namespace)
	}
	if len(o.Filenames) == 0 {
		o.Builder.ResourceTypes(o.Include...)
	}
	return nil
}

func (o *ResourceOptions) Validate() error {
	if len(o.Filenames) == 0 && len(o.Include) == 0 {
		return fmt.Errorf("you must specify at least one resource or resource type to migrate with --include or --filenames")
	}
	return nil
}

func (o *ResourceOptions) Visitor() *ResourceVisitor {
	return &ResourceVisitor{
		Out:      o.Out,
		Builder:  o.Builder,
		SaveFn:   o.SaveFn,
		PrintFn:  o.PrintFn,
		FilterFn: o.FilterFn,
		DryRun:   o.DryRun,
	}
}

type ResourceVisitor struct {
	Out io.Writer

	Builder *resource.Builder

	SaveFn   MigrateActionFunc
	PrintFn  MigrateActionFunc
	FilterFn MigrateFilterFunc

	DryRun bool
}

func (o *ResourceVisitor) Visit(fn MigrateVisitFunc) error {
	dryRun := o.DryRun
	summarize := true
	actionFn := o.SaveFn
	switch {
	case o.PrintFn != nil:
		actionFn = o.PrintFn
		dryRun = true
		summarize = false
	case dryRun:
		actionFn = nil
	}
	out := o.Out

	result := o.Builder.Do()
	if result.Err() != nil {
		return result.Err()
	}

	// Ignore any resource that does not support GET
	result.IgnoreErrors(errors.IsMethodNotSupported, errors.IsNotFound)

	t := migrateTracker{
		out:       out,
		migrateFn: fn,
		actionFn:  actionFn,
		dryRun:    dryRun,

		resourcesWithErrors: sets.NewString(),
	}

	err := result.Visit(func(info *resource.Info, err error) error {
		if err == nil && o.FilterFn != nil {
			var ok bool
			if ok, err = o.FilterFn(info); err == nil && !ok {
				t.found++
				t.ignored++
				if glog.V(2) {
					t.report("ignored:", info, nil)
				}
				return nil
			}
		}
		if err != nil {
			t.resourcesWithErrors.Insert(info.Mapping.Resource)
			t.errors++
			t.report("error:", info, err)
			return nil
		}
		t.attempt(info, 10)
		return nil
	})

	if summarize {
		if dryRun {
			fmt.Fprintf(out, "summary (dry run): total=%d errors=%d ignored=%d unchanged=%d migrated=%d\n", t.found, t.errors, t.ignored, t.unchanged, t.found-t.errors-t.unchanged-t.ignored)
		} else {
			fmt.Fprintf(out, "summary: total=%d errors=%d ignored=%d unchanged=%d migrated=%d\n", t.found, t.errors, t.ignored, t.unchanged, t.found-t.errors-t.unchanged-t.ignored)
		}
	}

	if t.resourcesWithErrors.Len() > 0 {
		fmt.Fprintf(out, "info: to rerun only failing resources, add --include=%s\n", strings.Join(t.resourcesWithErrors.List(), ","))
	}

	switch {
	case err != nil:
		fmt.Fprintf(out, "error: exited without processing all resources: %v\n", err)
		err = kcmdutil.ErrExit
	case t.errors > 0:
		fmt.Fprintf(out, "error: %d resources failed to migrate\n", t.errors)
		err = kcmdutil.ErrExit
	}
	return err
}

// ErrUnchanged may be returned by MigrateActionFunc to indicate that the object
// did not need migration (but that could only be determined when the action was taken).
var ErrUnchanged = fmt.Errorf("migration was not necessary")

// ErrRecalculate may be returned by MigrateActionFunc to indicate that the object
// has changed and needs to have its information recalculated prior to being saved.
// Use when a resource requires multiple API operations to persist (for instance,
// both status and spec must be changed).
var ErrRecalculate = fmt.Errorf("recalculate migration")

// ErrRetriable is a wrapper for an error that a migrator may use to indicate the
// specific error can be retried.
type ErrRetriable struct {
	error
}

func (ErrRetriable) Temporary() bool { return true }

// ErrNotRetriable is a wrapper for an error that a migrator may use to indicate the
// specific error cannot be retried.
type ErrNotRetriable struct {
	error
}

func (ErrNotRetriable) Temporary() bool { return false }

type temporary interface {
	// Temporary should return true if this is a temporary error
	Temporary() bool
}

// attemptResult is an enumeration of the result of a migration
type attemptResult int

const (
	attemptResultSuccess attemptResult = iota
	attemptResultError
	attemptResultUnchanged
	attemptResultIgnore
)

// migrateTracker abstracts transforming and saving resources and can be used to keep track
// of how many total resources have been updated.
type migrateTracker struct {
	out       io.Writer
	migrateFn MigrateVisitFunc
	actionFn  MigrateActionFunc
	dryRun    bool

	found, ignored, unchanged, errors int
	retries                           int

	resourcesWithErrors sets.String
}

// report prints a message to out that includes info about the current resource. If the optional error is
// provided it will be written as well.
func (t *migrateTracker) report(prefix string, info *resource.Info, err error) {
	ns := info.Namespace
	if len(ns) > 0 {
		ns = "-n " + ns
	}
	if err != nil {
		fmt.Fprintf(t.out, "%-10s %s %s/%s: %v\n", prefix, ns, info.Mapping.Resource, info.Name, err)
	} else {
		fmt.Fprintf(t.out, "%-10s %s %s/%s\n", prefix, ns, info.Mapping.Resource, info.Name)
	}
}

// attempt will try to invoke the migrateFn and saveFn on info, retrying any recalculation requests up
// to retries times.
func (t *migrateTracker) attempt(info *resource.Info, retries int) {
	t.found++
	t.retries = retries
	result, err := t.try(info)
	switch {
	case err != nil:
		t.resourcesWithErrors.Insert(info.Mapping.Resource)
		t.errors++
		t.report("error:", info, err)
	case result == attemptResultIgnore:
		t.ignored++
		if glog.V(2) {
			t.report("ignored:", info, nil)
		}
	case result == attemptResultUnchanged:
		t.unchanged++
		if glog.V(2) {
			t.report("unchanged:", info, nil)
		}
	case result == attemptResultSuccess:
		if glog.V(1) {
			if t.dryRun {
				t.report("migrated (dry run):", info, nil)
			} else {
				t.report("migrated:", info, nil)
			}
		}
	}
}

// try will mutate the info and attempt to save, recalculating if there are any retries left.
// The result of the attempt or an error will be returned.
func (t *migrateTracker) try(info *resource.Info) (attemptResult, error) {
	reporter, err := t.migrateFn(info)
	if err != nil {
		return attemptResultError, err
	}
	if reporter == nil {
		return attemptResultIgnore, nil
	}
	if !reporter.Changed() {
		return attemptResultUnchanged, nil
	}
	if t.actionFn != nil {
		if err := t.actionFn(info, reporter); err != nil {
			if err == ErrUnchanged {
				return attemptResultUnchanged, nil
			}
			if canRetry(err) {
				if t.retries > 0 {
					if bool(glog.V(1)) && err != ErrRecalculate {
						t.report("retry:", info, err)
					}
					result, err := t.try(info)
					switch result {
					case attemptResultUnchanged, attemptResultIgnore:
						result = attemptResultSuccess
					}
					return result, err
				}
			}
			return attemptResultError, err
		}
	}
	return attemptResultSuccess, nil
}

// canRetry returns true if the provided error indicates a retry is possible.
func canRetry(err error) bool {
	if temp, ok := err.(temporary); ok && temp.Temporary() {
		return true
	}
	return err == ErrRecalculate
}

// DefaultRetriable adds retry information to the provided error, and will refresh the
// info if the client info is stale. If the refresh fails the error is made fatal.
// All other errors are left in their natural state - they will not be retried unless
// they define a Temporary() method that returns true.
func DefaultRetriable(info *resource.Info, err error) error {
	// tolerate the deletion of resources during migration
	if err == nil || errors.IsNotFound(err) {
		return nil
	}
	switch {
	case errors.IsMethodNotSupported(err):
		return ErrNotRetriable{err}
	case errors.IsConflict(err):
		if refreshErr := info.Get(); refreshErr != nil {
			return ErrNotRetriable{err}
		}
		return ErrRetriable{err}
	case errors.IsServerTimeout(err):
		return ErrRetriable{err}
	}
	return err
}

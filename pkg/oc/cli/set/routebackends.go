package set

import (
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/resource"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	routev1 "github.com/openshift/api/route/v1"
)

var (
	backendsLong = templates.LongDesc(`
		Set and adjust route backends

		Routes may have one or more optional backend services with weights controlling how much
		traffic flows to each service. Traffic is assigned proportional to the combined weights
		of each backend. A weight of zero means that the backend will receive no traffic. If all
		weights are zero the route will not send traffic to any backends.

		When setting backends, the first backend is the primary and the other backends are
		considered alternates. For example:

		    $ %[1]s route-backends web prod=99 canary=1

		will set the primary backend to service "prod" with a weight of 99 and the first
		alternate backend to service "canary" with a weight of 1. This means 99%% of traffic will
		be sent to the service "prod".

		The --adjust flag allows you to alter the weight of an individual service relative to
		itself or to the primary backend. Specifying a percentage will adjust the backend
		relative to either the primary or the first alternate (if you specify the primary).
		If there are other backends their weights will be kept proportional to the changed.

		Not all routers may support multiple or weighted backends.`)

	backendsExample = templates.Examples(`
		# Print the backends on the route 'web'
	  %[1]s route-backends web

	  # Set two backend services on route 'web' with 2/3rds of traffic going to 'a'
	  %[1]s route-backends web a=2 b=1

	  # Increase the traffic percentage going to b by 10%% relative to a
	  %[1]s route-backends web --adjust b=+10%%

	  # Set traffic percentage going to b to 10%% of the traffic going to a
	  %[1]s route-backends web --adjust b=10%%

	  # Set weight of b to 10
	  %[1]s route-backends web --adjust b=10

	  # Set the weight to all backends to zero
	  %[1]s route-backends web --zero`)
)

type BackendsOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	Selector   string
	All        bool
	Local      bool
	PrintTable bool
	Transform  BackendTransform

	Mapper            meta.RESTMapper
	Client            dynamic.Interface
	Printer           printers.ResourcePrinter
	Builder           func() *resource.Builder
	Namespace         string
	ExplicitNamespace bool
	DryRun            bool
	Resources         []string

	resource.FilenameOptions
	genericclioptions.IOStreams
}

func NewBackendsOptions(streams genericclioptions.IOStreams) *BackendsOptions {
	return &BackendsOptions{
		PrintFlags: genericclioptions.NewPrintFlags("backends updated").WithTypeSetter(scheme.Scheme),
		IOStreams:  streams,
	}
}

// NewCmdRouteBackends implements the set route-backends command
func NewCmdRouteBackends(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewBackendsOptions(streams)
	cmd := &cobra.Command{
		Use:     "route-backends ROUTENAME [--zero|--equal] [--adjust] SERVICE=WEIGHT[%] [...]",
		Short:   "Update the backends for a route",
		Long:    fmt.Sprintf(backendsLong, fullName),
		Example: fmt.Sprintf(backendsExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}
	usage := "to use to edit the resource"
	kcmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, usage)
	cmd.Flags().StringVarP(&o.Selector, "selector", "l", o.Selector, "Selector (label query) to filter on")
	cmd.Flags().BoolVar(&o.All, "all", o.All, "If true, select all resources in the namespace of the specified resource types")
	cmd.Flags().BoolVar(&o.Local, "local", o.Local, "If true, set image will NOT contact api-server but run locally.")
	cmd.Flags().BoolVar(&o.Transform.Adjust, "adjust", o.Transform.Adjust, "Adjust a single backend using an absolute or relative weight. If the primary backend is selected and there is more than one alternate an error will be returned.")
	cmd.Flags().BoolVar(&o.Transform.Zero, "zero", o.Transform.Zero, "If true, set the weight of all backends to zero.")
	cmd.Flags().BoolVar(&o.Transform.Equal, "equal", o.Transform.Equal, "If true, set the weight of all backends to 100.")

	o.PrintFlags.AddFlags(cmd)
	kcmdutil.AddDryRunFlag(cmd)

	return cmd
}

// Complete takes command line information to fill out BackendOptions or returns an error.
func (o *BackendsOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	var err error
	o.Namespace, o.ExplicitNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	for _, arg := range args {
		if !strings.Contains(arg, "=") {
			o.Resources = append(o.Resources, arg)
			continue
		}
		input, err := ParseBackendInput(arg)
		if err != nil {
			return fmt.Errorf("invalid argument %q: %v", arg, err)
		}
		o.Transform.Inputs = append(o.Transform.Inputs, *input)
	}

	o.PrintTable = o.Transform.Empty()

	o.DryRun = kcmdutil.GetDryRunFlag(cmd)
	o.Mapper, err = f.ToRESTMapper()
	if err != nil {
		return err
	}
	o.Builder = f.NewBuilder

	if o.DryRun {
		o.PrintFlags.Complete("%s (dry run)")
	}
	o.Printer, err = o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.Client, err = dynamic.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	return nil
}

// Validate verifies the provided options are valid or returns an error.
func (o *BackendsOptions) Validate() error {
	return o.Transform.Validate()
}

// Run executes the BackendOptions or returns an error.
func (o *BackendsOptions) Run() error {
	b := o.Builder().
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		LocalParam(o.Local).
		ContinueOnError().
		NamespaceParam(o.Namespace).DefaultNamespace().
		FilenameParam(o.ExplicitNamespace, &o.FilenameOptions).
		Flatten()
	if !o.Local {
		b = b.
			LabelSelectorParam(o.Selector).
			SelectAllParam(o.All).
			ResourceNames("route", o.Resources...).
			Latest()
		if len(o.Resources) == 0 {
			b = b.ResourceTypes("routes")
		}
	}

	singleItemImplied := false
	infos, err := b.Do().IntoSingleItemImplied(&singleItemImplied).Infos()
	if err != nil {
		return err
	}

	if o.PrintTable {
		return o.printBackends(infos)
	}

	patches := CalculatePatchesExternal(infos, func(info *resource.Info) (bool, error) {
		return UpdateBackendsForObject(info.Object, o.Transform.Apply)
	})
	if singleItemImplied && len(patches) == 0 {
		name := infos[0].Name
		if infos[0].Mapping != nil {
			name = fmt.Sprintf("%s/%s", infos[0].Mapping.Resource.Resource, infos[0].Name)
		}
		return fmt.Errorf("%s is not a deployment config or build config", name)
	}

	allErrs := []error{}
	for _, patch := range patches {
		info := patch.Info
		name := getObjectName(info)
		if patch.Err != nil {
			allErrs = append(allErrs, fmt.Errorf("error: %s %v\n", name, patch.Err))
			continue
		}

		if string(patch.Patch) == "{}" || len(patch.Patch) == 0 {
			glog.V(1).Infof("info: %s was not changed\n", name)
			continue
		}

		if o.Local || o.DryRun {
			if err := o.Printer.PrintObj(info.Object, o.Out); err != nil {
				allErrs = append(allErrs, err)
			}
			continue
		}

		actual, err := o.Client.Resource(info.Mapping.Resource).Namespace(info.Namespace).Patch(info.Name, types.StrategicMergePatchType, patch.Patch)
		if err != nil {
			allErrs = append(allErrs, fmt.Errorf("failed to patch route backends: %v\n", err))
			continue
		}

		if err := o.Printer.PrintObj(actual, o.Out); err != nil {
			allErrs = append(allErrs, err)
		}
	}
	return utilerrors.NewAggregate(allErrs)
}

// printBackends displays a tabular output of the backends for each object.
func (o *BackendsOptions) printBackends(infos []*resource.Info) error {
	w := tabwriter.NewWriter(o.Out, 0, 2, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintf(w, "NAME\tKIND\tTO\tWEIGHT\n")
	for _, info := range infos {
		_, err := UpdateBackendsForObject(info.Object, func(backends *Backends) error {
			totalWeight := int32(0)
			for _, b := range backends.Backends {
				if b.Weight != nil {
					totalWeight += *b.Weight
				}
			}
			for _, b := range backends.Backends {
				switch {
				case b.Weight == nil:
					fmt.Fprintf(w, "%s/%s\t%s\t%s\t%s\n", info.Mapping.Resource.Resource, info.Name, b.Kind, b.Name, "")
				case totalWeight == 0, len(backends.Backends) == 1 && totalWeight != 0:
					fmt.Fprintf(w, "%s/%s\t%s\t%s\t%d\n", info.Mapping.Resource.Resource, info.Name, b.Kind, b.Name, totalWeight)
				default:
					fmt.Fprintf(w, "%s/%s\t%s\t%s\t%d (%d%%)\n", info.Mapping.Resource.Resource, info.Name, b.Kind, b.Name, *b.Weight, *b.Weight*100/totalWeight)
				}
			}
			return nil
		})
		if err != nil {
			fmt.Fprintf(w, "%s/%s\t%s\t%s\t%d\n", info.Mapping.Resource.Resource, info.Name, "", "<error>", 0)
		}
	}
	return nil
}

// BackendTransform describes the desired transformation of backends.
type BackendTransform struct {
	// Adjust expects a single Input to transform, relative to other backends.
	Adjust bool
	// Zero sets all backend weights to zero.
	Zero bool
	// Equal means backends will be set to equal weights.
	Equal bool
	// Inputs is the desired backends.
	Inputs []BackendInput
}

// Empty returns true if no transformations have been specified.
func (t BackendTransform) Empty() bool {
	return !(t.Zero || t.Equal || len(t.Inputs) > 0)
}

// Validate returns an error if the transformations are not internally consistent.
func (t BackendTransform) Validate() error {
	switch {
	case t.Adjust:
		if t.Zero {
			return fmt.Errorf("--adjust and --zero may not be specified together")
		}
		if t.Equal {
			return fmt.Errorf("--adjust and --equal may not be specified together")
		}
		if len(t.Inputs) != 1 {
			return fmt.Errorf("only one backend may be specified when adjusting")
		}

	case t.Zero, t.Equal:
		if t.Equal && t.Zero {
			return fmt.Errorf("--zero and --equal may not be specified together")
		}
		if len(t.Inputs) > 0 {
			return fmt.Errorf("arguments may not be provided when --zero or --equal is specified")
		}

	default:
		percent := false
		names := sets.NewString()
		for i, input := range t.Inputs {
			if names.Has(input.Name) {
				return fmt.Errorf("backend name %q may only be specified once", input.Name)
			}
			names.Insert(input.Name)
			if input.Percentage {
				if !percent && i != 0 {
					return fmt.Errorf("all backends must either be percentages or weights")
				}
				percent = true
			}
			if input.Value < 0 {
				return fmt.Errorf("negative percentages are not allowed")
			}
		}
	}
	return nil
}

// Apply transforms the provided backends or returns an error.
func (t BackendTransform) Apply(b *Backends) error {
	switch {
	case t.Zero:
		zero := int32(0)
		for i := range b.Backends {
			b.Backends[i].Weight = &zero
		}

	case t.Equal:
		equal := int32(100)
		for i := range b.Backends {
			b.Backends[i].Weight = &equal
		}

	case t.Adjust:
		input := t.Inputs[0]
		switch {
		case len(b.Backends) == 0:
			return fmt.Errorf("no backends can be adjusted")
		case len(b.Backends) == 1:
			// treat adjusting primary specially
			backend := &b.Backends[0]
			if backend.Name != input.Name {
				return fmt.Errorf("backend %q is not in the list of backends (%s)", input.Name, strings.Join(b.Names(), ", "))
			}
			if input.Relative {
				return fmt.Errorf("cannot adjust a single backend by relative weight")
			}
			// ignore distinction between percentage and weight for single backend
			backend.Weight = &input.Value
		case b.Backends[0].Name == input.Name:
			// changing the primary backend, multiple available
			if len(b.Backends) == 1 {
				input.Apply(&b.Backends[0], nil, b.Backends)
				return nil
			}
			input.Apply(&b.Backends[0], &b.Backends[1], b.Backends)

		default:
			// changing an alternate backend, multiple available
			for i := range b.Backends {
				if b.Backends[i].Name != input.Name {
					continue
				}
				input.Apply(&b.Backends[i], &b.Backends[0], b.Backends)
				return nil
			}
			return fmt.Errorf("backend %q is not in the list of backends (%s)", input.Name, strings.Join(b.Names(), ", "))
		}

	default:
		b.Backends = nil
		for _, input := range t.Inputs {
			weight := input.Value
			b.Backends = append(b.Backends, routev1.RouteTargetReference{
				Kind:   "Service",
				Name:   input.Name,
				Weight: &weight,
			})
		}
	}
	return nil
}

// BackendInput describes a change to a named service.
type BackendInput struct {
	// Name is the name of a service.
	Name string
	// Value is the amount to change.
	Value int32
	// Percentage means value should be interpreted as a percentage between -100 and 100, inclusive.
	Percentage bool
	// Relative means value is applied relative to the current values.
	Relative bool
}

// Apply alters the weights of two services.
func (input *BackendInput) Apply(ref, to *routev1.RouteTargetReference, backends []routev1.RouteTargetReference) {
	weight := int32(100)
	if ref.Weight != nil {
		weight = *ref.Weight
	}
	switch {
	case input.Percentage:
		if to == nil {
			weight += (weight * input.Value) / 100
			ref.Weight = &weight
			return
		}

		otherWeight := int32(0)
		if to.Weight != nil {
			otherWeight = *to.Weight
		}
		previousWeight := weight + otherWeight

		// rebalance all other backends to be relative in weight to the current
		for i, other := range backends {
			if previousWeight == 0 || other.Weight == nil || other.Name == ref.Name || other.Name == to.Name {
				continue
			}
			adjusted := *other.Weight * 100 / previousWeight
			backends[i].Weight = &adjusted
		}

		// adjust the weight between ref and to
		target := float32(input.Value) / 100
		if input.Relative {
			if previousWeight != 0 {
				percent := float32(weight) / float32(previousWeight)
				target = percent + target
			}
		}
		switch {
		case target < 0:
			target = 0
		case target > 1:
			target = 1
		}
		weight = int32(target * 100)
		otherWeight = int32((1 - target) * 100)
		ref.Weight = &weight
		to.Weight = &otherWeight

		// rescale the max to 200 in case we are dealing with very small percentages
		max := int32(0)
		for _, other := range backends {
			if other.Weight == nil {
				continue
			}
			if *other.Weight > max {
				max = *other.Weight
			}
		}
		if max > 256 {
			for i, other := range backends {
				if other.Weight == nil || *other.Weight == 0 {
					continue
				}
				adjusted := 200 * *other.Weight / max
				if adjusted < 1 {
					adjusted = 1
				}
				backends[i].Weight = &adjusted
			}
		}

	case input.Relative:
		weight += input.Value
		if weight < 0 {
			weight = 0
		}
		ref.Weight = &weight

	default:
		ref.Weight = &input.Value
	}
}

// ParseBackendInput turns the provided input into a BackendInput or returns an error.
func ParseBackendInput(s string) (*BackendInput, error) {
	parts := strings.SplitN(s, "=", 2)
	switch {
	case len(parts) != 2, len(parts[0]) == 0, len(parts[1]) == 0:
		return nil, fmt.Errorf("expected NAME=WEIGHT")
	}

	if strings.Contains(parts[0], "/") {
		return nil, fmt.Errorf("only NAME=WEIGHT may be specified")
	}

	input := &BackendInput{}
	input.Name = parts[0]

	if strings.HasSuffix(parts[1], "%") {
		input.Percentage = true
		parts[1] = strings.TrimSuffix(parts[1], "%")
	}
	if strings.HasPrefix(parts[1], "+") {
		input.Relative = true
		parts[1] = strings.TrimPrefix(parts[1], "+")
	}
	value, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("WEIGHT must be a number: %v", err)
	}
	input.Value = int32(value)
	if input.Value < 0 {
		input.Relative = true
	}
	return input, nil
}

// Backends is a struct that represents the backends to be transformed.
type Backends struct {
	Backends []routev1.RouteTargetReference
}

// Names returns the referenced backend service names, in the order they appear.
func (b *Backends) Names() []string {
	var names []string
	for _, backend := range b.Backends {
		names = append(names, backend.Name)
	}
	return names
}

// UpdateBackendsForObject extracts a backend definition array from the provided object, passes it to fn,
// and then applies the backend on the object. It returns true if the object was mutated and an optional error
// if any part of the flow returns error.
func UpdateBackendsForObject(obj runtime.Object, fn func(*Backends) error) (bool, error) {
	// TODO: replace with a swagger schema based approach (identify pod template via schema introspection)
	switch t := obj.(type) {
	case *routev1.Route:
		b := &Backends{
			Backends: []routev1.RouteTargetReference{t.Spec.To},
		}
		for _, backend := range t.Spec.AlternateBackends {
			b.Backends = append(b.Backends, backend)
		}
		if err := fn(b); err != nil {
			return true, err
		}
		if len(b.Backends) == 0 {
			t.Spec.To = routev1.RouteTargetReference{}
		} else {
			t.Spec.To = b.Backends[0]
		}
		if len(b.Backends) > 1 {
			t.Spec.AlternateBackends = b.Backends[1:]
		} else {
			t.Spec.AlternateBackends = nil
		}
		return true, nil
	default:
		return false, fmt.Errorf("the object is not a route")
	}
}

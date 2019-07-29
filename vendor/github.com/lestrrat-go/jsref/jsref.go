package jsref

import (
	"net/url"
	"reflect"

	"github.com/lestrrat-go/jspointer"
	"github.com/lestrrat-go/pdebug"
	"github.com/lestrrat-go/structinfo"
	"github.com/pkg/errors"
)

const ref = "$ref"
var refrv = reflect.ValueOf(ref)

type Option interface {
	Name() string
	Value() interface{}
}

type option struct {
	name  string
	value interface{}
}

func (o option) Name() string       { return o.name }
func (o option) Value() interface{} { return o.value }

// WithRecursiveResolution allows ou to enable recursive resolution
// on the *result* data structure. This means that after resolving
// the JSON reference in the structure at hand, it does another
// pass at resolving the entire data structure. Depending on your
// structure and size, this may incur significant cost.
//
// Please note that recursive resolution of the result is still
// experimental. If you find problems, please submit a pull request
// with a failing test case.
func WithRecursiveResolution(b bool) Option {
	return &option{
		name:  "recursiveResolution",
		value: b,
	}
}

var DefaultMaxRecursions = 10

// New creates a new Resolver
func New() *Resolver {
	return &Resolver{MaxRecursions: DefaultMaxRecursions}
}

// AddProvider adds a new Provider to be searched for in case
// a JSON pointer with more than just the URI fragment is given.
func (r *Resolver) AddProvider(p Provider) error {
	r.providers = append(r.providers, p)
	return nil
}

type resolveCtx struct {
	rlevel    int         // recurse level
	maxrlevel int         // max recurse level
	object    interface{} // the main object that was passed to `Resolve()`
}

// Resolve takes a target `v`, and a JSON pointer `spec`.
// spec is expected to be in the form of
//
//    [scheme://[userinfo@]host/path[?query]]#fragment
//    [scheme:opaque[?query]]#fragment
//
// where everything except for `#fragment` is optional.
// If the fragment is empty, an error is returned.
//
// If `spec` is the empty string, `v` is returned
// This method handles recursive JSON references.
//
// If `WithRecursiveResolution` option is given and its value is true,
// an attempt to resolve all references within the resulting object
// is made by traversing the structure recursively. Default is false
func (r *Resolver) Resolve(v interface{}, ptr string, options ...Option) (ret interface{}, err error) {
	if pdebug.Enabled {
		g := pdebug.Marker("Resolver.Resolve(%s)", ptr).BindError(&err)
		defer g.End()
	}
	var recursiveResolution bool
	for _, opt := range options {
		switch opt.Name() {
		case "recursiveResolution":
			recursiveResolution = opt.Value().(bool)
		}
	}

	ctx := resolveCtx{
		rlevel:    0,
		maxrlevel: r.MaxRecursions,
		object:    v,
	}

	// First, expand the target as much as we can
	v, err = expandRefRecursive(&ctx, r, v)
	if err != nil {
		return nil, errors.Wrap(err, "recursive search failed")
	}

	result, err := evalptr(&ctx, r, v, ptr)
	if err != nil {
		return nil, err
	}

	if recursiveResolution {
		rv, err := traverseExpandRefRecursive(&ctx, r, reflect.ValueOf(result))
		if err != nil {
			return nil, errors.Wrap(err, `failed to resolve result`)
		}
		result = rv.Interface()
	}

	return result, nil
}

func setPtrOrInterface(container, value reflect.Value) bool {
	switch container.Kind() {
	case reflect.Ptr:
		if !value.CanAddr() {
			return false
		}
		container.Set(value.Addr())
	case reflect.Interface:
		container.Set(value)
	default:
		return false
	}
	return true
}

func traverseExpandRefRecursive(ctx *resolveCtx, r *Resolver, rv reflect.Value) (reflect.Value, error) {
	if pdebug.Enabled {
		g := pdebug.Marker("traverseExpandRefRecursive")
		defer g.End()
	}

	switch rv.Kind() {
	case reflect.Ptr, reflect.Interface:
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Array, reflect.Slice:
		for i := 0; i < rv.Len(); i++ {
			elem := rv.Index(i)
			var elemcontainer reflect.Value
			switch elem.Kind() {
			case reflect.Ptr, reflect.Interface:
				elemcontainer = elem
				elem = elem.Elem()
			}

			// Need to check for elem being Valid, otherwise the
			// subsequent call to Interface() will fail
			if !elem.IsValid() {
				continue
			}

			if elemcontainer.IsValid() {
				if !elemcontainer.CanSet() {
					continue
				}
			}
			newv, err := expandRefRecursive(ctx, r, elem.Interface())
			if err != nil {
				return zeroval, errors.Wrap(err, `failed to expand array/slice element`)
			}
			newrv, err := traverseExpandRefRecursive(ctx, r, reflect.ValueOf(newv))
			if err != nil {
				return zeroval, errors.Wrap(err, `failed to recurse into array/slice element`)
			}

			if elemcontainer.IsValid() {
				setPtrOrInterface(elemcontainer, newrv)
			} else {
				elem.Set(newrv)
			}
		}
	case reflect.Map:
		// No refs found in the map keys, but there could be more
		// in the values
		if _, err := findRef(rv.Interface()); err != nil {
			for _, key := range rv.MapKeys() {
				value, err := traverseExpandRefRecursive(ctx, r, rv.MapIndex(key))
				if err != nil {
					return zeroval, errors.Wrap(err, `failed to traverse map value`)
				}
				rv.SetMapIndex(key, value)
			}
			return rv, nil
		}
		newv, err := expandRefRecursive(ctx, r, rv.Interface())
		if err != nil {
			return zeroval, errors.Wrap(err, `failed to expand map element`)
		}
		return traverseExpandRefRecursive(ctx, r, reflect.ValueOf(newv))
	case reflect.Struct:
		// No refs found in the map keys, but there could be more
		// in the values
		if _, err := findRef(rv.Interface()); err != nil {
			for i := 0; i < rv.NumField(); i++ {
				field := rv.Field(i)
				value, err := traverseExpandRefRecursive(ctx, r, field)
				if err != nil {
					return zeroval, errors.Wrap(err, `failed to traverse struct field value`)
				}
				field.Set(value)
			}
			return rv, nil
		}
		newv, err := expandRefRecursive(ctx, r, rv.Interface())
		if err != nil {
			return zeroval, errors.Wrap(err, `failed to expand struct element`)
		}
		return traverseExpandRefRecursive(ctx, r, reflect.ValueOf(newv))
	}
	return rv, nil
}

// expands $ref with in v, until all $refs are expanded.
// note: DOES NOT recurse down into structures
func expandRefRecursive(ctx *resolveCtx, r *Resolver, v interface{}) (ret interface{}, err error) {
	if pdebug.Enabled {
		g := pdebug.Marker("expandRefRecursive")
		defer g.End()
	}
	for {
		ref, err := findRef(v)
		if err != nil {
			if pdebug.Enabled {
				pdebug.Printf("No refs found. bailing out of loop")
			}
			break
		}

		if pdebug.Enabled {
			pdebug.Printf("Found ref '%s'", ref)
		}

		newv, err := expandRef(ctx, r, v, ref)
		if err != nil {
			if pdebug.Enabled {
				pdebug.Printf("Failed to expand ref '%s': %s", ref, err)
			}
			return nil, errors.Wrap(err, "failed to expand ref")
		}

		v = newv
	}

	return v, nil
}

func expandRef(ctx *resolveCtx, r *Resolver, v interface{}, ref string) (ret interface{}, err error) {
	ctx.rlevel++
	if ctx.rlevel > ctx.maxrlevel {
		return nil, ErrMaxRecursion
	}

	defer func() { ctx.rlevel-- }()

	u, err := url.Parse(ref)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse ref as URL")
	}

	ptr := "#" + u.Fragment
	if u.Host == "" && u.Path == "" {
		if pdebug.Enabled {
			pdebug.Printf("ptr doesn't contain any host/path part, apply json pointer directly to object")
		}
		return evalptr(ctx, r, ctx.object, ptr)
	}

	u.Fragment = ""
	for _, p := range r.providers {
		pv, err := p.Get(u)
		if err == nil {
			if pdebug.Enabled {
				pdebug.Printf("Found object matching %s", u)
			}

			return evalptr(ctx, r, pv, ptr)
		}
	}

	return nil, errors.New("element pointed by $ref '" + ref + "' not found")
}

func findRef(v interface{}) (ref string, err error) {
	if pdebug.Enabled {
		g := pdebug.Marker("findRef").BindError(&err)
		defer g.End()
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Interface, reflect.Ptr:
		rv = rv.Elem()
	}

	if pdebug.Enabled {
		pdebug.Printf("object is a '%s'", rv.Kind())
	}

	// Find if we have a "$ref" element
	var refv reflect.Value
	switch rv.Kind() {
	case reflect.Map:
		refv = rv.MapIndex(refrv)
	case reflect.Struct:
		if fn := structinfo.StructFieldFromJSONName(rv, ref); fn != "" {
			refv = rv.FieldByName(fn)
		}
	default:
		return "", errors.New("element is not a map-like container")
	}

	if !refv.IsValid() {
		return "", errors.New("$ref element not found")
	}

	switch refv.Kind() {
	case reflect.Interface, reflect.Ptr:
		refv = refv.Elem()
	}

	switch refv.Kind() {
	case reflect.String:
		// Empty string isn't a valid pointer
		if refv.Len() <= 0 {
			return "", errors.New("$ref element not found (empty)")
		}
		if pdebug.Enabled {
			pdebug.Printf("Found ref '%s'", refv)
		}
		return refv.String(), nil
	case reflect.Invalid:
		return "", errors.New("$ref element not found")
	default:
		if pdebug.Enabled {
			pdebug.Printf("'$ref' was found, but its kind is %s", refv.Kind())
		}
	}

	return "", errors.New("$ref element must be a string")
}

func evalptr(ctx *resolveCtx, r *Resolver, v interface{}, ptrspec string) (ret interface{}, err error) {
	if pdebug.Enabled {
		g := pdebug.Marker("evalptr(%s)", ptrspec).BindError(&err)
		defer g.End()
	}

	// If the reference is empty, return v
	if ptrspec == "" || ptrspec == "#" {
		if pdebug.Enabled {
			pdebug.Printf("Empty pointer, return v itself")
		}
		return v, nil
	}

	// Parse the spec.
	u, err := url.Parse(ptrspec)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse reference spec")
	}

	ptr := u.Fragment

	// We are evaluating the pointer part. That means if the
	// Fragment portion is not set, there's no point in evaluating
	if ptr == "" {
		return nil, errors.Wrap(err, "empty json pointer")
	}

	p, err := jspointer.New(ptr)
	if err != nil {
		return nil, errors.Wrap(err, "failed create a new JSON pointer")
	}
	x, err := p.Get(v)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch value")
	}

	if pdebug.Enabled {
		pdebug.Printf("Evaulated JSON pointer, now checking if we can expand further")
	}
	// If this result contains more refs, expand that
	return expandRefRecursive(ctx, r, x)
}

package schema

import (
	"encoding/json"
	"io"
	"net/url"
	"os"
	"reflect"
	"strconv"

	"github.com/lestrrat-go/jsref"
	"github.com/lestrrat-go/jsref/provider"
	"github.com/lestrrat-go/pdebug"
	"github.com/pkg/errors"
)

// This is used to check against result of reflect.MapIndex
var zeroval = reflect.Value{}
var _schema Schema
var _hyperSchema Schema

func init() {
	buildJSSchema()
	buildHyperSchema()
}

// New creates a new schema object
func New() *Schema {
	s := Schema{}
	s.initialize()
	return &s
}

func (s *Schema) initialize() {
	resolver := jsref.New()

	mp := provider.NewMap()
	mp.Set(SchemaURL, &_schema)
	mp.Set(HyperSchemaURL, &_hyperSchema)
	resolver.AddProvider(mp)

	s.resolvedSchemas = make(map[string]interface{})
	s.resolver = resolver
}

// ReadFile reads the file `f` and parses its content to create
// a new Schema object
func ReadFile(f string) (*Schema, error) {
	in, err := os.Open(f)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	return Read(in)
}

// Read reads from `in` and parses its content to create
// a new Schema object
func Read(in io.Reader) (*Schema, error) {
	s := New()
	if err := s.Decode(in); err != nil {
		return nil, err
	}
	return s, nil
}

// Decode reads from `in` and parses its content to
// initialize the schema object
func (s *Schema) Decode(in io.Reader) error {
	dec := json.NewDecoder(in)
	if err := dec.Decode(s); err != nil {
		return err
	}
	s.applyParentSchema()
	return nil
}

func (s *Schema) setParent(v *Schema) {
	s.parent = v
}

func (s *Schema) applyParentSchema() {
	// Find all components that may be a Schema
	for _, v := range s.Definitions {
		v.setParent(s)
		v.applyParentSchema()
	}

	if props := s.AdditionalProperties; props != nil {
		if sc := props.Schema; sc != nil {
			sc.setParent(s)
			sc.applyParentSchema()
		}
	}
	if items := s.AdditionalItems; items != nil {
		if sc := items.Schema; sc != nil {
			sc.setParent(s)
			sc.applyParentSchema()
		}
	}
	if items := s.Items; items != nil {
		for _, v := range items.Schemas {
			v.setParent(s)
			v.applyParentSchema()
		}
	}

	for _, v := range s.Properties {
		v.setParent(s)
		v.applyParentSchema()
	}

	for _, v := range s.AllOf {
		v.setParent(s)
		v.applyParentSchema()
	}

	for _, v := range s.AnyOf {
		v.setParent(s)
		v.applyParentSchema()
	}

	for _, v := range s.OneOf {
		v.setParent(s)
		v.applyParentSchema()
	}

	if v := s.Not; v != nil {
		v.setParent(s)
		v.applyParentSchema()
	}
}

// BaseURL returns the base URL registered for this schema
func (s *Schema) BaseURL() *url.URL {
	scope := s.Scope()
	u, err := url.Parse(scope)
	if err != nil {
		// XXX hmm, not sure what to do here
		u = &url.URL{}
	}

	return u
}

// Root returns the upmost parent schema object within the
// hierarchy of schemas. For example, the `item` element
// in a schema for an array is also a schema, and you could
// reference elements in parent schemas.
func (s *Schema) Root() *Schema {
	if s.parent == nil {
		if pdebug.Enabled {
			pdebug.Printf("Schema %p is root", s)
		}
		return s
	}

	return s.parent.Root()
}

func (s *Schema) findSchemaByID(id string) (*Schema, error) {
	if s.ID == id {
		return s, nil
	}

	// XXX Quite unimplemented
	return nil, errors.Errorf("schema %s not found", strconv.Quote(id))
}

// ResolveURL takes a url string, and resolves it if it's
// a relative URL
func (s *Schema) ResolveURL(v string) (u *url.URL, err error) {
	if pdebug.Enabled {
		g := pdebug.IPrintf("START Schema.ResolveURL '%s'", v)
		defer func() {
			if err != nil {
				g.IRelease("END Schema.ResolveURL '%s': error %s", v, err)
			} else {
				g.IRelease("END Schema.ResolveURL '%s' -> '%s'", v, u)
			}
		}()
	}
	base := s.BaseURL()
	if pdebug.Enabled {
		pdebug.Printf("Using base URL '%s'", base)
	}
	u, err = base.Parse(v)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// IsResolved returns true if this schema has no Reference.
func (s *Schema) IsResolved() bool {
	return s.Reference == ""
}

// Resolve returns the schema after it has been resolved.
// If s.Reference is the empty string, the current schema is returned.
//
// `ctx` is an optional context to resolve the reference with. If not
// specified, the root schema as returned by `Root` will be used.
func (s *Schema) Resolve(ctx interface{}) (ref *Schema, err error) {
	if s.Reference == "" {
		return s, nil
	}

	if pdebug.Enabled {
		g := pdebug.IPrintf("START Schema.Resolve (%s)", s.Reference)
		defer func() {
			if err != nil {
				g.IRelease("END Schema.Resolve (%s): %s", s.Reference, err)
			} else {
				g.IRelease("END Schema.Resolve (%s)", s.Reference)
			}
		}()
	}

	var thing interface{}
	var ok bool
	s.resolveLock.Lock()
	thing, ok = s.resolvedSchemas[s.Reference]
	s.resolveLock.Unlock()

	if ok {
		ref, ok = thing.(*Schema)
		if ok {
			if pdebug.Enabled {
				pdebug.Printf("Cache HIT on '%s'", s.Reference)
			}
		} else {
			if pdebug.Enabled {
				pdebug.Printf("Negative Cache HIT on '%s'", s.Reference)
			}
			return nil, thing.(error)
		}
	} else {
		if pdebug.Enabled {
			pdebug.Printf("Cache MISS on '%s'", s.Reference)
		}
		var err error
		if ctx == nil {
			ctx = s.Root()
		}
		thing, err := s.resolver.Resolve(ctx, s.Reference)
		if err != nil {
			err = errors.Wrapf(err, "failed to resolve reference %s", strconv.Quote(s.Reference))
			s.resolveLock.Lock()
			s.resolvedSchemas[s.Reference] = err
			s.resolveLock.Unlock()
			return nil, err
		}

		ref, ok = thing.(*Schema)
		if !ok {
			err = errors.Wrapf(err, "resolved reference %s is not a schema", strconv.Quote(s.Reference))
			s.resolveLock.Lock()
			s.resolvedSchemas[s.Reference] = err
			s.resolveLock.Unlock()
			return nil, err
		}
		s.resolveLock.Lock()
		s.resolvedSchemas[s.Reference] = ref
		s.resolveLock.Unlock()
	}

	return ref, nil
}

// IsPropRequired can be used to query this schema if a
// given property name is required.
func (s *Schema) IsPropRequired(pname string) bool {
	for _, name := range s.Required {
		if name == pname {
			return true
		}
	}
	return false
}

// Scope returns the scope ID for this schema
func (s *Schema) Scope() string {
	if pdebug.Enabled {
		g := pdebug.IPrintf("START Schema.Scope")
		defer g.IRelease("END Schema.Scope")
	}
	if s.ID != "" || s.parent == nil {
		if pdebug.Enabled {
			pdebug.Printf("Returning id '%s'", s.ID)
		}
		return s.ID
	}

	return s.parent.Scope()
}

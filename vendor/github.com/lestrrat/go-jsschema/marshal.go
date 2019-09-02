package schema

import (
	"encoding/json"
	"regexp"
	"strconv"

	"github.com/lestrrat-go/pdebug"
	"github.com/pkg/errors"
)

func extractNumber(n *Number, m map[string]interface{}, s string) error {
	v, ok := m[s]
	if !ok {
		return nil
	}

	val, ok := v.(float64)
	if !ok {
		return errors.Wrap(errInvalidType("float64", v), "failed to extract number")
	}

	n.Val = val
	n.Initialized = true
	return nil
}

func extractInt(n *Integer, m map[string]interface{}, s string) error {
	v, ok := m[s]
	if !ok {
		return nil
	}

	val, ok := v.(float64)
	if !ok {
		return errors.Wrap(errInvalidType("float64", v), "failed to extract int")
	}

	n.Val = int(val)
	n.Initialized = true
	return nil
}

func extractBool(b *Bool, m map[string]interface{}, s string, def bool) error {
	b.Default = def
	v, ok := m[s]
	if !ok {
		return nil
	}

	val, ok := v.(bool)
	if !ok {
		return errors.Wrap(errInvalidType("bool", v), "failed to extract bool")
	}

	b.Val = val
	b.Initialized = true
	return nil
}

func extractString(s *string, m map[string]interface{}, name string) error {
	v, ok := m[name]
	if !ok {
		return nil
	}

	val, ok := v.(string)
	if !ok {
		return errors.Wrap(errInvalidType("string", v), "failed to extract string")
	}

	*s = val
	return nil
}

func convertStringList(l *[]string, v interface{}) error {
	switch val := v.(type) {
	case string: // One element
		*l = []string{val}
	case []interface{}: // List of elements.
		*l = make([]string, len(val))
		for i, x := range val {
			s, ok := x.(string)
			if !ok {
				return ErrExpectedArrayOfString
			}

			(*l)[i] = s
		}
	default:
		return ErrExpectedArrayOfString
	}
	return nil
}

func extractStringList(l *[]string, m map[string]interface{}, s string) error {
	v, ok := m[s]
	if !ok {
		return nil
	}
	return convertStringList(l, v)
}

func extractFormat(f *Format, m map[string]interface{}, s string) error {
	var v string
	if err := extractString(&v, m, s); err != nil {
		return err
	}
	*f = Format(v)
	return nil
}

func extractJSPointer(s *string, m map[string]interface{}, name string) error {
	return extractString(s, m, name)
}

func extractInterface(r *interface{}, m map[string]interface{}, s string) error {
	if v, ok := m[s]; ok {
		*r = v
	}
	return nil
}

func extractInterfaceList(l *[]interface{}, m map[string]interface{}, s string) error {
	v, ok := m[s]
	if !ok {
		return nil
	}

	val, ok := v.([]interface{})
	if !ok {
		return errors.Wrap(
			errInvalidType("[]interface{}", v),
			"failed to extract interface list",
		)
	}

	*l = make([]interface{}, len(val))
	copy(*l, val)
	return nil
}

func extractRegexp(r **regexp.Regexp, m map[string]interface{}, s string) error {
	v, ok := m[s]
	if !ok {
		return nil
	}
	val, ok := v.(string)
	if !ok {
		return errors.Wrap(
			errInvalidType("string", v),
			"failed to extract regular expression",
		)
	}

	rx, err := regexp.Compile(val)
	if err != nil {
		return errors.Wrap(
			errors.Wrapf(
				err,
				"failed to compile regular expression: %s",
				strconv.Quote(val),
			),
			"failed to extract regular expression",
		)
	}
	*r = rx
	return nil
}

func extractSchema(s **Schema, m map[string]interface{}, name string) error {
	v, ok := m[name]
	if !ok {
		return nil
	}

	if pdebug.Enabled {
		pdebug.Printf("Found property '%s'", name)
	}

	val, ok := v.(map[string]interface{})
	if !ok {
		return errors.Wrap(
			errInvalidType("map[string]interface{}", v),
			"failed to extract schema",
		)
	}

	return extractSingleSchema(s, val)
}

func extractSingleSchema(s **Schema, m map[string]interface{}) error {
	*s = New()
	if err := (*s).Extract(m); err != nil {
		return errors.Wrap(err, "failed to extract schema")
	}
	return nil
}

func (l *SchemaList) extractIfPresent(m map[string]interface{}, name string) error {
	v, ok := m[name]
	if !ok {
		return nil
	}

	if pdebug.Enabled {
		pdebug.Printf("Found property '%s'", name)
	}

	return l.Extract(v)
}

// Extract takes either a list of `map[string]interface{}` or
// a single `map[string]interface{}` to initialize this list
// of schemas
func (l *SchemaList) Extract(v interface{}) error {
	switch val := v.(type) {
	case []interface{}:
		*l = make([]*Schema, len(val))
		var s *Schema
		for i, d := range val {
			m, ok := d.(map[string]interface{})
			if !ok {
				return errors.Wrap(
					errInvalidType("map[string]interface{}", d),
					"failed to extract schema list",
				)
			}
			if err := extractSingleSchema(&s, m); err != nil {
				return errors.Wrap(err, "failed to extract schema list")
			}
			(*l)[i] = s
		}
		return nil
	case map[string]interface{}:
		var s *Schema
		if err := extractSingleSchema(&s, val); err != nil {
			return errors.Wrap(err, "failed to extract schema list")
		}
		*l = []*Schema{s}
		return nil
	default:
		return errors.Wrap(
			errInvalidType("[]*Schema or *Schema", v),
			"failed to extract schema list",
		)
	}
}

func extractSchemaMapEntry(s *Schema, name string, m map[string]interface{}) error {
	if pdebug.Enabled {
		g := pdebug.Marker("Schema map entry '%s'", name)
		defer g.End()
	}
	return s.Extract(m)
}

func extractSchemaMap(m map[string]interface{}, name string) (map[string]*Schema, error) {
	v, ok := m[name]
	if !ok {
		return nil, nil
	}

	val, ok := v.(map[string]interface{})
	if !ok {
		return nil, errors.Wrap(
			errInvalidType("map[string]interface{}", v),
			"failed to extract schema map",
		)
	}

	r := make(map[string]*Schema)
	for k, data := range val {
		// data better be a map
		m, ok := data.(map[string]interface{})
		if !ok {
			return nil, errors.Wrap(
				errInvalidType("map[string]interface{}", data),
				"failed to extract sub field",
			)
		}

		s := New()
		if err := extractSchemaMapEntry(s, k, m); err != nil {
			return nil, err
		}
		r[k] = s

		if k == "domain" {
			if pdebug.Enabled {
				pdebug.Printf("after extractSchemaMapEntry: %#v", s.Extras)
			}
		}
	}
	return r, nil
}

func extractRegexpToSchemaMap(m map[string]interface{}, name string) (map[*regexp.Regexp]*Schema, error) {
	v, ok := m[name]
	if !ok {
		return nil, nil
	}

	val, ok := v.(map[string]interface{})
	if !ok {
		return nil, errors.Wrap(
			errInvalidType("map[string]interface{}", v),
			"failed to extract regexp to schema map",
		)
	}

	r := make(map[*regexp.Regexp]*Schema)
	for k, data := range val {
		// data better be a map
		m, ok := data.(map[string]interface{})
		if !ok {
			return nil, errors.Wrap(
				errInvalidType("map[string]interface{}", data),
				"failed to extract regexp to schema map",
			)
		}
		s := New()
		if err := s.Extract(m); err != nil {
			return nil, errors.Wrap(err, "failed to extract schema within schema map")
		}

		rx, err := regexp.Compile(k)
		if err != nil {
			return nil, errors.Wrap(err, "failed to compile regular expression for regexp to schema map")
		}

		r[rx] = s
	}
	return r, nil
}

func extractItems(res **ItemSpec, m map[string]interface{}, name string) error {
	v, ok := m[name]
	if !ok {
		return nil
	}

	if pdebug.Enabled {
		pdebug.Printf("Found array element '%s'", name)
	}

	tupleMode := false
	switch v.(type) {
	case []interface{}:
		tupleMode = true
	case map[string]interface{}:
	default:
		return errors.Wrap(
			errInvalidType("[]interface{} or map[string]interface{}", v),
			"failed to extract items",
		)
	}

	items := ItemSpec{}
	items.TupleMode = tupleMode

	var err error

	if err = items.Schemas.extractIfPresent(m, name); err != nil {
		return errors.Wrap(err, "failed to schema for item")
	}
	*res = &items
	return nil
}

func extractDependecies(res *DependencyMap, m map[string]interface{}, name string) error {
	v, ok := m[name]
	if !ok {
		return nil
	}

	m, ok = v.(map[string]interface{})
	if !ok {
		return errors.Wrap(
			errInvalidType("map[string]interface{}", v),
			"failed to extract dependencies",
		)
	}

	if len(m) == 0 {
		return nil
	}

	return res.extract(m)
}

func extractType(pt *PrimitiveTypes, m map[string]interface{}, name string) error {
	v, ok := m[name]
	if !ok {
		return nil
	}

	switch val := v.(type) {
	case string:
		t, err := primitiveFromString(val)
		if err != nil {
			return errors.Wrap(err, "failed to parse primitive type")
		}
		*pt = PrimitiveTypes{t}
		return nil
	case []interface{}:
		*pt = make(PrimitiveTypes, len(val))
		for i, ts := range val {
			s, ok := ts.(string)
			if !ok {
				return errors.Wrap(
					errInvalidType("string", ts),
					"failed to extract type",
				)
			}
			t, err := primitiveFromString(s)
			if err != nil {
				return errors.Wrap(err, "failed to parse primitive type")
			}
			(*pt)[i] = t
		}
		return nil
	default:
		return errors.Wrap(
			errInvalidType("[]string or string", v),
			"failed to extract 'type'",
		)
	}
}

func (dm *DependencyMap) extract(m map[string]interface{}) error {
	dm.Names = make(map[string][]string)
	dm.Schemas = make(map[string]*Schema)
	for k, p := range m {
		switch val := p.(type) {
		case []interface{}:
			// This list needs to be a list of strings
			var l []string
			if err := convertStringList(&l, val); err != nil {
				return err
			}

			dm.Names[k] = l
		case map[string]interface{}:
			s := New()
			if err := s.Extract(val); err != nil {
				return err
			}
			dm.Schemas[k] = s
		default:
			return errors.Wrap(
				errInvalidType("[]interface{} or map[string]interface{}", p),
				"failed to extract 'type'",
			)
		}
	}

	return nil
}

// UnmarshalJSON takes a JSON string and initializes
// the schema
func (s *Schema) UnmarshalJSON(data []byte) error {
	m := map[string]interface{}{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	return s.Extract(m)
}

// Extract takes a `map[string]interface{}` and initializes
// the schema
func (s *Schema) Extract(m map[string]interface{}) error {
	if pdebug.Enabled {
		g := pdebug.IPrintf("START Schema.Extract")
		defer g.IRelease("END Schema.Extract")
	}

	var err error

	if err = extractString(&s.ID, m, "id"); err != nil {
		return errors.Wrapf(err, "failed to extract 'id'")
	}

	if err = extractString(&s.Title, m, "title"); err != nil {
		return errors.Wrap(err, "failed to extract 'title'")
	}

	if err = extractString(&s.Description, m, "description"); err != nil {
		return errors.Wrap(err, "failed to extract 'description'")
	}

	if err = extractStringList(&s.Required, m, "required"); err != nil {
		return errors.Wrap(err, "failed to extract 'required'")
	}

	if err = extractJSPointer(&s.SchemaRef, m, "$schema"); err != nil {
		return errors.Wrap(err, "failed to extract '$schema'")
	}

	if err = extractJSPointer(&s.Reference, m, "$ref"); err != nil {
		return errors.Wrap(err, "failed to extract '$ref'")
	}

	if err = extractFormat(&s.Format, m, "format"); err != nil {
		return errors.Wrap(err, "failed to extract 'format'")
	}

	if err = extractInterfaceList(&s.Enum, m, "enum"); err != nil {
		return errors.Wrap(err, "failed to extract 'enum'")
	}

	if err = extractInterface(&s.Default, m, "default"); err != nil {
		return errors.Wrap(err, "failed to extract 'default'")
	}

	if err = extractType(&s.Type, m, "type"); err != nil {
		return errors.Wrap(err, "failed to extract 'type'")
	}

	if s.Definitions, err = extractSchemaMap(m, "definitions"); err != nil {
		return errors.Wrap(err, "failed to extract 'definitions'")
	}

	if err = extractItems(&s.Items, m, "items"); err != nil {
		return errors.Wrap(err, "failed to extract 'items'")
	}

	if err = extractRegexp(&s.Pattern, m, "pattern"); err != nil {
		return errors.Wrap(err, "failed to extract 'patterns'")
	}

	if extractInt(&s.MinLength, m, "minLength"); err != nil {
		return errors.Wrap(err, "failed to extract 'minLength'")
	}

	if extractInt(&s.MaxLength, m, "maxLength"); err != nil {
		return errors.Wrap(err, "failed to extract 'maxLength'")
	}

	if extractInt(&s.MinItems, m, "minItems"); err != nil {
		return errors.Wrap(err, "failed to extract 'minItems'")
	}

	if extractInt(&s.MaxItems, m, "maxItems"); err != nil {
		return errors.Wrap(err, "failed to extract 'maxItems'")
	}

	if err = extractBool(&s.UniqueItems, m, "uniqueItems", false); err != nil {
		return errors.Wrap(err, "failed to extract 'uniqueItems'")
	}

	if err = extractInt(&s.MaxProperties, m, "maxProperties"); err != nil {
		return errors.Wrap(err, "failed to extract 'maxProperties'")
	}

	if err = extractInt(&s.MinProperties, m, "minProperties"); err != nil {
		return errors.Wrap(err, "failed to extract 'minProperties'")
	}

	if err = extractNumber(&s.Minimum, m, "minimum"); err != nil {
		return errors.Wrap(err, "failed to extract 'minimum'")
	}

	if err = extractBool(&s.ExclusiveMinimum, m, "exclusiveMinimum", false); err != nil {
		return errors.Wrap(err, "failed to extract 'exclusiveMinimum'")
	}

	if err = extractNumber(&s.Maximum, m, "maximum"); err != nil {
		return errors.Wrap(err, "failed to extract 'maximum'")
	}

	if err = extractBool(&s.ExclusiveMaximum, m, "exclusiveMaximum", false); err != nil {
		return errors.Wrap(err, "failed to extract 'exclusiveMaximum'")
	}

	if err = extractNumber(&s.MultipleOf, m, "multipleOf"); err != nil {
		return errors.Wrap(err, "failed to extract 'multipleOf'")
	}

	if s.Properties, err = extractSchemaMap(m, "properties"); err != nil {
		return errors.Wrap(err, "failed to extract 'properties'")
	}

	if err = extractDependecies(&s.Dependencies, m, "dependencies"); err != nil {
		return errors.Wrap(err, "failed to extract 'dependencies'")
	}

	if _, ok := m["additionalItems"]; !ok {
		// doesn't exist. it's an empty schema
		s.AdditionalItems = &AdditionalItems{}
	} else {
		var b Bool
		if err = extractBool(&b, m, "additionalItems", true); err == nil {
			if b.Bool() {
				s.AdditionalItems = &AdditionalItems{}
			}
		} else {
			// Oh, it's not a boolean?
			var apSchema *Schema
			if err = extractSchema(&apSchema, m, "additionalItems"); err != nil {
				return errors.Wrap(err, "failed to extract 'additionalItems'")
			}
			s.AdditionalItems = &AdditionalItems{apSchema}
		}
	}

	if _, ok := m["additionalProperties"]; !ok {
		// doesn't exist. it's an empty schema
		s.AdditionalProperties = &AdditionalProperties{}
	} else {
		var b Bool
		if err = extractBool(&b, m, "additionalProperties", true); err == nil {
			if b.Bool() {
				s.AdditionalProperties = &AdditionalProperties{}
			}
		} else {
			// Oh, it's not a boolean?
			var apSchema *Schema
			if err = extractSchema(&apSchema, m, "additionalProperties"); err != nil {
				return errors.Wrap(err, "failed to extract 'additionalProperties'")
			}
			s.AdditionalProperties = &AdditionalProperties{apSchema}
		}
	}

	if s.PatternProperties, err = extractRegexpToSchemaMap(m, "patternProperties"); err != nil {
		return errors.Wrap(err, "failed to extract 'patternProperties'")
	}

	if err = s.AllOf.extractIfPresent(m, "allOf"); err != nil {
		return errors.Wrap(err, "failed to extract 'allOf'")
	}

	if err = s.AnyOf.extractIfPresent(m, "anyOf"); err != nil {
		return errors.Wrap(err, "failed to extract 'anyOf'")
	}

	if err = s.OneOf.extractIfPresent(m, "oneOf"); err != nil {
		return errors.Wrap(err, "failed to extract 'oneOf'")
	}

	if err = extractSchema(&s.Not, m, "not"); err != nil {
		return errors.Wrap(err, "failed to extract 'not'")
	}

	s.applyParentSchema()

	s.Extras = make(map[string]interface{})
	for k, v := range m {
		switch k {
		case "id", "title", "description", "required", "$schema", "$ref", "format", "enum", "default", "type", "definitions", "items", "pattern", "minLength", "maxLength", "minItems", "maxItems", "uniqueItems", "maxProperties", "minProperties", "minimum", "exclusiveMinimum", "maximum", "exclusiveMaximum", "multipleOf", "properties", "dependencies", "additionalItems", "additionalProperties", "patternProperties", "allOf", "anyOf", "oneOf", "not":
			continue
		}
		if pdebug.Enabled {
			pdebug.Printf("Extracting extra field '%s'", k)
		}
		s.Extras[k] = v
	}

	if pdebug.Enabled {
		pdebug.Printf("Successfully extracted schema")
	}

	return nil
}

func place(m map[string]interface{}, name string, v interface{}) {
	m[name] = v
}

func placeString(m map[string]interface{}, name, s string) {
	if s != "" {
		place(m, name, s)
	}
}

func placeList(m map[string]interface{}, name string, l []interface{}) {
	if len(l) > 0 {
		place(m, name, l)
	}
}
func placeSchemaList(m map[string]interface{}, name string, l []*Schema) {
	if len(l) > 0 {
		place(m, name, l)
	}
}

func placeSchemaMap(m map[string]interface{}, name string, l map[string]*Schema) {
	if len(l) > 0 {
		defs := make(map[string]*Schema)
		place(m, name, defs)

		for k, v := range l {
			defs[k] = v
		}
	}
}

func placeStringList(m map[string]interface{}, name string, l []string) {
	if len(l) > 0 {
		place(m, name, l)
	}
}

func placeBool(m map[string]interface{}, name string, value Bool) {
	place(m, name, value.Bool())
}

func placeNumber(m map[string]interface{}, name string, n Number) {
	if !n.Initialized {
		return
	}
	place(m, name, n.Val)
}

func placeInteger(m map[string]interface{}, name string, n Integer) {
	if !n.Initialized {
		return
	}
	place(m, name, n.Val)
}

func canBeType(s *Schema, primType PrimitiveType) bool {
	if len(s.Type) == 0 {
		return true
	}
	for _, t := range s.Type {
		if t == primType {
			return true
		}
	}
	return false
}

// MarshalJSON serializes the schema into a JSON string
func (s *Schema) MarshalJSON() ([]byte, error) {
	m := make(map[string]interface{})

	placeString(m, "id", s.ID)
	placeString(m, "title", s.Title)
	placeString(m, "description", s.Description)
	placeString(m, "$schema", s.SchemaRef)
	placeString(m, "$ref", s.Reference)
	placeStringList(m, "required", s.Required)
	placeList(m, "enum", s.Enum)
	switch len(s.Type) {
	case 0:
	case 1:
		m["type"] = s.Type[0]
	default:
		m["type"] = s.Type
	}

	if items := s.AdditionalItems; items != nil {
		if items.Schema != nil {
			place(m, "additionalItems", items.Schema)
		}
	} else {
		// According to
		// https://spacetelescope.github.io/understanding-json-schema/reference/array.html#list-validation
		// additionalItems only makes sense if we are an array type, no
		// need to inject 'false' for things that are
		// object/int/float/etc.
		if canBeType(s, ArrayType) {
			place(m, "additionalItems", false)
		}
	}

	if rx := s.Pattern; rx != nil {
		placeString(m, "pattern", rx.String())
	}
	placeInteger(m, "maxLength", s.MaxLength)
	placeInteger(m, "minLength", s.MinLength)
	placeInteger(m, "maxItems", s.MaxItems)
	placeInteger(m, "minItems", s.MinItems)
	placeInteger(m, "maxProperties", s.MaxProperties)
	placeInteger(m, "minProperties", s.MinProperties)
	if s.UniqueItems.Initialized {
		placeBool(m, "uniqueItems", s.UniqueItems)
	}
	placeSchemaMap(m, "definitions", s.Definitions)

	if items := s.Items; items != nil {
		if items.TupleMode {
			m["items"] = s.Items.Schemas
		} else {
			m["items"] = s.Items.Schemas[0]
		}
	}

	placeSchemaMap(m, "properties", s.Properties)
	if len(s.PatternProperties) > 0 {
		rxm := make(map[string]*Schema)
		for rx, rxs := range s.PatternProperties {
			rxm[rx.String()] = rxs
		}
		placeSchemaMap(m, "patternProperties", rxm)
	}

	placeSchemaList(m, "allOf", s.AllOf)
	placeSchemaList(m, "anyOf", s.AnyOf)
	placeSchemaList(m, "oneOf", s.OneOf)

	if s.Default != nil {
		m["default"] = s.Default
	}

	placeString(m, "format", string(s.Format))
	placeNumber(m, "minimum", s.Minimum)
	if s.ExclusiveMinimum.Initialized {
		placeBool(m, "exclusiveMinimum", s.ExclusiveMinimum)
	}
	placeNumber(m, "maximum", s.Maximum)
	if s.ExclusiveMaximum.Initialized {
		placeBool(m, "exclusiveMaximum", s.ExclusiveMaximum)
	}

	if ap := s.AdditionalProperties; ap != nil {
		if ap.Schema != nil {
			place(m, "additionalProperties", ap.Schema)
		}
	} else {
		// Only set
		// additionalProperties: false
		// If we are an Object type.
		// https://spacetelescope.github.io/understanding-json-schema/reference/object.html#properties
		// additionalProperties only has meaning for Object types.
		if canBeType(s, ObjectType) {
			placeBool(m, "additionalProperties", Bool{Val: false, Initialized: true})
		}
	}

	if s.MultipleOf.Val != 0 {
		placeNumber(m, "multipleOf", s.MultipleOf)
	}

	if v := s.Not; v != nil {
		place(m, "not", v)
	}

	deps := map[string]interface{}{}
	if v := s.Dependencies.Schemas; v != nil {
		for pname, depschema := range v {
			deps[pname] = depschema
		}
	}
	if v := s.Dependencies.Names; v != nil {
		for pname, deplist := range v {
			deps[pname] = deplist
		}
	}

	if len(deps) > 0 {
		place(m, "dependencies", deps)
	}

	if x := s.Extras; x != nil {
		for k, v := range x {
			m[k] = v
		}
	}

	return json.Marshal(m)
}

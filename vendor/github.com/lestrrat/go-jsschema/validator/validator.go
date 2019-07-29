package validator

import (
	"sync"

	"github.com/lestrrat-go/jsschema"
	"github.com/lestrrat-go/jsval"
	"github.com/lestrrat-go/jsval/builder"
	"github.com/pkg/errors"
)

// Validator is an object that wraps jsval.JSVal, and
// can be used to validate an object against a schema
type Validator struct {
	lock   sync.Mutex
	schema *schema.Schema
	jsval  *jsval.JSVal
}

// New creates a new Validator from a JSON Schema
func New(s *schema.Schema) *Validator {
	return &Validator{
		schema: s,
	}
}

// Compile takes the underlying schema and compiles
// the validator from it.
// You usually should NOT use this method (the main
// reason this is exposed is for benchmarking), as it
// is automatically called when `Validate` is called.
func (v *Validator) Compile() (*jsval.JSVal, error) {
	b := builder.New()
	jsv, err := b.Build(v.schema)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build validator")
	}
	return jsv, nil
}

func (v *Validator) validator() (*jsval.JSVal, error) {
	v.lock.Lock()
	defer v.lock.Unlock()

	if v.jsval == nil {
		val, err := v.Compile()
		if err != nil {
			return nil, err
		}
		v.jsval = val
	}
	return v.jsval, nil
}

// Validate takes an arbitrary piece of data and
// validates it against the schema.
func (v *Validator) Validate(x interface{}) error {
	jsv, err := v.validator()
	if err != nil {
		return err
	}
	return jsv.Validate(x)
}

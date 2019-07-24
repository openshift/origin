package schema_test

import (
	"log"

	"github.com/lestrrat-go/jsschema"
	"github.com/lestrrat-go/jsschema/validator"
)

func Example() {
	s, err := schema.ReadFile("schema.json")
	if err != nil {
		log.Printf("failed to read schema: %s", err)
		return
	}

	for name, pdef := range s.Properties {
		// Do what you will with `pdef`, which contain
		// Schema information for `name` property
		_ = name
		_ = pdef
	}

	// Create a validator
	v := validator.New(s)

	// You can also validate an arbitrary piece of data
	var p interface{} // initialize using json.Unmarshal...
	if err := v.Validate(p); err != nil {
		log.Printf("failed to validate data: %s", err)
	}
}

package jsref

import (
	"errors"
	"net/url"
	"reflect"
)

var zeroval = reflect.Value{}

var ErrMaxRecursion = errors.New("reached max number of recursions")

// Resolver is responsible for interpreting the provided JSON
// reference.
type Resolver struct {
	providers     []Provider
	MaxRecursions int
}

// Provider resolves a URL into a ... thing.
type Provider interface {
	Get(*url.URL) (interface{}, error)
}

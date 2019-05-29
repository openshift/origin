package generator

// Generator is an interface for generating random values
// from an input expression
type Generator interface {
	GenerateValue(expression string) (interface{}, error)
}

package generator

import "fmt"

type GeneratorType interface {
	Value() (string, error)
}

type ExpresionGenerator struct {
	expr string
}

func (g ExpresionGenerator) Value() (string, error) {
	return FromTemplate(g.expr)
}

type PasswordGenerator struct {
	length int
}

func (g PasswordGenerator) Value() (string, error) {
	return FromTemplate(fmt.Sprintf("[\\a]{%d}", g.length))
}

type Generator struct{}

func (g Generator) Generate(t string) GeneratorType {
	switch t {
	case "password":
		return PasswordGenerator{length: 8}
	default:
		return ExpresionGenerator{expr: t}
	}
}

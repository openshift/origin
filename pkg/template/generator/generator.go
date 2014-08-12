package generator

import (
	"fmt"
	"math/rand"
	"net/http"
)

type GeneratorType interface {
	Value() (string, error)
}

func (g PasswordGenerator) Value() (string, error) {
	return GeneratorTemplate{Expression: fmt.Sprintf("[\\a]{%d}", g.length)}.Value()
}

type Generator struct {
	Seed *rand.Rand
}

type PasswordGenerator struct {
	*Generator
	length int
}

func (g Generator) Generate(t string) GeneratorType {
	if g.Seed == nil {
		return nil
	}
	switch t {
	case "password":
		return PasswordGenerator{length: 8}
	default:
		return GeneratorTemplate{Expression: t, HttpClient: &http.Client{}, Seed: g.Seed}
	}
}

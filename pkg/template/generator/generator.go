package generator

import (
	"fmt"
	"math/rand"
	"net/http"
)

type GeneratorType interface {
	Value() (string, error)
}

type PasswordGenerator struct {
	length int
}

func (g PasswordGenerator) Value() (string, error) {
	return Template{Expression: fmt.Sprintf("[\\a]{%d}", g.length)}.Value()
}

type Generator struct {
	seed *rand.Rand
}

func (g *Generator) SetSeed(r *rand.Rand) {
	g.seed = r
}

func (g Generator) Generate(t string) GeneratorType {
	if g.seed == nil {
		return nil
	}
	switch t {
	case "password":
		return PasswordGenerator{length: 8}
	default:
		return Template{Expression: t, HttpClient: &http.Client{}, seed: g.seed}
	}
}

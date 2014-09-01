package generator

import (
	"fmt"
	"math/rand"
	"regexp"
)

// PasswordGenerator generates string of 8 random alphanumeric characters
// from an input expression matching "password" string.
//
// Example:
//   - "password" => "hW4yQU5i"
type PasswordGenerator struct {
	expressionValueGenerator ExpressionValueGenerator
}

var passwordExp = regexp.MustCompile(`password`)

func init() {
	RegisterGenerator(passwordExp, func(seed *rand.Rand) (GeneratorInterface, error) { return newPasswordGenerator(seed) })
}

func newPasswordGenerator(seed *rand.Rand) (PasswordGenerator, error) {
	return PasswordGenerator{ExpressionValueGenerator{seed: seed}}, nil
}

func (g PasswordGenerator) GenerateValue(string) (interface{}, error) {
	return g.expressionValueGenerator.GenerateValue(fmt.Sprintf("[\\a]{%d}", 8))
}

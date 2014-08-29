package generator

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
)

// ExpressionValueGenerator generates random string based on the input
// expression. The input expression is a string, which may contain
// generator constructs matching "[a-zA-Z0-9]{length}" expression.
//
// Examples:
//   - "test[0-9]{1}x" => "test7x"
//   - "[0-1]{8}" => "01001100"
//   - "0x[A-F0-9]{4}" => "0xB3AF"
//   - "[a-zA-Z0-9]{8}" => "hW4yQU5i"
type ExpressionValueGenerator struct {
	seed *rand.Rand
}

const (
	Alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	Numerals = "0123456789"
	Ascii    = Alphabet + Numerals + "~!@#$%^&*()-_+={}[]\\|<,>.?/\"';:`"
)

var (
	rangeExp      = regexp.MustCompile(`([\\]?[a-zA-Z0-9]\-?[a-zA-Z0-9]?)`)
	generatorsExp = regexp.MustCompile(`\[([a-zA-Z0-9\-\\]+)\](\{([0-9]+)\})`)
)

func init() {
	RegisterGenerator(generatorsExp, func(seed *rand.Rand) (GeneratorInterface, error) { return newExpressionValueGenerator(seed) })
}

func newExpressionValueGenerator(seed *rand.Rand) (ExpressionValueGenerator, error) {
	return ExpressionValueGenerator{seed: seed}, nil
}

func (g ExpressionValueGenerator) GenerateValue(expression string) (interface{}, error) {
	genMatches := generatorsExp.FindAllStringIndex(expression, -1)
	for _, r := range genMatches {
		ranges, length, err := rangesAndLength(expression[r[0]:r[1]])
		if err != nil {
			return "", err
		}
		positions := findExpressionPos(ranges)
		err = replaceWithGenerated(&expression, expression[r[0]:r[1]], positions, length, g.seed)
		if err != nil {
			return "", err
		}
	}
	return expression, nil
}

func alphabetSlice(from, to byte) (string, error) {
	leftPos := strings.Index(Ascii, string(from))
	rightPos := strings.LastIndex(Ascii, string(to))
	if leftPos > rightPos {
		return "", fmt.Errorf("Invalid range specified: %s-%s", string(from), string(to))
	}
	return Ascii[leftPos:rightPos], nil
}

func replaceWithGenerated(s *string, expression string, ranges [][]byte, length int, seed *rand.Rand) error {
	var alphabet string
	for _, r := range ranges {
		switch string(r[0]) + string(r[1]) {
		case `\w`:
			alphabet += Ascii
		case `\d`:
			alphabet += Numerals
		case `\a`:
			alphabet += Alphabet + Numerals
		default:
			if slice, err := alphabetSlice(r[0], r[1]); err != nil {
				return err
			} else {
				alphabet += slice
			}
		}
	}
	if len(alphabet) == 0 {
		return fmt.Errorf("Empty range in expression: %s", expression)
	}
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = alphabet[seed.Intn(len(alphabet))]
	}
	*s = strings.Replace(*s, expression, string(result), 1)
	return nil
}

func findExpressionPos(s string) [][]byte {
	matches := rangeExp.FindAllStringIndex(s, -1)
	result := make([][]byte, len(matches))
	for i, r := range matches {
		result[i] = []byte{s[r[0]], s[r[1]-1]}
	}
	return result
}

func parseLength(s string) (int, error) {
	lengthStr := string(s[strings.LastIndex(s, "{")+1 : len(s)-1])
	if l, err := strconv.Atoi(lengthStr); err != nil {
		return 0, fmt.Errorf("Unable to parse length from %v", s)
	} else {
		return l, nil
	}
}

func rangesAndLength(s string) (string, int, error) {
	l := strings.LastIndex(s, "{")
	if l > 0 {
		expr := s[0:strings.LastIndex(s, "{")]
		length, err := parseLength(s)
		return expr, length, err
	} else {
		return "", 0, fmt.Errorf("Unable to parse length from %v", s)
	}
}

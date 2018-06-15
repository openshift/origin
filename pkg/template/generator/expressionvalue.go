package generator

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
)

// ExpressionValueGenerator implements Generator interface. It generates
// random string based on the input expression. The input expression is
// a string, which may contain "[a-zA-Z0-9]{length}" constructs,
// defining range and length of the result random characters.
//
// Examples:
//
// from             | value
// -----------------------------
// "test[0-9]{1}x"  | "test7x"
// "[0-1]{8}"       | "01001100"
// "0x[A-F0-9]{4}"  | "0xB3AF"
// "[a-zA-Z0-9]{8}" | "hW4yQU5i"
//
// TODO: Support more regexp constructs.
type ExpressionValueGenerator struct {
	seed *rand.Rand
}

const (
	Alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	Numerals = "0123456789"
	Symbols  = "~!@#$%^&*()-_+={}[]\\|<,>.?/\"';:`"
	ASCII    = Alphabet + Numerals + Symbols
)

var (
	rangeExp      = regexp.MustCompile(`([\\]?[a-zA-Z0-9]\-?[a-zA-Z0-9]?)`)
	generatorsExp = regexp.MustCompile(`\[([a-zA-Z0-9\-\\]+)\](\{([0-9]+)\})`)
	expressionExp = regexp.MustCompile(`\[(\\w|\\d|\\a|\\A)|([a-zA-Z0-9]\-[a-zA-Z0-9])+\]`)
)

// NewExpressionValueGenerator creates new ExpressionValueGenerator.
func NewExpressionValueGenerator(seed *rand.Rand) ExpressionValueGenerator {
	return ExpressionValueGenerator{seed: seed}
}

// GenerateValue generates random string based on the input expression.
// The input expression is a pseudo-regex formatted string. See
// ExpressionValueGenerator for more details.
func (g ExpressionValueGenerator) GenerateValue(expression string) (interface{}, error) {
	for {
		r := generatorsExp.FindStringIndex(expression)
		if r == nil {
			break
		}
		ranges, length, err := rangesAndLength(expression[r[0]:r[1]])
		if err != nil {
			return "", err
		}
		err = replaceWithGenerated(
			&expression,
			expression[r[0]:r[1]],
			findExpressionPos(ranges),
			length,
			g.seed,
		)
		if err != nil {
			return "", err
		}
	}
	return expression, nil
}

// alphabetSlice produces a string slice that contains all characters within
// a specified range.
func alphabetSlice(from, to byte) (string, error) {
	leftPos := strings.Index(ASCII, string(from))
	rightPos := strings.LastIndex(ASCII, string(to))
	if leftPos > rightPos {
		return "", fmt.Errorf("invalid range specified: %s-%s", string(from), string(to))
	}
	return ASCII[leftPos:rightPos], nil
}

// replaceWithGenerated replaces all occurrences of the given expression
// in the string with random characters of the specified range and length.
func replaceWithGenerated(s *string, expression string, ranges [][]byte, length int, seed *rand.Rand) error {
	var alphabet string
	for _, r := range ranges {
		switch string(r[0]) + string(r[1]) {
		case `\w`:
			alphabet += Alphabet + Numerals + "_"
		case `\d`:
			alphabet += Numerals
		case `\a`:
			alphabet += Alphabet + Numerals
		case `\A`:
			alphabet += Symbols
		default:
			slice, err := alphabetSlice(r[0], r[1])
			if err != nil {
				return err
			}
			alphabet += slice
		}
	}
	result := make([]byte, length)
	alphabet = removeDuplicateChars(alphabet)
	for i := 0; i < length; i++ {
		result[i] = alphabet[seed.Intn(len(alphabet))]
	}
	*s = strings.Replace(*s, expression, string(result), 1)
	return nil
}

// removeDuplicateChars removes the duplicate characters from the data slice
func removeDuplicateChars(input string) string {
	data := []byte(input)
	length := len(data) - 1
	for i := 0; i < length; i++ {
		for j := i + 1; j <= length; j++ {
			if data[i] == data[j] {
				data[j] = data[length]
				data = data[0:length]
				length--
				j--
			}
		}
	}
	return string(data)
}

// findExpressionPos searches the given string for the valid expressions
// and returns their corresponding indexes.
func findExpressionPos(s string) [][]byte {
	matches := rangeExp.FindAllStringIndex(s, -1)
	result := make([][]byte, len(matches))
	for i, r := range matches {
		result[i] = []byte{s[r[0]], s[r[1]-1]}
	}
	return result
}

// rangesAndLength extracts the expression ranges (eg. [A-Z0-9]) and length
// (eg. {3}). This helper function also validates the expression syntax and
// its length (must be within 1..255).
func rangesAndLength(s string) (string, int, error) {
	expr := s[0:strings.LastIndex(s, "{")]
	if !expressionExp.MatchString(expr) {
		return "", 0, fmt.Errorf("malformed expresion syntax: %s", expr)
	}

	length, _ := strconv.Atoi(s[strings.LastIndex(s, "{")+1 : len(s)-1])
	// TODO: We do need to set a better limit for the number of generated characters.
	if length > 0 && length <= 255 {
		return expr, length, nil
	}
	return "", 0, fmt.Errorf("range must be within [1-255] characters (%d)", length)
}

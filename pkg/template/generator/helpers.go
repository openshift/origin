package generator

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
)

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

func rangesAndLength(s string) (string, int, error) {
	l := strings.LastIndex(s, "{")
	// If the length ({}) is not specified in expression,
	// then assume the length is 1 character
	if l > 0 {
		expr := s[0:strings.LastIndex(s, "{")]
		length, err := parseLength(s)
		return expr, length, err
	} else {
		return s, 1, nil
	}
}

func parseLength(s string) (int, error) {
	lengthStr := string(s[strings.LastIndex(s, "{")+1 : len(s)-1])
	if l, err := strconv.Atoi(lengthStr); err != nil {
		return 0, fmt.Errorf("Unable to parse length from %v", s)
	} else {
		return l, nil
	}
}

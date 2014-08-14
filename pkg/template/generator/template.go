package generator

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
)

const (
	Alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	Numerals = "0123456789"
	Ascii    = Alphabet + Numerals + "~!@#$%^&*()-_+={}[]\\|<,>.?/\"';:`"
)

var (
	rangeExp      = regexp.MustCompile(`([\\]?[a-zA-Z0-9]\-?[a-zA-Z0-9]?)`)
	generatorsExp = regexp.MustCompile(`\[([a-zA-Z0-9\-\\]+)\](\{([0-9]+)\})`)
	remoteExp     = regexp.MustCompile(`\[GET\:(http(s)?:\/\/(.+))\]`)
)

type GeneratorExprRanges [][]byte

func randomInt(n int) int {
	return rand.Intn(n)
}

func alphabetSlice(from, to byte) (string, error) {
	leftPos := strings.Index(Ascii, string(from))
	rightPos := strings.LastIndex(Ascii, string(to))
	if leftPos > rightPos {
		return "", fmt.Errorf("Invalid range specified: %s-%s", string(from), string(to))
	}
	return Ascii[leftPos:rightPos], nil
}

func replaceWithGenerated(s *string, expresion string, ranges [][]byte, length int) error {
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
		return fmt.Errorf("Empty range in expresion: %s", expresion)
	}
	result := make([]byte, length, length)
	for i := 0; i <= length-1; i++ {
		result[i] = alphabet[randomInt(len(alphabet))]
	}
	*s = strings.Replace(*s, expresion, string(result), 1)
	return nil
}

func findExpresionPos(s string) GeneratorExprRanges {
	matches := rangeExp.FindAllStringIndex(s, -1)
	result := make(GeneratorExprRanges, len(matches), len(matches))
	for i, r := range matches {
		result[i] = []byte{s[r[0]], s[r[1]-1]}
	}
	return result
}

func rangesAndLength(s string) (string, int, error) {
	l := strings.LastIndex(s, "{")
	// If the length ({}) is not specified in expresion,
	// then assume the length is 1 character
	//
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

func FromTemplate(template string) (string, error) {
	result := template
	genMatches := generatorsExp.FindAllStringIndex(template, -1)
	remMatches := remoteExp.FindAllStringIndex(template, -1)
	// Parse [a-z]{} types
	for _, r := range genMatches {
		ranges, length, err := rangesAndLength(template[r[0]:r[1]])
		if err != nil {
			return "", err
		}
		positions := findExpresionPos(ranges)
		if err := replaceWithGenerated(&result, template[r[0]:r[1]], positions, length); err != nil {
			return "", err
		}
	}
	// Parse [GET:<url>] type
	//
	for _, r := range remMatches {
		if err := replaceUrlWithData(&result, template[r[0]:r[1]]); err != nil {
			return "", err
		}
	}
	return result, nil
}

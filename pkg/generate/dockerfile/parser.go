package dockerfile

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"regexp"
	"strings"
	"unicode"

	dparser "github.com/docker/docker/builder/parser"
)

type dockerfile [][]string

// Parser is a Dockerfile parser
type Parser interface {
	Parse(input io.Reader) (Dockerfile, error)
}

type parser struct{}

// NewParser creates a new Dockerfile parser
func NewParser() Parser {
	return &parser{}
}

// Dockerfile represents a parsed Dockerfile
type Dockerfile interface {
	GetDirective(name string) ([]string, bool)
}

// Parse parses an input Dockerfile
func (_ *parser) Parse(input io.Reader) (Dockerfile, error) {
	b, err := ioutil.ReadAll(input)
	if err != nil {
		return nil, err
	}
	r := bytes.NewReader(b)

	// Add one more level of validation by using the Docker parser
	if _, err := dparser.Parse(r); err != nil {
		return nil, fmt.Errorf("cannot parse Dockerfile: %v", err)
	}

	if _, err = r.Seek(0, 0); err != nil {
		return nil, err
	}

	d := dockerfile{}
	scanner := bufio.NewScanner(r)
	for {
		line, ok := nextLine(scanner, true)
		if !ok {
			break
		}
		parts, err := parseLine(line)
		if err != nil {
			return nil, err
		}
		d = append(d, parts)
	}
	return d, nil
}

// GetDirective returns a list of lines that begin with the given directive
// and a flag that is true if the directive was found in the Dockerfile
func (d dockerfile) GetDirective(s string) ([]string, bool) {
	values := []string{}
	s = strings.ToLower(s)
	for _, line := range d {
		if strings.ToLower(line[0]) == s {
			values = append(values, line[1])
		}
	}
	return values, len(values) > 0
}

func isComment(line string) bool {
	return strings.HasPrefix(line, "#")
}

func hasContinuation(line string) bool {
	return strings.HasSuffix(strings.TrimRightFunc(line, unicode.IsSpace), "\\")
}

func stripContinuation(line string) string {
	line = strings.TrimRightFunc(line, unicode.IsSpace)
	return line[:len(line)-1]
}

func nextLine(scanner *bufio.Scanner, trimLeft bool) (string, bool) {
	if scanner.Scan() {
		line := scanner.Text()
		if trimLeft {
			line = strings.TrimLeftFunc(line, unicode.IsSpace)
		}
		if line == "" || isComment(line) {
			return nextLine(scanner, true)
		}
		if hasContinuation(line) {
			line := stripContinuation(line)
			next, ok := nextLine(scanner, false)
			if ok {
				return line + next, true
			} else {
				return line, true
			}
		}
		return line, true
	}
	return "", false
}

var dockerLineDelim = regexp.MustCompile(`[\t\v\f\r ]+`)

func parseLine(line string) ([]string, error) {
	parts := dockerLineDelim.Split(line, 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("Invalid Dockerfile")
	}
	return parts, nil
}

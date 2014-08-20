package generator

import (
	"io/ioutil"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
)

type Template struct {
	Expression string
	HttpClient *http.Client
	seed       *rand.Rand
}

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

func (t Template) Value() (string, error) {
	result := t.Expression
	genMatches := generatorsExp.FindAllStringIndex(t.Expression, -1)
	remMatches := remoteExp.FindAllStringIndex(t.Expression, -1)

	// Parse [a-zA-Z0-9]{length} types
	//
	for _, r := range genMatches {
		ranges, length, err := rangesAndLength(t.Expression[r[0]:r[1]])
		if err != nil {
			return "", err
		}
		positions := findExpresionPos(ranges)
		err = replaceWithGenerated(&result, t.Expression[r[0]:r[1]], positions, length, t.seed)
		if err != nil {
			return "", err
		}
	}

	// Parse [GET:<url>] parameters
	//
	for _, r := range remMatches {
		response, err := t.HttpClient.Get(t.Expression[5 : len(t.Expression)-1])
		if err != nil {
			return "", err
		}
		defer response.Body.Close()
		val, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return "", err
		}
		result = strings.Replace(result, t.Expression[r[0]:r[1]], strings.TrimSpace(string(val)), 1)
	}
	return result, nil
}

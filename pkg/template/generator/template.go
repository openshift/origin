package generator

import (
	"io/ioutil"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
)

type GeneratorTemplate struct {
	Expression string
	HttpClient *http.Client
	Seed       *rand.Rand
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

func (t GeneratorTemplate) Value() (string, error) {
	exp := t.Expression
	result := exp
	genMatches := generatorsExp.FindAllStringIndex(exp, -1)
	remMatches := remoteExp.FindAllStringIndex(exp, -1)

	// Parse [a-zA-Z0-9]{length} types
	for _, r := range genMatches {
		ranges, length, err := rangesAndLength(exp[r[0]:r[1]])
		if err != nil {
			return "", err
		}
		positions := findExpressionPos(ranges)
		err = replaceWithGenerated(&result, exp[r[0]:r[1]], positions, length, t.Seed)
		if err != nil {
			return "", err
		}
	}

	// Parse [GET:<url>] parameters
	for _, r := range remMatches {
		response, err := t.HttpClient.Get(exp[5 : len(exp)-1])
		if err != nil {
			return "", err
		}
		defer response.Body.Close()
		val, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return "", err
		}
		result = strings.Replace(result, exp[r[0]:r[1]], strings.TrimSpace(string(val)), 1)
	}
	return result, nil
}

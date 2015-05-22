package examples

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
)

// RemoteValueGenerator implements GeneratorInterface. It fetches random value
// from an external url endpoint based on the "[GET:<url>]" input expression.
//
// Example:
//   - "[GET:http://api.example.com/generateRandomValue]"
type RemoteValueGenerator struct {
}

var remoteExp = regexp.MustCompile(`\[GET\:(http(s)?:\/\/(.+))\]`)

// NewRemoteValueGenerator creates new RemoteValueGenerator.
func NewRemoteValueGenerator() RemoteValueGenerator {
	return RemoteValueGenerator{}
}

// GenerateValue fetches random value from an external url. The input
// expression must be of the form "[GET:<url>]".
func (g RemoteValueGenerator) GenerateValue(expression string) (interface{}, error) {
	matches := remoteExp.FindAllStringIndex(expression, -1)
	if len(matches) < 1 {
		return expression, fmt.Errorf("no matches found.")
	}
	for _, r := range matches {
		response, err := http.Get(expression[5 : len(expression)-1])
		if err != nil {
			return "", err
		}
		defer response.Body.Close()
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return "", err
		}
		expression = strings.Replace(expression, expression[r[0]:r[1]], strings.TrimSpace(string(body)), 1)
	}
	return expression, nil
}

package scope

import (
	"strings"
)

func Split(scope string) []string {
	return strings.Split(scope, " ")
}

func Join(scopes []string) string {
	return strings.Join(scopes, " ")
}

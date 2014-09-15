package scope

import (
	"sort"
	"strings"
)

func Split(scope string) []string {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return []string{}
	}
	return strings.Split(scope, " ")
}

func Join(scopes []string) string {
	return strings.Join(scopes, " ")
}

func Covers(has, requested []string) bool {
	has, requested = sortAndCopy(has), sortAndCopy(requested)
NextRequested:
	for i := range requested {
		for j := range has {
			if has[j] == requested[i] {
				continue NextRequested
			}
		}
		return false
	}
	return true
}

func sortAndCopy(arr []string) []string {
	newArr := make([]string, len(arr))
	copy(newArr, arr)
	sort.Sort(sort.StringSlice(newArr))
	return newArr
}

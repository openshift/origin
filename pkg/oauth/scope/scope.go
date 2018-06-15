package scope

import (
	"sort"
	"strings"
)

// Add takes two sets of scopes, and returns a combined sorted set of scopes
func Add(has []string, new []string) []string {
	sorted := sortAndCopy(has)
	for _, s := range new {
		i := sort.SearchStrings(sorted, s)
		if i == len(sorted) {
			sorted = append(sorted, s)
		} else if sorted[i] != s {
			sorted = append(sorted, "")
			copy(sorted[i+1:], sorted[i:])
			sorted[i] = s
		}
	}
	return sorted
}

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
	// no scopes allows all access, so requesting an empty list is NOT covered by a list with anything in it
	if len(requested) == 0 && len(has) > 0 {
		return false
	}

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

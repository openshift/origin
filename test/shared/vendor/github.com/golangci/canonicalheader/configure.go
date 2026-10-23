package canonicalheader

import (
	"strings"
)

// stringSet is a set-of-nonempty-strings-valued flag.
type stringSet []string

func (s *stringSet) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSet) Set(flag string) error {
	list := strings.Split(flag, ",")

	*s = make(stringSet, 0, len(list))

	for _, element := range list {
		element = strings.TrimSpace(element)
		if element == "" {
			continue
		}

		*s = append(*s, element)
	}
	return nil
}

package trace

import (
	"strings"
)

func labelNameForNode(importPath string) string {
	segs := strings.Split(importPath, "/vendor/")
	if len(segs) > 1 {
		return segs[1]
	}

	return importPath
}

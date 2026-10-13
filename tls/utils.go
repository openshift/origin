package ownership

import (
	"embed"
	_ "embed"
)

//go:embed ownership/ownership.json
var PKIOwnership []byte

//go:embed violations/ownership/ownership-violations.json
var PKIViolations []byte

//go:embed violations
var AllViolations embed.FS

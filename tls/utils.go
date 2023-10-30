package ownership

import _ "embed"

//go:embed ownership/tls-ownership.json
var PKIOwnership []byte

//go:embed violations/ownership/ownership-violations.json
var PKIViolations []byte

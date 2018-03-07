package svcat

import (
	// This workaround a gap in dep where we have no control over the version of
	// our transitive dependencies.
	// Once https://github.com/golang/dep/pull/1489 is merged, we can remove this file.
	_ "github.com/Azure/go-autorest/autorest"
)

package master

import (
	"os"
	"strings"
)

var disableEndpointRegistration = strings.EqualFold(os.Getenv("DISABLE_KUBE_ENDPOINT_REGISTRATION"), "true")

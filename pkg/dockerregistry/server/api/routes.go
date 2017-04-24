package api

import (
	"github.com/docker/distribution/reference"
)

var (
	AdminPrefix      = "/admin/"
	ExtensionsPrefix = "/extensions/v2/"

	AdminPath      = "/blobs/{digest:" + reference.DigestRegexp.String() + "}"
	SignaturesPath = "/{name:" + reference.NameRegexp.String() + "}/signatures/{digest:" + reference.DigestRegexp.String() + "}"
	MetricsPath    = "/metrics"
)

package nethttplibrary

import "go.opentelemetry.io/otel/attribute"

// HTTP Request attributes
const (
	httpRequestBodySizeAttribute          = attribute.Key("http.request.body.size")
	httpRequestResendCountAttribute       = attribute.Key("http.request.resend_count")
	httpRequestMethodAttribute            = attribute.Key("http.request.method")
	httpRequestHeaderContentTypeAttribute = attribute.Key("http.request.header.content-type")
)

// HTTP Response attributes
const (
	httpResponseBodySizeAttribute          = attribute.Key("http.response.body.size")
	httpResponseHeaderContentTypeAttribute = attribute.Key("http.response.header.content-type")
	httpResponseStatusCodeAttribute        = attribute.Key("http.response.status_code")
)

// Network attributes
const (
	networkProtocolNameAttribute = attribute.Key("network.protocol.name")
)

// Server attributes
const (
	serverAddressAttribute = attribute.Key("server.address")
)

// URL attributes
const (
	urlFullAttribute        = attribute.Key("url.full")
	urlSchemeAttribute      = attribute.Key("url.scheme")
	urlUriTemplateAttribute = attribute.Key("url.uri_template")
)

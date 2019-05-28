package tokens

const (
	// URLToken in the query of the redirectURL gets replaced with the original request URL, escaped as a query parameter.
	// Example use: https://www.example.com/login?then=${url}
	URLToken = "${url}"

	// ServerRelativeURLToken in the query of the redirectURL gets replaced with the server-relative portion of the original request URL, escaped as a query parameter.
	// Example use: https://www.example.com/login?then=${server-relative-url}
	ServerRelativeURLToken = "${server-relative-url}"

	// QueryToken in the query of the redirectURL gets replaced with the original request URL, unescaped.
	// Example use: https://www.example.com/sso/oauth/authorize?${query}
	QueryToken = "${query}"
)

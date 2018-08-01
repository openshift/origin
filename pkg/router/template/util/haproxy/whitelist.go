package haproxy

import (
	"strings"
)

const (
	// HAPROXY_MAX_LINE_ARGS is the maximum number of arguments that haproxy
	// supports on a configuration line.
	// Ref: https://github.com/haproxy/haproxy/blob/master/include/common/defaults.h#L75
	HAPROXY_MAX_LINE_ARGS = 64

	// HAPROXY_MAX_WHITELIST_LENGTH is the maximum number of CIDRs allowed
	// for an "acl whitelist src [<cidr>]*" config line.
	HAPROXY_MAX_WHITELIST_LENGTH = HAPROXY_MAX_LINE_ARGS - 3
)

// ValidateWhiteList validates a haproxy acl whitelist from an annotation value.
func ValidateWhiteList(value string) ([]string, bool) {
	values := strings.Split(value, " ")

	cidrs := make([]string, 0)
	for _, v := range values {
		if len(v) > 0 {
			cidrs = append(cidrs, v)
		}
	}

	return cidrs, len(cidrs) <= HAPROXY_MAX_WHITELIST_LENGTH
}

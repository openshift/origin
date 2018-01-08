package validation

import (
	"errors"
)

const imageRefWithoutDomain = "foo/bar"

// validateDomain validates that doamin (e.g., "myregistry.io") can be
// used as the domain component in a docker image reference. Returns
// an error if domain is invalid.
func validateDomain(domain string) error {
	matchedDomain, remainder, err := ParseDomainName(domain + "/" + imageRefWithoutDomain)
	if err != nil {
		return err
	}
	if domain != matchedDomain && remainder != imageRefWithoutDomain {
		return errors.New("invalid domain")
	}
	return nil
}

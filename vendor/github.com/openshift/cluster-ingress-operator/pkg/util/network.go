package k8s

import (
	"fmt"
	"net/url"
	"strconv"

	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	SchemeHTTPS = "https"
)

// URI validates uri as being a http(s) valid url and returns the url scheme.
func URI(uri string) (string, error) {
	parsed, err := url.ParseRequestURI(uri)
	if err != nil {
		return "", err
	}
	if port := parsed.Port(); len(port) != 0 {
		intPort, err := strconv.Atoi(port)
		if err != nil {
			return "", fmt.Errorf("failed converting port to integer for URI %q: %v", uri, err)
		}
		if err := Port(intPort); err != nil {
			return "", fmt.Errorf("failed to validate port for URL %q: %v", uri, err)
		}
	}

	return parsed.Scheme, nil
}

// Port validates if port is a valid port number between 1-65535.
func Port(port int) error {
	invalidPorts := validation.IsValidPortNum(port)
	if invalidPorts != nil {
		return fmt.Errorf("invalid port number: %d", port)
	}

	return nil
}

package flagtypes

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// urlPrefixes is the list of string prefix values that may indicate a URL
// is present.
var urlPrefixes = []string{"http://", "https://", "tcp://"}

// Addr is a flag type that attempts to load a host, IP, host:port, or
// URL value from a string argument. It tracks whether the value was set
// and allows the caller to provide defaults for the scheme and port.
type Addr struct {
	// Specified by the caller
	DefaultScheme string
	DefaultPort   int
	AllowPrefix   bool

	// Provided will be true if Set is invoked
	Provided bool
	// Value is the exact value provided on the flag
	Value string

	// URL represents the user input. The Host field is guaranteed
	// to be set if Provided is true
	URL *url.URL
	// Host is the hostname or IP portion of the user input
	Host string
	// Port is the port portion of the user input. Will be 0 if no port was found
	// and no default port could be established.
	Port int
}

// Default creates a new Address with the value set
func (a Addr) Default() Addr {
	if err := a.Set(a.Value); err != nil {
		panic(err)
	}
	a.Provided = false
	return a
}

// String returns the string representation of the Addr
func (a *Addr) String() string {
	if a.URL == nil {
		return a.Value
	}
	return a.URL.String()
}

// Set attempts to set a string value to an address
func (a *Addr) Set(value string) error {
	var addr *url.URL
	isURL := a.isURL(value)
	if isURL {
		parsed, err := url.Parse(value)
		if err != nil {
			return fmt.Errorf("not a valid URL: %v", err)
		}
		addr = parsed
	} else {
		addr = &url.URL{
			Scheme: a.DefaultScheme,
			Host:   value,
		}
		if len(addr.Scheme) == 0 {
			addr.Scheme = "tcp"
		}
	}

	if strings.Contains(addr.Host, ":") {
		host, port, err := net.SplitHostPort(addr.Host)
		if err != nil {
			return fmt.Errorf("not a valid host:port: %v", err)
		}
		portNum, err := strconv.ParseUint(port, 10, 64)
		if err != nil {
			return fmt.Errorf("not a valid port: %v", err)
		}
		a.Host = host
		a.Port = int(portNum)
	} else {
		port := 0
		if !isURL {
			port = a.DefaultPort
		}
		if port == 0 {
			switch addr.Scheme {
			case "http":
				port = 80
			case "https":
				port = 443
			default:
				return fmt.Errorf("no port specified")
			}
		}
		a.Host = addr.Host
		a.Port = port
		addr.Host = net.JoinHostPort(addr.Host, strconv.FormatInt(int64(a.Port), 10))
	}

	if !a.AllowPrefix {
		addr.Path = ""
	}
	addr.RawQuery = ""
	addr.Fragment = ""

	if value != a.Value {
		a.Provided = true
	}
	a.URL = addr
	a.Value = value

	return nil
}

// Type returns a string representation of what kind of value this is
func (a *Addr) Type() string {
	return "string"
}

// isURL returns true if the provided value appears to be a valid URL.
func (a *Addr) isURL(value string) bool {
	prefixes := urlPrefixes
	if a.DefaultScheme != "" {
		prefixes = append(prefixes, fmt.Sprintf("%s://", a.DefaultScheme))
	}
	for _, p := range prefixes {
		if strings.HasPrefix(value, p) {
			return true
		}
	}
	return false
}

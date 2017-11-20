package config

import (
	"crypto/x509"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	x509request "k8s.io/apiserver/pkg/authentication/request/x509"

	"k8s.io/apimachinery/third_party/forked/golang/netutil"
)

// GetClusterNicknameFromURL returns host:port of the apiServerLocation, with .'s replaced by -'s
func GetClusterNicknameFromURL(apiServerLocation string) (string, error) {
	u, err := url.Parse(apiServerLocation)
	if err != nil {
		return "", err
	}
	hostPort := netutil.CanonicalAddr(u)

	// we need a character other than "." to avoid conflicts with.  replace with '-'
	return strings.Replace(hostPort, ".", "-", -1), nil
}

func GetUserNicknameFromCert(clusterNick string, chain ...*x509.Certificate) (string, error) {
	userInfo, _, err := x509request.CommonNameUserConversion(chain)
	if err != nil {
		return "", err
	}

	return userInfo.GetName() + "/" + clusterNick, nil
}

func GetContextNickname(namespace, clusterNick, userNick string) string {
	tokens := strings.SplitN(userNick, "/", 2)
	return namespace + "/" + clusterNick + "/" + tokens[0]
}

var validURLSchemes = []string{"https://", "http://", "tcp://"}

// NormalizeServerURL is opinionated normalization of a string that represents a URL. Returns the URL provided matching the format
// expected when storing a URL in a config. Sets a scheme and port if not present, removes unnecessary trailing
// slashes, etc. Can be used to normalize a URL provided by user input.
func NormalizeServerURL(s string) (string, error) {
	// normalize scheme
	if !hasScheme(s) {
		s = validURLSchemes[0] + s
	}

	addr, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("Not a valid URL: %v.", err)
	}

	// normalize host:port
	if strings.Contains(addr.Host, ":") {
		_, port, err := net.SplitHostPort(addr.Host)
		if err != nil {
			return "", fmt.Errorf("Not a valid host:port: %v.", err)
		}
		_, err = strconv.ParseUint(port, 10, 16)
		if err != nil {
			return "", fmt.Errorf("Not a valid port: %v. Port numbers must be between 0 and 65535.", port)
		}
	} else {
		port := 0
		switch addr.Scheme {
		case "http":
			port = 80
		case "https":
			port = 443
		default:
			return "", fmt.Errorf("No port specified.")
		}
		addr.Host = net.JoinHostPort(addr.Host, strconv.FormatInt(int64(port), 10))
	}

	// remove trailing slash if that's the only path we have
	if addr.Path == "/" {
		addr.Path = ""
	}

	return addr.String(), nil
}

func hasScheme(s string) bool {
	for _, p := range validURLSchemes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

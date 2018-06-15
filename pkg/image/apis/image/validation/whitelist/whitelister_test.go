package whitelist

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/util/diff"

	serverapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

func mkAllowed(insecure bool, regs ...string) serverapi.AllowedRegistries {
	ret := make(serverapi.AllowedRegistries, 0, len(regs))
	for _, reg := range regs {
		ret = append(ret, serverapi.RegistryLocation{DomainName: reg, Insecure: insecure})
	}
	return ret
}

func TestRegistryWhitelister(t *testing.T) {
	for _, tc := range []struct {
		name      string
		transport WhitelistTransport
		whitelist serverapi.AllowedRegistries
		hostnames map[string]error
		difs      map[imageapi.DockerImageReference]error
		pullSpecs map[string]error
	}{
		{
			name:      "empty whitelist",
			transport: WhitelistTransportSecure,
			whitelist: mkAllowed(true),
			hostnames: map[string]error{
				"example.com":    fmt.Errorf(`registry "example.com" not allowed by empty whitelist`),
				"localhost:5000": fmt.Errorf(`registry "localhost:5000" not allowed by empty whitelist`),
			},
			difs: map[imageapi.DockerImageReference]error{
				{Registry: "docker.io", Namespace: "library", Name: "busybox"}: fmt.Errorf(`registry "docker.io" not allowed by empty whitelist`),
				{Name: "busybox"}:                                              fmt.Errorf(`registry "docker.io:443" not allowed by empty whitelist`),
			},
		},

		{
			name:      "allow any host with secure transport",
			transport: WhitelistTransportSecure,
			whitelist: mkAllowed(false, "*"),
			hostnames: map[string]error{
				"docker.io":      nil,
				"example.com":    nil,
				"localhost:443":  nil,
				"localhost:5000": fmt.Errorf(`registry "localhost:5000" not allowed by whitelist: "*:443"`),
				"localhost:80":   fmt.Errorf(`registry "localhost:80" not allowed by whitelist: "*:443"`),
				"localhost":      nil,
			},
			difs: map[imageapi.DockerImageReference]error{
				{Registry: "docker.io", Namespace: "library", Name: "busybox"}: nil,
			},
		},

		{
			name:      "allow any host with insecure transport",
			transport: WhitelistTransportInsecure,
			whitelist: mkAllowed(true, "*"),
			hostnames: map[string]error{
				"docker.io":      nil,
				"example.com":    nil,
				"localhost:443":  fmt.Errorf(`registry "localhost:443" not allowed by whitelist: "*:80"`),
				"localhost:5000": fmt.Errorf(`registry "localhost:5000" not allowed by whitelist: "*:80"`),
				"localhost:80":   nil,
				"localhost":      nil,
			},
			difs: map[imageapi.DockerImageReference]error{
				{Registry: "docker.io", Namespace: "library", Name: "busybox"}: nil,
			},
		},

		{
			name:      "allow any host with any transport",
			transport: WhitelistTransportAny,
			whitelist: mkAllowed(true, "*"),
			hostnames: map[string]error{
				"docker.io":      nil,
				"example.com":    nil,
				"localhost:443":  fmt.Errorf(`registry "localhost:443" not allowed by whitelist: "*:80"`),
				"localhost:5000": fmt.Errorf(`registry "localhost:5000" not allowed by whitelist: "*:80"`),
				"localhost:80":   nil,
				"localhost":      nil,
			},
			difs: map[imageapi.DockerImageReference]error{
				{Registry: "docker.io", Namespace: "library", Name: "busybox"}: nil,
			},
		},

		{
			name:      "allow any host:port with secure transport",
			transport: WhitelistTransportSecure,
			whitelist: mkAllowed(false, "*:*"),
			hostnames: map[string]error{
				"docker.io":      nil,
				"example.com":    nil,
				"localhost:443":  nil,
				"localhost:5000": nil,
				"localhost:80":   nil,
			},
			difs: map[imageapi.DockerImageReference]error{
				{Registry: "docker.io", Namespace: "library", Name: "busybox"}: nil,
				{Registry: "example.com", Namespace: "a", Name: "b"}:           nil,
				{Registry: "localhost:80", Namespace: "ns", Name: "repo"}:      nil,
				{Registry: "localhost:443", Namespace: "ns", Name: "repo"}:     nil,
				{Registry: "docker.io", Name: "busybox"}:                       nil,
				{Registry: "localhost:5000", Namespace: "my", Name: "app"}:     nil,
			},
		},

		{
			name:      "allow any host:port with insecure transport",
			transport: WhitelistTransportInsecure,
			whitelist: mkAllowed(true, "*:*"),
			hostnames: map[string]error{
				"localhost:5000": nil,
				"docker.io":      nil,
				"localhost:443":  nil,
				"localhost:80":   nil,
				"example.com":    nil,
			},
			difs: map[imageapi.DockerImageReference]error{
				{Registry: "docker.io", Namespace: "library", Name: "busybox"}: nil,
				{Registry: "example.com", Namespace: "a", Name: "b"}:           nil,
				{Registry: "localhost:80", Namespace: "ns", Name: "repo"}:      nil,
				{Registry: "localhost:443", Namespace: "ns", Name: "repo"}:     nil,
				{Registry: "docker.io", Name: "busybox"}:                       nil,
				{Registry: "localhost:5000", Namespace: "my", Name: "app"}:     nil,
			},
		},

		{
			name:      "allow any host:port with any transport",
			transport: WhitelistTransportAny,
			whitelist: mkAllowed(true, "*:*"),
			hostnames: map[string]error{
				"localhost:5000": nil,
				"docker.io":      nil,
				"localhost:443":  nil,
				"localhost:80":   nil,
				"example.com":    nil,
			},
			difs: map[imageapi.DockerImageReference]error{
				{Registry: "docker.io", Namespace: "library", Name: "busybox"}: nil,
				{Registry: "example.com", Namespace: "a", Name: "b"}:           nil,
				{Registry: "localhost:80", Namespace: "ns", Name: "repo"}:      nil,
				{Registry: "localhost:443", Namespace: "ns", Name: "repo"}:     nil,
				{Registry: "docker.io", Name: "busybox"}:                       nil,
				{Registry: "localhost:5000", Namespace: "my", Name: "app"}:     nil,
			},
		},

		{
			name:      "allow whitelisted with secure transport",
			transport: WhitelistTransportSecure,
			whitelist: mkAllowed(false, "localhost:5000", "docker.io", "example.com:*", "registry.com:80"),
			hostnames: map[string]error{
				"example.com:5000":         nil,
				"example.com:80":           nil,
				"example.com":              nil,
				"localhost:443":            fmt.Errorf(`registry "localhost:443" not allowed by whitelist: "localhost:5000", "docker.io:443", "example.com:*", "registry.com:80"`),
				"localhost:5000":           nil,
				"registry-1.docker.io:443": nil,
				"registry.com:443":         fmt.Errorf(`registry "registry.com:443" not allowed by whitelist: "localhost:5000", "docker.io:443", "example.com:*", "registry.com:80"`),
				"registry.com:80":          nil,
				"registry.com":             fmt.Errorf(`registry "registry.com" not allowed by whitelist: "localhost:5000", "docker.io:443", "example.com:*", "registry.com:80"`),
			},
			difs: map[imageapi.DockerImageReference]error{
				{Registry: "docker.io"}:            nil,
				{Registry: "index.docker.io"}:      nil,
				{Registry: "example.com"}:          nil,
				{Registry: "docker.io"}:            nil,
				{Registry: "localhost:5000"}:       nil,
				{Registry: "registry.example.com"}: fmt.Errorf(`registry "registry.example.com" not allowed by whitelist: "localhost:5000", "docker.io:443", "example.com:*", "registry.com:80"`),
				{Name: "busybox"}:                  nil,
			},
		},

		{
			name:      "allow whitelisted with insecure transport",
			transport: WhitelistTransportInsecure,
			whitelist: mkAllowed(true, "localhost:5000", "docker.io", "example.com:*", "registry.com:80", "*.foo.com", "*domain.ltd"),
			hostnames: map[string]error{
				"a.b.c.d.foo.com:80": nil,
				"domain.ltd":         nil,
				"example.com":        nil,
				"foo.com":            fmt.Errorf(`registry "foo.com" not allowed by whitelist: "localhost:5000", "docker.io:80", "example.com:*", "registry.com:80", and 2 more ...`),
				"index.docker.io":    nil,
				"localhost:5000":     nil,
				"my.domain.ltd:443":  fmt.Errorf(`registry "my.domain.ltd:443" not allowed by whitelist: "localhost:5000", "docker.io:80", "example.com:*", "registry.com:80", and 2 more ...`),
				"my.domain.ltd:80":   nil,
				"my.domain.ltd":      nil,
				"mydomain.ltd":       nil,
				"registry.com":       nil,
				"registry.foo.com":   nil,
			},
			difs: map[imageapi.DockerImageReference]error{
				{Registry: "docker.io", Namespace: "library", Name: "busybox"}: nil,
				{Registry: "foo.com", Namespace: "library", Name: "busybox"}:   fmt.Errorf(`registry "foo.com" not allowed by whitelist: "localhost:5000", "docker.io:80", "example.com:*", "registry.com:80", and 2 more ...`),
				{Registry: "ffoo.com", Namespace: "library", Name: "busybox"}:  fmt.Errorf(`registry "ffoo.com" not allowed by whitelist: "localhost:5000", "docker.io:80", "example.com:*", "registry.com:80", and 2 more ...`),
			},
		},

		{
			name:      "allow whitelisted with any transport",
			transport: WhitelistTransportAny,
			whitelist: mkAllowed(false, "localhost:5000", "docker.io", "example.com:*", "registry.com:80", "*.foo.com", "*domain.ltd"),
			hostnames: map[string]error{
				"a.b.c.d.foo.com:80": fmt.Errorf(`registry "a.b.c.d.foo.com:80" not allowed by whitelist: "localhost:5000", "docker.io:443", "example.com:*", "registry.com:80", and 2 more ...`),
				"domain.ltd":         nil,
				"example.com":        nil,
				"foo.com":            fmt.Errorf(`registry "foo.com" not allowed by whitelist: "localhost:5000", "docker.io:443", "example.com:*", "registry.com:80", and 2 more ...`),
				"index.docker.io":    nil,
				"localhost:5000":     nil,
				"my.domain.ltd:443":  nil,
				"my.domain.ltd:80":   fmt.Errorf(`registry "my.domain.ltd:80" not allowed by whitelist: "localhost:5000", "docker.io:443", "example.com:*", "registry.com:80", and 2 more ...`),
				"my.domain.ltd":      nil,
				"mydomain.ltd":       nil,
				"registry.com:443":   fmt.Errorf(`registry "registry.com:443" not allowed by whitelist: "localhost:5000", "docker.io:443", "example.com:*", "registry.com:80", and 2 more ...`),
				"registry.com":       nil,
				"registry.foo.com":   nil,
			},
			difs: map[imageapi.DockerImageReference]error{
				{Registry: "docker.io", Namespace: "library", Name: "busybox"}: nil,
				{Registry: "foo.com", Namespace: "library", Name: "busybox"}:   fmt.Errorf(`registry "foo.com" not allowed by whitelist: "localhost:5000", "docker.io:443", "example.com:*", "registry.com:80", and 2 more ...`),
				{Registry: "ffoo.com", Namespace: "library", Name: "busybox"}:  fmt.Errorf(`registry "ffoo.com" not allowed by whitelist: "localhost:5000", "docker.io:443", "example.com:*", "registry.com:80", and 2 more ...`),
			},
		},

		{
			name:      "allow whitelisted pullspecs with any transport",
			transport: WhitelistTransportAny,
			whitelist: mkAllowed(true, "localhost:5000", "docker.io", "example.com:*", "registry.com:80", "*.foo.com", "*domain.ltd"),
			pullSpecs: map[string]error{
				"a.b.c.d.foo.com:80/repo":     nil,
				"domain.ltd/a/b":              nil,
				"example.com/c/d":             nil,
				"foo.com/foo":                 fmt.Errorf(`registry "foo.com" not allowed by whitelist: "localhost:5000", "docker.io:80", "example.com:*", "registry.com:80", and 2 more ...`),
				"index.docker.io/bar":         nil,
				"localhost:5000/repo":         nil,
				"my.domain.ltd:443/a/b":       fmt.Errorf(`registry "my.domain.ltd:443" not allowed by whitelist: "localhost:5000", "docker.io:80", "example.com:*", "registry.com:80", and 2 more ...`),
				"my.domain.ltd:80/foo:latest": nil,
				"my.domain.ltd/bar:1.3.4":     nil,
				"mydomain.ltd/my/repo/sitory": nil,
				"registry.com:443/ab:tag":     fmt.Errorf(`registry "registry.com:443" not allowed by whitelist: "localhost:5000", "docker.io:80", "example.com:*", "registry.com:80", and 2 more ...`),
				"registry.com/repo":           nil,
				"registry.foo.com/123":        nil,
				"repository:latest":           nil,
				"nm/repo:latest":              nil,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rw, err := NewRegistryWhitelister(tc.whitelist, nil)
			if err != nil {
				t.Fatal(err)
			}

			for hn, expErr := range tc.hostnames {
				t.Run("hostname "+hn, func(t *testing.T) {
					err := rw.AdmitHostname(hn, tc.transport)
					assertExpectedError(t, err, expErr)
				})
			}

			for dif, expErr := range tc.difs {
				t.Run("dockerImageReference "+dif.String(), func(t *testing.T) {
					err := rw.AdmitDockerImageReference(&dif, tc.transport)
					assertExpectedError(t, err, expErr)
				})
			}

			for ps, expErr := range tc.pullSpecs {
				t.Run("pull spec "+ps, func(t *testing.T) {
					err := rw.AdmitPullSpec(ps, tc.transport)
					assertExpectedError(t, err, expErr)
				})

			}
		})
	}
}

func TestWhitelistRegistry(t *testing.T) {
	rwClean, err := NewRegistryWhitelister(serverapi.AllowedRegistries{}, nil)
	if err != nil {
		t.Fatal(err)
	}

	rw := rwClean.Copy()
	if err := rw.WhitelistRegistry("foo.com", WhitelistTransportAny); err != nil {
		t.Fatal(err)
	}
	exp := fmt.Errorf(`registry "sub.foo.com" not allowed by whitelist: "foo.com:443", "foo.com:80"`)
	if err := rw.AdmitHostname("sub.foo.com", WhitelistTransportAny); err == nil || err.Error() != exp.Error() {
		t.Fatalf("got unexpected error: %s", diff.ObjectGoPrintDiff(err, exp))
	}

	rw = rwClean.Copy()
	if err := rw.WhitelistRegistry("foo.com", WhitelistTransportInsecure); err != nil {
		t.Fatal(err)
	}
	exp = fmt.Errorf(`registry "sub.foo.com" not allowed by whitelist: "foo.com:80"`)
	if err := rw.AdmitHostname("sub.foo.com", WhitelistTransportAny); err == nil || err.Error() != exp.Error() {
		t.Fatalf("got unexpected error: %s", diff.ObjectGoPrintDiff(err, exp))
	}
	// add duplicate
	if err := rw.WhitelistRegistry("foo.com", WhitelistTransportInsecure); err != nil {
		t.Fatal(err)
	}
	exp = fmt.Errorf(`registry "sub.foo.com" not allowed by whitelist: "foo.com:80"`)
	if err := rw.AdmitHostname("sub.foo.com", WhitelistTransportAny); err == nil || err.Error() != exp.Error() {
		t.Fatalf("got unexpected error: %s", diff.ObjectGoPrintDiff(err, exp))
	}
	// add duplicate with different port
	if err := rw.WhitelistRegistry("foo.com", WhitelistTransportAny); err != nil {
		t.Fatal(err)
	}
	exp = fmt.Errorf(`registry "sub.foo.com" not allowed by whitelist: "foo.com:443", "foo.com:80"`)
	if err := rw.AdmitHostname("sub.foo.com", WhitelistTransportAny); err == nil || err.Error() != exp.Error() {
		t.Fatalf("got unexpected error: %s", diff.ObjectGoPrintDiff(err, exp))
	}
}

func TestNewRegistryWhitelister(t *testing.T) {
	for _, tc := range []struct {
		name          string
		insecure      bool
		whitelist     serverapi.AllowedRegistries
		expectedError error
	}{
		{
			name:      "chinese works",
			whitelist: mkAllowed(true, "先生，先生！"),
		},

		{
			name:          "don't try it with multiple colons though",
			whitelist:     mkAllowed(true, "0:1:2:3"),
			expectedError: fmt.Errorf(`failed to parse allowed registry "0:1:2:3": too many colons`),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewRegistryWhitelister(tc.whitelist, nil)
			assertExpectedError(t, err, tc.expectedError)
		})
	}
}

func assertExpectedError(t *testing.T, a, e error) {
	switch {
	case a == nil && e != nil:
		t.Errorf("got unexpected non-error; expected: %q", e)
	case a != nil && e == nil:
		t.Errorf("got unexpected error: %q", a)
	case a != nil && e != nil && a.Error() != e.Error():
		t.Errorf("got unexpected error: %s", diff.ObjectGoPrintDiff(a, e))
	}
}

// +build integration

package integration

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/dockerregistry"
)

const (
	pulpRegistryName        = "registry.access.redhat.com"
	dockerHubV2RegistryName = "index.docker.io"
	dockerHubV1RegistryName = "registry.hub.docker.com"
	quayRegistryName        = "quay.io"

	maxRetryCount = 4
	retryAfter    = time.Millisecond * 500
)

// retryWhenUnreachable invokes given function several times until it succeeds,
// returns network unrelated error or maximum number of attempts is reached.
// Should be used to wrap calls to remote registry to prevent test failures
// because of short-term outages.
func retryWhenUnreachable(t *testing.T, f func() error) error {
	timeout := retryAfter
	attempt := 0
	for err := f(); err != nil; err = f() {
		// Examples of catched error messages:
		//   Get https://registry.com/v2/: dial tcp registry.com:443: i/o timeout
		//   Get https://registry.com/v2/: dial tcp: lookup registry.com: no such host
		//   Get https://registry.com/v2/: dial tcp registry.com:443: getsockopt: connection refused
		//   Get https://registry.com/v2/: read tcp 127.0.0.1:39849->registry.com:443: read: connection reset by peer
		//   Get https://registry.com/v2/: net/http: request cancelled while waiting for connection
		//   Get https://registry.com/v2/: net/http: TLS handshake timeout
		//   the registry "https://registry.com/v2/ " could not be reached
		reachable := true
		for _, substr := range []string{
			"dial tcp",
			"read tcp",
			"net/http",
			"could not be reached",
		} {
			if strings.Contains(err.Error(), substr) {
				reachable = false
				break
			}
		}

		if reachable || attempt >= maxRetryCount {
			return err
		}

		t.Logf("registry unreachable \"%v\", retry in %s", err, timeout.String())
		time.Sleep(timeout)
		timeout = timeout * 2
		attempt += 1
	}
	return nil
}

func TestRegistryClientConnect(t *testing.T) {
	c := dockerregistry.NewClient()
	conn, err := c.Connect("docker.io", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"index.docker.io", "https://docker.io", "https://index.docker.io"} {
		otherConn, err := c.Connect(s, false)
		if err != nil {
			t.Errorf("%s: can't connect: %v", s, err)
			continue
		}
		if !reflect.DeepEqual(otherConn, conn) {
			t.Errorf("%s: did not reuse connection: %#v %#v", s, conn, otherConn)
		}
	}

	otherConn, err := c.Connect("index.docker.io:443", false)
	if err != nil || reflect.DeepEqual(otherConn, conn) {
		t.Errorf("should not have reused index.docker.io:443: %v", err)
	}

	if _, err := c.Connect("http://ba%3/", false); err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestRegistryClientConnectPulpRegistry(t *testing.T) {
	c := dockerregistry.NewClient()
	conn, err := c.Connect(pulpRegistryName, false)
	if err != nil {
		t.Fatal(err)
	}

	var image *dockerregistry.Image
	err = retryWhenUnreachable(t, func() error {
		image, err = conn.ImageByTag("library", "rhel", "latest")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(image.ID) == 0 {
		t.Fatalf("image had no ID: %#v", image)
	}
}

func TestRegistryClientDockerHubV2(t *testing.T) {
	c := dockerregistry.NewClient()
	conn, err := c.Connect(dockerHubV2RegistryName, false)
	if err != nil {
		t.Fatal(err)
	}

	var image *dockerregistry.Image
	err = retryWhenUnreachable(t, func() error {
		image, err = conn.ImageByTag("kubernetes", "guestbook", "latest")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(image.ID) == 0 {
		t.Fatalf("image had no ID: %#v", image)
	}
}

func TestRegistryClientDockerHubV1(t *testing.T) {
	c := dockerregistry.NewClient()
	// a v1 only path
	conn, err := c.Connect(dockerHubV1RegistryName, false)
	if err != nil {
		t.Fatal(err)
	}

	var image *dockerregistry.Image
	err = retryWhenUnreachable(t, func() error {
		image, err = conn.ImageByTag("kubernetes", "guestbook", "latest")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(image.ID) == 0 {
		t.Fatalf("image had no ID: %#v", image)
	}
}

func TestRegistryClientRegistryNotFound(t *testing.T) {
	conn, err := dockerregistry.NewClient().Connect("localhost:65000", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ImageByID("foo", "bar", "baz"); !dockerregistry.IsRegistryNotFound(err) {
		t.Error(err)
	}
}

func doTestRegistryClientImage(t *testing.T, registry, version string) {
	conn, err := dockerregistry.NewClient().Connect(registry, false)
	if err != nil {
		t.Fatal(err)
	}

	err = retryWhenUnreachable(t, func() error {
		_, err := conn.ImageByTag("openshift", "origin-not-found", "latest")
		return err
	})
	if err == nil || (!dockerregistry.IsRepositoryNotFound(err) && !dockerregistry.IsTagNotFound(err)) {
		t.Errorf("%s: unexpected error: %v", version, err)
	}

	var image *dockerregistry.Image
	err = retryWhenUnreachable(t, func() error {
		image, err = conn.ImageByTag("openshift", "origin", "latest")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(image.ContainerConfig.Entrypoint) == 0 {
		t.Errorf("%s: unexpected image: %#v", version, image)
	}
	if version == "v2" && !image.PullByID {
		t.Errorf("%s: should be able to pull by ID %s", version, image.ID)
	}

	var other *dockerregistry.Image
	err = retryWhenUnreachable(t, func() error {
		other, err = conn.ImageByID("openshift", "origin", image.ID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(other.ContainerConfig.Entrypoint, image.ContainerConfig.Entrypoint) {
		t.Errorf("%s: unexpected image: %#v", version, other)
	}
}

func TestRegistryClientImageV2(t *testing.T) {
	doTestRegistryClientImage(t, dockerHubV2RegistryName, "v2")
}

func TestRegistryClientImageV1(t *testing.T) {
	doTestRegistryClientImage(t, dockerHubV1RegistryName, "v1")
}

func TestRegistryClientQuayIOImage(t *testing.T) {
	conn, err := dockerregistry.NewClient().Connect("quay.io", false)
	if err != nil {
		t.Fatal(err)
	}
	err = retryWhenUnreachable(t, func() error {
		_, err := conn.ImageByTag("coreos", "etcd", "latest")
		return err
	})
	if err != nil {
		t.Skip("SKIPPING: unexpected error from quay.io: %v", err)
	}
}

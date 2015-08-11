// +build integration

package integration

import (
	"reflect"
	"testing"

	"github.com/openshift/origin/pkg/dockerregistry"
)

func TestRegistryClientConnect(t *testing.T) {
	c := dockerregistry.NewClient()
	conn, err := c.Connect("docker.io", false, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"index.docker.io", "https://docker.io", "https://index.docker.io"} {
		otherConn, err := c.Connect(s, false, true)
		if err != nil {
			t.Errorf("%s: can't connect: %v", s, err)
			continue
		}
		if !reflect.DeepEqual(otherConn, conn) {
			t.Errorf("%s: did not reuse connection: %#v %#v", s, conn, otherConn)
		}
	}

	otherConn, err := c.Connect("index.docker.io:443", false, true)
	if err != nil || reflect.DeepEqual(otherConn, conn) {
		t.Errorf("should not have reused index.docker.io:443: %v", err)
	}

	if _, err := c.Connect("http://ba%3/", false, true); err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestRegistryClientConnectPulpRegistry(t *testing.T) {
	c := dockerregistry.NewClient()
	conn, err := c.Connect("registry.access.redhat.com", false, true)
	if err != nil {
		t.Fatal(err)
	}
	image, err := conn.ImageByTag("library", "rhel", "latest")
	if err != nil {
		t.Fatalf("unable to retrieve image info: %v", err)
	}
	if len(image.ID) == 0 {
		t.Fatalf("image had no ID: %#v", image)
	}
}

func TestRegistryClientRegistryNotFound(t *testing.T) {
	conn, err := dockerregistry.NewClient().Connect("localhost:65000", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ImageByID("foo", "bar", "baz"); !dockerregistry.IsRegistryNotFound(err) {
		t.Error(err)
	}
}

func TestRegistryClientImage(t *testing.T) {
	for _, v2 := range []bool{true, false} {
		conn, err := dockerregistry.NewClient().Connect("", false, !v2)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := conn.ImageByTag("openshift", "origin-not-found", "latest"); !dockerregistry.IsRepositoryNotFound(err) && !dockerregistry.IsTagNotFound(err) {
			t.Errorf("V2=%t: unexpected error: %v", v2, err)
		}

		image, err := conn.ImageByTag("openshift", "origin", "latest")
		if err != nil {
			t.Fatalf("V2=%t: %v", v2, err)
		}
		if len(image.ContainerConfig.Entrypoint) == 0 {
			t.Errorf("V2=%t: unexpected image: %#v", v2, image)
		}
		if v2 && !image.PullByID {
			t.Errorf("V2=%t: should be able to pull by ID %s", v2, image.ID)
		}

		other, err := conn.ImageByID("openshift", "origin", image.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(other.ContainerConfig.Entrypoint, image.ContainerConfig.Entrypoint) {
			t.Errorf("V2=%t: unexpected image: %#v", v2, other)
		}
	}
}

func TestRegistryClientQuayIOImage(t *testing.T) {
	for _, v2 := range []bool{true, false} {
		conn, err := dockerregistry.NewClient().Connect("quay.io", false, v2)
		if err != nil {
			t.Fatal(err)
		}

		_, err = conn.ImageByTag("coreos", "etcd", "latest")
		if err != nil {
			t.Errorf("v2=%t: unexpected error: %v", v2, err)
		}
	}
}

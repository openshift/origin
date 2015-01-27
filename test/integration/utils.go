// +build integration

package integration

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"

	"github.com/coreos/go-etcd/etcd"
	dockerClient "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/capabilities"
)

const (
	testNamespace         = "integration-test"
	defaultDockerEndpoint = "unix:///var/run/docker.sock"
)

func newEtcdClient() *etcd.Client {
	etcdServers := []string{"http://127.0.0.1:4001"}

	etcdFromEnv := os.Getenv("ETCD_SERVER")
	if len(etcdFromEnv) > 0 {
		etcdServers = []string{etcdFromEnv}
	}

	return etcd.NewClient(etcdServers)
}

func init() {
	capabilities.SetForTests(capabilities.Capabilities{
		AllowPrivileged: true,
	})
	flag.Set("v", "5")
}

func logEtcd() {
	etcd.SetLogger(log.New(os.Stderr, "go-etcd", log.LstdFlags))
}

func requireEtcd() {
	if _, err := newEtcdClient().Get("/", false, false); err != nil {
		glog.Fatalf("unable to connect to etcd for integration testing: %v", err)
	}
}

// requireDocker ensures that a new docker client can be created and that a ListImages command can be run on the client
// or it fails with glog.Fatal
func requireDocker() {
	client, err := newDockerClient()

	if err != nil {
		glog.Fatalf("unable to create docker client for integration testing: %v", err)
	}

	//simple test to make sure you can take action with the client
	_, err = client.ListImages(dockerClient.ListImagesOptions{All: false})

	if err != nil {
		glog.Fatalf("unable to create docker client for integration testing: %v", err)
	}
}

// newDockerClient creates a docker client using the env var DOCKER_ENDPOINT or, if not supplied, uses the default
// docker endpoint /var/run/docker.sock
func newDockerClient() (*dockerClient.Client, error) {
	endpoint := os.Getenv("DOCKER_ENDPOINT")

	if len(endpoint) == 0 {
		endpoint = defaultDockerEndpoint
	}

	client, err := dockerClient.NewClient(endpoint)

	if err != nil {
		return nil, err
	}

	return client, nil
}

func withEtcdKey(f func(string)) {
	prefix := fmt.Sprintf("/test-%d", rand.Int63())
	defer newEtcdClient().Delete(prefix, true)
	f(prefix)
}

func deleteAllEtcdKeys() {
	client := newEtcdClient()
	keys, err := client.Get("/", false, false)
	if err != nil {
		glog.Fatalf("Unable to list root etcd keys: %v", err)
	}
	for _, node := range keys.Node.Nodes {
		if _, err := client.Delete(node.Key, true); err != nil {
			glog.Fatalf("Unable delete key: %v", err)
		}
	}
}

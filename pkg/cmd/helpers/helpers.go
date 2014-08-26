package helpers

import (
  "runtime"
  "strings"
  "os"

  "github.com/fsouza/go-dockerclient"
  "github.com/golang/glog"
)

func GoVersion() string {
  return strings.TrimLeft(runtime.Version(), "go")
}

func DockerServerVersion() (string, string) {
  addr := GetDockerEndpoint("")
  client, err := docker.NewClient(addr)
  if err != nil {
    glog.Fatal("Couldn't connect to docker.")
  }
  if err := client.Ping(); err != nil {
    glog.Errorf("WARNING: Docker could not be reached at %s.  Docker must be installed and running to start containers.\n%v", addr, err)
  }
  dockerVersion, err := client.Version()
  if err != nil {
    glog.Fatal("Unable to check docker version.")
  }
  version := dockerVersion.Get("Version")
  commit := dockerVersion.Get("GitCommit")
  return version, commit
}

func Env(key string, defaultValue string) string {
  val := os.Getenv(key)
  if len(val) == 0 {
    return defaultValue
  } else {
    return val
  }
}

func GetDockerEndpoint(dockerEndpoint string) string {
  var endpoint string
  if len(dockerEndpoint) > 0 {
    endpoint = dockerEndpoint
  } else if len(os.Getenv("DOCKER_HOST")) > 0 {
    endpoint = os.Getenv("DOCKER_HOST")
  } else {
    endpoint = "unix:///var/run/docker.sock"
  }
  return endpoint
}

/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package image

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

const (
	// Agnhost image
	Agnhost = iota
	// AgnhostPrivate image
	AgnhostPrivate
	// APIServer image
	APIServer
	// AppArmorLoader image
	AppArmorLoader
	// AuthenticatedAlpine image
	AuthenticatedAlpine
	// AuthenticatedWindowsNanoServer image
	AuthenticatedWindowsNanoServer
	// BusyBox image
	BusyBox
	// CheckMetadataConcealment image
	CheckMetadataConcealment
	// CudaVectorAdd image
	CudaVectorAdd
	// CudaVectorAdd2 image
	CudaVectorAdd2
	Dnsutils
	// EchoServer image
	EchoServer
	// Etcd image
	Etcd
	// GlusterDynamicProvisioner image
	GlusterDynamicProvisioner
	// Httpd image
	Httpd
	// HttpdNew image
	HttpdNew
	// InvalidRegistryImage image
	InvalidRegistryImage
	// IpcUtils image
	IpcUtils
	// JessieDnsutils image
	JessieDnsutils
	// Kitten image
	Kitten
	// Mounttest image
	Mounttest
	// MounttestUser image
	MounttestUser
	// Nautilus image
	Nautilus
	// NFSProvisioner image
	NFSProvisioner
	// Nginx image
	Nginx
	// NginxNew image
	NginxNew
	// Nonewprivs image
	Nonewprivs
	// NonRoot runs with a default user of 1234
	NonRoot
	// Pause - when these values are updated, also update cmd/kubelet/app/options/container_runtime.go
	// Pause image
	Pause
	// Perl image
	Perl
	// PrometheusDummyExporter image
	PrometheusDummyExporter
	// PrometheusToSd image
	PrometheusToSd
	// Redis image
	Redis
	// RegressionIssue74839 image
	RegressionIssue74839
	// ResourceConsumer image
	ResourceConsumer
	ResourceController
	// SdDummyExporter image
	SdDummyExporter
	StartupScript
	TestWebserver
	// VolumeNFSServer image
	VolumeNFSServer
	// VolumeISCSIServer image
	VolumeISCSIServer
	// VolumeGlusterServer image
	VolumeGlusterServer
	// VolumeRBDServer image
	VolumeRBDServer
)

// GetImageConfigs returns the map of imageConfigs by their constant.
func GetImageConfigs() map[int]Config {
	return imageConfigs
}

// GetConfig returns the Config object for an image.
func GetConfig(image int) Config {
	return imageConfigs[image]
}

// GetE2EImage returns the fully qualified URI to an image (including version).
func GetE2EImage(image int) string {
	return fmt.Sprintf("%s/%s:%s", imageConfigs[image].registry, imageConfigs[image].name, imageConfigs[image].version)
}

// GetE2EImage returns the fully qualified URI to an image (including version).
func (i *Config) GetE2EImage() string {
	return fmt.Sprintf("%s/%s:%s", i.registry, i.name, i.version)
}

// GetPauseImageName returns the pause image name with proper version.
func GetPauseImageName() string {
	return GetE2EImage(Pause)
}

func GetAlternateImageConfig(repository string) (defaults map[int]Config, updated map[int]Config) {
	defaults = defaultImageConfigs(defaultRegistryList())
	updated = defaultImageConfigsForRepository(repository)
	return
}

// ReplaceRegistryInImageURL replaces the registry in the image URL with a custom one.
func ReplaceRegistryInImageURL(imageURL string) (string, error) {
	parts := strings.Split(imageURL, "/")
	countParts := len(parts)
	registryAndUser := strings.Join(parts[:countParts-1], "/")

	list := registry

	switch registryAndUser {
	case "gcr.io/kubernetes-e2e-test-images":
		registryAndUser = list.E2eRegistry
	case "k8s.gcr.io":
		registryAndUser = list.GcRegistry
	case "gcr.io/k8s-authenticated-test":
		registryAndUser = list.PrivateRegistry
	case "gcr.io/google-samples":
		registryAndUser = list.SampleRegistry
	case "gcr.io/gke-release":
		registryAndUser = list.GcrReleaseRegistry
	case "docker.io/library":
		registryAndUser = list.DockerLibraryRegistry
	case "quay.io/k8scsi":
		registryAndUser = list.QuayK8sCSI
	default:
		if countParts == 1 {
			// We assume we found an image from docker hub library
			// e.g. openjdk -> docker.io/library/openjdk
			registryAndUser = list.DockerLibraryRegistry
			break
		}

		return "", fmt.Errorf("Registry: %s is missing in test/utils/image/manifest.go, please add the registry, otherwise the test will fail on air-gapped clusters", registryAndUser)
	}

	return fmt.Sprintf("%s/%s", registryAndUser, parts[countParts-1]), nil
}

// Config holds an images registry, name, and version
type Config struct {
	registry string
	name     string
	version  string
}

// SetRegistry sets an image registry in a Config struct
func (i *Config) SetRegistry(registry string) {
	i.registry = registry
}

// SetName sets an image name in a Config struct
func (i *Config) SetName(name string) {
	i.name = name
}

// SetVersion sets an image version in a Config struct
func (i *Config) SetVersion(version string) {
	i.version = version
}

var (
	registry     RegistryList
	imageConfigs map[int]Config
)

func init() {
	registry = defaultRegistryList()

	// user wants to mirror to another repo
	if repoList := os.Getenv("KUBE_TEST_REPO_LIST"); len(repoList) > 0 {
		fileContent, err := ioutil.ReadFile(repoList)
		if err != nil {
			panic(fmt.Errorf("Error reading '%v' file contents: %v", repoList, err))
		}

		err = yaml.Unmarshal(fileContent, &registry)
		if err != nil {
			panic(fmt.Errorf("Error unmarshalling '%v' YAML file: %v", repoList, err))
		}
	}

	// user wants all images to be coming from a single repo, except those that are special cased
	if repo := os.Getenv("KUBE_TEST_REPO"); len(repo) > 0 {
		imageConfigs = defaultImageConfigsForRepository(repo)
		return
	}

	// use the images mapped according to the registry list
	imageConfigs = defaultImageConfigs(registry)
}

// RegistryList holds public and private image registries
type RegistryList struct {
	GcAuthenticatedRegistry string `yaml:"gcAuthenticatedRegistry"`
	DockerLibraryRegistry   string `yaml:"dockerLibraryRegistry"`
	DockerGluster           string `yaml:"dockerGluster"`
	E2eRegistry             string `yaml:"e2eRegistry"`
	PromoterE2eRegistry     string `yaml:"promoterE2eRegistry"`
	InvalidRegistry         string `yaml:"invalidRegistry"`
	GcRegistry              string `yaml:"gcRegistry"`
	GcrReleaseRegistry      string `yaml:"gcrReleaseRegistry"`
	// TODO: The last consumer of this has been removed and it should be deprecated
	GoogleContainerRegistry string `yaml:"googleContainerRegistry"`
	PrivateRegistry         string `yaml:"privateRegistry"`
	SampleRegistry          string `yaml:"sampleRegistry"`
	QuayK8sCSI              string `yaml:"quayK8sCSI"`
	QuayIncubator           string `yaml:"quayIncubator"`
}

// defaultRegistryList returns the default registries constants are pulled from.
func defaultRegistryList() RegistryList {
	return RegistryList{
		GcAuthenticatedRegistry: "gcr.io/authenticated-image-pulling",
		DockerLibraryRegistry:   "docker.io/library",
		DockerGluster:           "docker.io/gluster",
		E2eRegistry:             "gcr.io/kubernetes-e2e-test-images",
		// TODO: After the domain flip, this should instead be k8s.gcr.io/k8s-artifacts-prod/e2e-test-images
		PromoterE2eRegistry: "us.gcr.io/k8s-artifacts-prod/e2e-test-images",
		InvalidRegistry:     "invalid.com/invalid",
		GcRegistry:          "k8s.gcr.io",
		GcrReleaseRegistry:  "gcr.io/gke-release",
		// TODO: The last consumer of this has been removed and it should be deleted
		GoogleContainerRegistry: "gcr.io/google-containers",
		PrivateRegistry:         "gcr.io/k8s-authenticated-test",
		SampleRegistry:          "gcr.io/google-samples",
		QuayK8sCSI:              "quay.io/k8scsi",
		QuayIncubator:           "quay.io/kubernetes_incubator",
	}
}

// defaultImageConfigsForRepository generates an image config map that allows all
// public images to be accessed from a single repository. This is used by those
// who wish to mirror their images inside a private network. The mapping is
// the full original name -> <repository>:e2e-[ID]-[HASH_OF_NAME]-mirror. The
// trailing '-mirror' segment is required to ensure the name forms a valid
// container tag.
func defaultImageConfigsForRepository(repository string) map[int]Config {
	parts := strings.SplitN(repository, "/", 2)
	if len(parts) == 1 {
		panic("repository provided via env must be of the form REGISTRY/NAME")
	}
	defaults := defaultImageConfigs(defaultRegistryList())
	h := sha256.New()
	reCharSafe := regexp.MustCompile(`[^\w]`)
	reDashes := regexp.MustCompile(`-+`)
	for i, config := range defaults {
		switch i {
		case InvalidRegistryImage, AuthenticatedAlpine,
			AuthenticatedWindowsNanoServer, AgnhostPrivate:
			// These images are special and can't be run out of the cloud - some because they
			// are authenticated, and others because they are not real images. Tests that depend
			// on these images can't be run without access to the public internet.
			continue
		}
		pullSpec := config.GetE2EImage()

		// Build a new tag with a the index, a hash of the image spec (to be unique) and
		// shorten and make the pull spec "safe" so it will fit in the tag
		h.Reset()
		h.Write([]byte(pullSpec))
		hash := base64.RawURLEncoding.EncodeToString(h.Sum(nil)[:16])
		shortName := reCharSafe.ReplaceAllLiteralString(pullSpec, "-")
		shortName = reDashes.ReplaceAllLiteralString(shortName, "-")
		maxLength := 127 - 16 - 6 - 10
		if len(shortName) > maxLength {
			shortName = shortName[len(shortName)-maxLength:]
		}
		newTag := fmt.Sprintf("e2e-%d-%s-%s", i, shortName, hash)

		config.SetRegistry(parts[0])
		config.SetName(parts[1])
		config.SetVersion(newTag)
		defaults[i] = config
	}
	return defaults
}

// defaultImageConfigs returns a map of images by their constant based on the provided RegistryList.
func defaultImageConfigs(list RegistryList) map[int]Config {
	configs := map[int]Config{}
	configs[Agnhost] = Config{list.PromoterE2eRegistry, "agnhost", "2.12"}
	configs[AgnhostPrivate] = Config{list.PrivateRegistry, "agnhost", "2.6"}
	configs[AuthenticatedAlpine] = Config{list.GcAuthenticatedRegistry, "alpine", "3.7"}
	configs[AuthenticatedWindowsNanoServer] = Config{list.GcAuthenticatedRegistry, "windows-nanoserver", "v1"}
	configs[APIServer] = Config{list.E2eRegistry, "sample-apiserver", "1.17"}
	configs[AppArmorLoader] = Config{list.E2eRegistry, "apparmor-loader", "1.0"}
	configs[BusyBox] = Config{list.DockerLibraryRegistry, "busybox", "1.29"}
	configs[CheckMetadataConcealment] = Config{list.E2eRegistry, "metadata-concealment", "1.2"}
	configs[CudaVectorAdd] = Config{list.E2eRegistry, "cuda-vector-add", "1.0"}
	configs[CudaVectorAdd2] = Config{list.E2eRegistry, "cuda-vector-add", "2.0"}
	configs[Dnsutils] = Config{list.E2eRegistry, "dnsutils", "1.1"}
	configs[EchoServer] = Config{list.E2eRegistry, "echoserver", "2.2"}
	configs[Etcd] = Config{list.GcRegistry, "etcd", "3.4.4"}
	configs[GlusterDynamicProvisioner] = Config{list.DockerGluster, "glusterdynamic-provisioner", "v1.0"}
	configs[Httpd] = Config{list.DockerLibraryRegistry, "httpd", "2.4.38-alpine"}
	configs[HttpdNew] = Config{list.DockerLibraryRegistry, "httpd", "2.4.39-alpine"}
	configs[InvalidRegistryImage] = Config{list.InvalidRegistry, "alpine", "3.1"}
	configs[IpcUtils] = Config{list.E2eRegistry, "ipc-utils", "1.0"}
	configs[JessieDnsutils] = Config{list.E2eRegistry, "jessie-dnsutils", "1.0"}
	configs[Kitten] = Config{list.E2eRegistry, "kitten", "1.0"}
	configs[Mounttest] = Config{list.E2eRegistry, "mounttest", "1.0"}
	configs[MounttestUser] = Config{list.E2eRegistry, "mounttest-user", "1.0"}
	configs[Nautilus] = Config{list.E2eRegistry, "nautilus", "1.0"}
	configs[NFSProvisioner] = Config{list.QuayIncubator, "nfs-provisioner", "v2.2.2"}
	configs[Nginx] = Config{list.DockerLibraryRegistry, "nginx", "1.14-alpine"}
	configs[NginxNew] = Config{list.DockerLibraryRegistry, "nginx", "1.15-alpine"}
	configs[Nonewprivs] = Config{list.E2eRegistry, "nonewprivs", "1.0"}
	configs[NonRoot] = Config{list.E2eRegistry, "nonroot", "1.0"}
	// Pause - when these values are updated, also update cmd/kubelet/app/options/container_runtime.go
	configs[Pause] = Config{list.GcRegistry, "pause", "3.2"}
	configs[Perl] = Config{list.DockerLibraryRegistry, "perl", "5.26"}
	configs[PrometheusDummyExporter] = Config{list.GcRegistry, "prometheus-dummy-exporter", "v0.1.0"}
	configs[PrometheusToSd] = Config{list.GcRegistry, "prometheus-to-sd", "v0.5.0"}
	configs[Redis] = Config{list.DockerLibraryRegistry, "redis", "5.0.5-alpine"}
	configs[RegressionIssue74839] = Config{list.E2eRegistry, "regression-issue-74839-amd64", "1.0"}
	configs[ResourceConsumer] = Config{list.E2eRegistry, "resource-consumer", "1.5"}
	configs[ResourceController] = Config{list.E2eRegistry, "resource-consumer-controller", "1.0"}
	configs[SdDummyExporter] = Config{list.GcRegistry, "sd-dummy-exporter", "v0.2.0"}
	configs[StartupScript] = Config{list.GoogleContainerRegistry, "startup-script", "v1"}
	configs[TestWebserver] = Config{list.E2eRegistry, "test-webserver", "1.0"}
	configs[VolumeNFSServer] = Config{list.E2eRegistry, "volume/nfs", "1.0"}
	configs[VolumeISCSIServer] = Config{list.E2eRegistry, "volume/iscsi", "2.0"}
	configs[VolumeGlusterServer] = Config{list.E2eRegistry, "volume/gluster", "1.0"}
	configs[VolumeRBDServer] = Config{list.E2eRegistry, "volume/rbd", "1.0.1"}
	return configs
}

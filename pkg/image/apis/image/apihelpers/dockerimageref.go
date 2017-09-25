package apihelpers

import (
	"net"
	"strings"

	"net/url"

	"github.com/openshift/origin/pkg/image/apis/image/internal/digest"
	"github.com/openshift/origin/pkg/image/apis/image/internal/reference"
)

const (
	// DockerDefaultNamespace is the value for namespace when a single segment name is provided.
	DockerDefaultNamespace = "library"
	// DockerDefaultRegistry is the value for the registry when none was provided.
	DockerDefaultRegistry = "docker.io"
	// DockerDefaultV1Registry is the host name of the default v1 registry
	DockerDefaultV1Registry = "index." + DockerDefaultRegistry
	// DockerDefaultV2Registry is the host name of the default v2 registry
	DockerDefaultV2Registry = "registry-1." + DockerDefaultRegistry

	DefaultImageTag = "latest"
)

type DockerImageReference interface {
	GetRegistry() string
	GetNamespace() string
	GetName() string
	GetTag() string
	GetID() string
}

type namedDockerImageReference struct {
	registry  string
	namespace string
	name      string
	tag       string
	id        string
}

func (r *namedDockerImageReference) GetRegistry() string {
	return r.registry
}
func (r *namedDockerImageReference) GetNamespace() string {
	return r.namespace
}
func (r *namedDockerImageReference) GetName() string {
	return r.name
}
func (r *namedDockerImageReference) GetTag() string {
	return r.tag
}
func (r *namedDockerImageReference) GetID() string {
	return r.id
}
func copyDockerImageReference(r DockerImageReference) *namedDockerImageReference {
	return &namedDockerImageReference{
		registry:  r.GetRegistry(),
		namespace: r.GetNamespace(),
		name:      r.GetName(),
		tag:       r.GetTag(),
		id:        r.GetID(),
	}
}

// ParseDockerImageReference parses a Docker pull spec string into a
// DockerImageReference.
func ParseDockerImageReference(spec string) (DockerImageReference, error) {
	ref := &namedDockerImageReference{}

	namedRef, err := parseNamedDockerImageReference(spec)
	if err != nil {
		return ref, err
	}

	ref.registry = namedRef.GetRegistry()
	ref.namespace = namedRef.GetNamespace()
	ref.name = namedRef.GetName()
	ref.tag = namedRef.GetTag()
	ref.id = namedRef.GetID()

	return ref, nil
}

// parseNamedDockerImageReference parses a Docker pull spec string into a
// NamedDockerImageReference.
func parseNamedDockerImageReference(spec string) (DockerImageReference, error) {
	ref := &namedDockerImageReference{}

	namedRef, err := reference.ParseNamed(spec)
	if err != nil {
		return ref, err
	}

	name := namedRef.Name()
	i := strings.IndexRune(name, '/')
	if i == -1 || (!strings.ContainsAny(name[:i], ":.") && name[:i] != "localhost") {
		ref.name = name
	} else {
		ref.registry, ref.name = name[:i], name[i+1:]
	}

	if named, ok := namedRef.(reference.NamedTagged); ok {
		ref.tag = named.Tag()
	}

	if named, ok := namedRef.(reference.Canonical); ok {
		ref.id = named.Digest().String()
	}

	// It's not enough just to use the reference.ParseNamed(). We have to fill
	// ref.Namespace from ref.Name
	if i := strings.IndexRune(ref.name, '/'); i != -1 {
		ref.namespace, ref.name = ref.name[:i], ref.name[i+1:]
	}

	return ref, nil
}

// Equal returns true if the other DockerImageReference is equivalent to the
// reference r. The comparison applies defaults to the Docker image reference,
// so that e.g., "foobar" equals "docker.io/library/foobar:latest".
func Equal(r DockerImageReference, other DockerImageReference) bool {
	defaultedRef := DockerClientDefaults(r)
	otherDefaultedRef := DockerClientDefaults(other)
	return defaultedRef == otherDefaultedRef
}

// DockerClientDefaults sets the default values used by the Docker client.
func DockerClientDefaults(r DockerImageReference) DockerImageReference {
	ret := r
	if len(r.GetRegistry()) == 0 {
		newRef := copyDockerImageReference(r)
		newRef.registry = DockerDefaultRegistry
		ret = newRef
	}
	if len(r.GetNamespace()) == 0 && IsRegistryDockerHub(r.GetRegistry()) {
		newRef := copyDockerImageReference(r)
		newRef.namespace = DockerDefaultNamespace
		ret = newRef
	}
	if len(r.GetTag()) == 0 {
		newRef := copyDockerImageReference(r)
		newRef.tag = DefaultImageTag
		ret = newRef
	}
	return ret
}

// minimal reduces a DockerImageReference to its minimalist form.
func minimal(r DockerImageReference) DockerImageReference {
	ret := r
	if r.GetTag() == DefaultImageTag {
		newRef := copyDockerImageReference(r)
		newRef.tag = ""
		ret = newRef
	}
	return ret
}

// AsRepository returns the reference without tags or IDs.
func AsRepository(r DockerImageReference) DockerImageReference {
	newRef := copyDockerImageReference(r)
	newRef.tag = ""
	newRef.id = ""
	return newRef
}

// RepositoryName returns the registry relative name
func RepositoryName(r DockerImageReference) string {
	newRef := copyDockerImageReference(r)
	newRef.tag = ""
	newRef.id = ""
	newRef.registry = ""
	return Exact(r)
}

// RegistryHostPort returns the registry hostname and the port.
// If the port is not specified in the registry hostname we default to 443.
// This will also default to Docker client defaults if the registry hostname is empty.
func RegistryHostPort(r DockerImageReference, insecure bool) (string, string) {
	registryHost := DockerClientDefaults(asV2(r)).GetRegistry()
	if strings.Contains(registryHost, ":") {
		hostname, port, _ := net.SplitHostPort(registryHost)
		return hostname, port
	}
	if insecure {
		return registryHost, "80"
	}
	return registryHost, "443"
}

// RepositoryName returns the registry relative name
func RegistryURL(r DockerImageReference) *url.URL {
	return &url.URL{
		Scheme: "https",
		Host:   asV2(r).GetRegistry(),
	}
}

// DaemonMinimal clears defaults that Docker assumes.
func DaemonMinimal(r DockerImageReference) DockerImageReference {
	ret := r
	switch r.GetRegistry() {
	case DockerDefaultV1Registry, DockerDefaultV2Registry:
		newRef := copyDockerImageReference(r)
		newRef.registry = DockerDefaultRegistry
		ret = newRef
	}
	if IsRegistryDockerHub(r.GetRegistry()) && r.GetNamespace() == DockerDefaultNamespace {
		newRef := copyDockerImageReference(r)
		newRef.namespace = ""
		ret = newRef
	}
	return minimal(ret)
}

func asV2(r DockerImageReference) DockerImageReference {
	ret := r
	switch r.GetRegistry() {
	case DockerDefaultV1Registry, DockerDefaultRegistry:
		newRef := copyDockerImageReference(r)
		newRef.registry = DockerDefaultV2Registry
		ret = newRef
	}
	return ret
}

// MostSpecific returns the most specific image reference that can be constructed from the
// current ref, preferring an ID over a Tag. Allows client code dealing with both tags and IDs
// to get the most specific reference easily.
func MostSpecific(r DockerImageReference) DockerImageReference {
	if len(r.GetID()) == 0 {
		return r
	}
	if _, err := digest.ParseDigest(r.GetID()); err == nil {
		newRef := copyDockerImageReference(r)
		newRef.tag = ""
		return newRef
	}
	if len(r.GetTag()) == 0 {
		newRef := copyDockerImageReference(r)
		newRef.tag = r.GetID()
		newRef.id = ""
		return newRef
	}
	return r
}

// NameString returns the name of the reference with its tag or ID.
func NameString(r DockerImageReference) string {
	switch {
	case len(r.GetName()) == 0:
		return ""
	case len(r.GetTag()) > 0:
		return r.GetName() + ":" + r.GetTag()
	case len(r.GetID()) > 0:
		var ref string
		if _, err := digest.ParseDigest(r.GetID()); err == nil {
			// if it parses as a digest, its v2 pull by id
			ref = "@" + r.GetID()
		} else {
			// if it doesn't parse as a digest, it's presumably a v1 registry by-id tag
			ref = ":" + r.GetID()
		}
		return r.GetName() + ref
	default:
		return r.GetName()
	}
}

// Exact returns a string representation of the set fields on the DockerImageReference
func Exact(r DockerImageReference) string {
	name := NameString(r)
	if len(name) == 0 {
		return name
	}
	s := r.GetRegistry()
	if len(s) > 0 {
		s += "/"
	}

	if len(r.GetNamespace()) != 0 {
		s += r.GetNamespace() + "/"
	}
	return s + name
}

// String converts a DockerImageReference to a Docker pull spec (which implies a default namespace
// according to V1 Docker registry rules). Use Exact() if you want no defaulting.
func String(r DockerImageReference) string {
	toCheck := r
	if len(r.GetNamespace()) == 0 && IsRegistryDockerHub(r.GetRegistry()) {
		newRef := copyDockerImageReference(r)
		newRef.namespace = DockerDefaultNamespace
		toCheck = newRef
	}
	return Exact(toCheck)
}

// IsRegistryDockerHub returns true if the given registry name belongs to
// Docker hub.
func IsRegistryDockerHub(registry string) bool {
	switch registry {
	case DockerDefaultRegistry, DockerDefaultV1Registry, DockerDefaultV2Registry:
		return true
	default:
		return false
	}
}

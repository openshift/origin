package imageprune

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	kapi "k8s.io/kubernetes/pkg/apis/core"

	"github.com/docker/distribution/registry/api/errcode"
	"github.com/golang/glog"

	kerrors "k8s.io/apimachinery/pkg/util/errors"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/util/netutils"
)

// order younger images before older
type imgByAge []*imageapi.Image

func (ba imgByAge) Len() int      { return len(ba) }
func (ba imgByAge) Swap(i, j int) { ba[i], ba[j] = ba[j], ba[i] }
func (ba imgByAge) Less(i, j int) bool {
	return ba[i].CreationTimestamp.After(ba[j].CreationTimestamp.Time)
}

// order younger image stream before older
type isByAge []imageapi.ImageStream

func (ba isByAge) Len() int      { return len(ba) }
func (ba isByAge) Swap(i, j int) { ba[i], ba[j] = ba[j], ba[i] }
func (ba isByAge) Less(i, j int) bool {
	return ba[i].CreationTimestamp.After(ba[j].CreationTimestamp.Time)
}

// DetermineRegistryHost returns registry host embedded in a pull-spec of the latest unmanaged image or the
// latest imagestream from the provided lists. If no such pull-spec is found, error is returned.
func DetermineRegistryHost(images *imageapi.ImageList, imageStreams *imageapi.ImageStreamList) (string, error) {
	var pullSpec string
	var managedImages []*imageapi.Image

	// 1st try to determine registry url from a pull spec of the youngest managed image
	for i := range images.Items {
		image := &images.Items[i]
		if image.Annotations[imageapi.ManagedByOpenShiftAnnotation] != "true" {
			continue
		}
		managedImages = append(managedImages, image)
	}
	// be sure to pick up the newest managed image which should have an up to date information
	sort.Sort(imgByAge(managedImages))

	if len(managedImages) > 0 {
		pullSpec = managedImages[0].DockerImageReference
	} else {
		// 2nd try to get the pull spec from any image stream
		// Sorting by creation timestamp may not get us up to date info. Modification time would be much
		// better if there were such an attribute.
		sort.Sort(isByAge(imageStreams.Items))
		for _, is := range imageStreams.Items {
			if len(is.Status.DockerImageRepository) == 0 {
				continue
			}
			pullSpec = is.Status.DockerImageRepository
		}
	}

	if len(pullSpec) == 0 {
		return "", fmt.Errorf("no managed image found")
	}

	ref, err := imageapi.ParseDockerImageReference(pullSpec)
	if err != nil {
		return "", fmt.Errorf("unable to parse %q: %v", pullSpec, err)
	}

	if len(ref.Registry) == 0 {
		return "", fmt.Errorf("%s does not include a registry", pullSpec)
	}

	return ref.Registry, nil
}

// RegistryPinger performs a health check against a registry.
type RegistryPinger interface {
	// Ping performs a health check against registry. It returns registry url qualified with schema unless an
	// error occurs.
	Ping(registry string) (*url.URL, error)
}

// DefaultRegistryPinger implements RegistryPinger.
type DefaultRegistryPinger struct {
	Client   *http.Client
	Insecure bool
}

// Ping verifies that the integrated registry is ready, determines its transport protocol and returns its url
// or error.
func (drp *DefaultRegistryPinger) Ping(registry string) (*url.URL, error) {
	var (
		registryURL *url.URL
		err         error
	)

pathLoop:
	// first try the new default / path, then fall-back to the obsolete /healthz endpoint
	for _, path := range []string{"/", "/healthz"} {
		registryURL, err = TryProtocolsWithRegistryURL(registry, drp.Insecure, func(u url.URL) error {
			u.Path = path
			healthResponse, err := drp.Client.Get(u.String())
			if err != nil {
				return err
			}
			defer healthResponse.Body.Close()

			if healthResponse.StatusCode != http.StatusOK {
				return &retryPath{err: fmt.Errorf("unexpected status: %s", healthResponse.Status)}
			}

			return nil
		})

		// determine whether to retry with another endpoint
		switch t := err.(type) {
		case *retryPath:
			// return the nested error if this is the last ping attempt
			err = t.err
			continue pathLoop
		case kerrors.Aggregate:
			// if any aggregated error indicates a possible retry, do it
			for _, err := range t.Errors() {
				if _, ok := err.(*retryPath); ok {
					continue pathLoop
				}
			}
		}

		break
	}

	return registryURL, err
}

// DryRunRegistryPinger implements RegistryPinger.
type DryRunRegistryPinger struct {
}

// Ping implements Ping method.
func (*DryRunRegistryPinger) Ping(registry string) (*url.URL, error) {
	return url.Parse("https://" + registry)
}

// TryProtocolsWithRegistryURL runs given action with different protocols until no error is returned. The
// https protocol is the first attempt. If it fails and allowInsecure is true, http will be the next. Obtained
// errors will be concatenated and returned.
func TryProtocolsWithRegistryURL(registry string, allowInsecure bool, action func(registryURL url.URL) error) (*url.URL, error) {
	var errs []error

	if !strings.Contains(registry, "://") {
		registry = "unset://" + registry
	}
	url, err := url.Parse(registry)
	if err != nil {
		return nil, err
	}
	var protos []string
	switch {
	case len(url.Scheme) > 0 && url.Scheme != "unset":
		protos = []string{url.Scheme}
	case allowInsecure || netutils.IsPrivateAddress(registry):
		protos = []string{"https", "http"}
	default:
		protos = []string{"https"}
	}
	registry = url.Host

	for _, proto := range protos {
		glog.V(4).Infof("Trying protocol %s for the registry URL %s", proto, registry)
		url.Scheme = proto
		err := action(*url)
		if err == nil {
			return url, nil
		}

		if err != nil {
			glog.V(4).Infof("Error with %s for %s: %v", proto, registry, err)
		}

		if _, ok := err.(*errcode.Errors); ok {
			// we got a response back from the registry, so return it
			return url, err
		}
		errs = append(errs, err)
		if proto == "https" && strings.Contains(err.Error(), "server gave HTTP response to HTTPS client") && !allowInsecure {
			errs = append(errs, fmt.Errorf("\n* Append --force-insecure if you really want to prune the registry using insecure connection."))
		} else if proto == "http" && strings.Contains(err.Error(), "malformed HTTP response") {
			errs = append(errs, fmt.Errorf("\n* Are you trying to connect to a TLS-enabled registry without TLS?"))
		}
	}

	return nil, kerrors.NewAggregate(errs)
}

// retryPath is an error indicating that another connection attempt may be retried with a different path
type retryPath struct{ err error }

func (rp *retryPath) Error() string { return rp.err.Error() }

// ErrBadReference denotes an invalid reference to image, imagestreamtag or imagestreamimage stored in a
// particular object. The object is identified by kind, namespace and name.
type ErrBadReference struct {
	kind       string
	namespace  string
	name       string
	targetKind string
	reference  string
	reason     string
}

func newErrBadReferenceToImage(reference string, obj *kapi.ObjectReference, reason string) error {
	kind := "<UnknownType>"
	namespace := ""
	name := "<unknown-name>"
	if obj != nil {
		kind = obj.Kind
		namespace = obj.Namespace
		name = obj.Name
	}

	return &ErrBadReference{
		kind:      kind,
		namespace: namespace,
		name:      name,
		reference: reference,
		reason:    reason,
	}
}

func newErrBadReferenceTo(targetKind, reference string, obj *kapi.ObjectReference, reason string) error {
	return &ErrBadReference{
		kind:       obj.Kind,
		namespace:  obj.Namespace,
		name:       obj.Name,
		targetKind: targetKind,
		reference:  reference,
		reason:     reason,
	}
}

func (e *ErrBadReference) Error() string {
	return e.String()
}

func (e *ErrBadReference) String() string {
	name := e.name
	if len(e.namespace) > 0 {
		name = e.namespace + "/" + name
	}
	targetKind := "docker image"
	if len(e.targetKind) > 0 {
		targetKind = e.targetKind
	}
	return fmt.Sprintf("%s[%s]: invalid %s reference %q: %s", e.kind, name, targetKind, e.reference, e.reason)
}

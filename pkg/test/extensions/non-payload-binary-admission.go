package extensions

import (
	"context"
	"strings"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	testextensionv1 "github.com/openshift/origin/pkg/apis/testextension/v1"
)

const (
	// ComponentAnnotation is the annotation on ImageStreamTags that advertise a non-payload extension.
	// Value format: product-name-component-name.
	ComponentAnnotation = "testextension.redhat.io/component"
	// BinaryAnnotation is the annotation on ImageStreamTags that identifies the binary path
	// and optional args. Value format: <binary-path>.gz [--argument=value]
	BinaryAnnotation = "testextension.redhat.io/binary"
)

// PermitPattern represents one entry from TestExtensionAdmission spec.permit.
// Namespace and ImageStream may be "*" to match any.
// ImageStream is the ImageStream resource name only (tag is irrelevant for matching).
type PermitPattern struct {
	Namespace   string
	ImageStream string
}

// ParsePermitPattern parses a permit string "namespace/imagestream" into a PermitPattern.
// Supports "*" for namespace and/or imagestream (e.g. "openshift/*", "*/*", "ns/stream").
func ParsePermitPattern(permit string) (PermitPattern, bool) {
	parts := strings.SplitN(permit, "/", 2)
	if len(parts) != 2 {
		return PermitPattern{}, false
	}
	ns := strings.TrimSpace(parts[0])
	is := strings.TrimSpace(parts[1])
	if ns == "" || is == "" {
		return PermitPattern{}, false
	}
	return PermitPattern{Namespace: ns, ImageStream: is}, true
}

// MatchesAnyPermit returns true if (namespace, imagestream) matches any of the patterns.
func MatchesAnyPermit(namespace, imagestream string, patterns []PermitPattern) bool {
	for _, p := range patterns {
		nsMatch := p.Namespace == "*" || p.Namespace == namespace
		isMatch := p.ImageStream == "*" || p.ImageStream == imagestream
		if nsMatch && isMatch {
			return true
		}
	}
	return false
}

// DiscoverNonPayloadBinaryAdmission checks for the TestExtensionAdmission CRD, lists instances, and builds the permit set
func DiscoverNonPayloadBinaryAdmission(ctx context.Context, config *rest.Config) ([]PermitPattern, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	gv := testextensionv1.SchemeGroupVersion
	_, err = discoveryClient.ServerResourcesForGroupVersion(gv.String())
	if err != nil {
		if isNotFound(err) {
			logrus.Infof("TestExtensionAdmission CRD not found; will discover extension ImageStreamTags cluster-wide and report all as unpermitted")
			return nil, nil
		}
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	gvr := schema.GroupVersionResource{
		Group:    testextensionv1.GroupName,
		Version:  testextensionv1.Version,
		Resource: "testextensionadmissions",
	}
	list, err := dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	var patterns []PermitPattern
	for _, item := range list.Items {
		var admission testextensionv1.TestExtensionAdmission
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &admission)
		if err != nil {
			logrus.Warnf("Failed to convert TestExtensionAdmission: %v", err)
			continue
		}

		for _, permitStr := range admission.Spec.Permit {
			pat, ok := ParsePermitPattern(permitStr)
			if !ok {
				logrus.Warnf("Invalid permit pattern in TestExtensionAdmission %q: %q", admission.Name, permitStr)
				continue
			}
			patterns = append(patterns, pat)
		}
	}

	return patterns, nil
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "not found") ||
		strings.Contains(err.Error(), "NotFound") ||
		strings.Contains(err.Error(), "the server could not find the requested resource")
}

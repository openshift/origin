package limitrange

import (
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/openshift/api/image"
	imagev1 "github.com/openshift/api/image/v1"
	"github.com/openshift/origin/pkg/api/legacy"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/util/testutil"
)

func TestAdmitImageStreamMapping(t *testing.T) {
	tests := map[string]struct {
		imageStreamMapping *imageapi.ImageStreamMapping
		limitRange         *corev1.LimitRange
		shouldAdmit        bool
		operation          admission.Operation
	}{
		"new ism, no limit range": {
			imageStreamMapping: getImageStreamMapping(),
			operation:          admission.Create,
			shouldAdmit:        true,
		},
		"new ism, under limit range": {
			imageStreamMapping: getImageStreamMapping(),
			limitRange:         getLimitRange("1Ki"),
			operation:          admission.Create,
			shouldAdmit:        true,
		},
		"new ism, over limit range": {
			imageStreamMapping: getImageStreamMapping(),
			limitRange:         getLimitRange("0Ki"),
			operation:          admission.Create,
			shouldAdmit:        false,
		},
	}

	for k, v := range tests {
		var fakeKubeClient kubernetes.Interface
		if v.limitRange != nil {
			fakeKubeClient = fake.NewSimpleClientset(v.limitRange)
		} else {
			fakeKubeClient = fake.NewSimpleClientset()
		}
		plugin, informerFactory, err := newHandlerForTest(fakeKubeClient)
		if err != nil {
			t.Errorf("%s failed creating plugin %v", k, err)
			continue
		}
		informerFactory.Start(wait.NeverStop)

		attrs := admission.NewAttributesRecord(v.imageStreamMapping, nil,
			image.Kind("ImageStreamMapping").WithVersion("version"),
			v.imageStreamMapping.Namespace,
			v.imageStreamMapping.Name,
			image.Resource("imagestreammappings").WithVersion("version"),
			"",
			v.operation,
			false,
			nil)

		err = plugin.(admission.MutationInterface).Admit(attrs, nil)
		if v.shouldAdmit && err != nil {
			t.Errorf("%s expected to be admitted but received error %v", k, err)
		}
		if !v.shouldAdmit && err == nil {
			t.Errorf("%s expected to be rejected but received no error", k)
		}
	}
}

func TestAdmitImage(t *testing.T) {
	tests := map[string]struct {
		size           resource.Quantity
		limitSize      resource.Quantity
		shouldAdmit    bool
		limitRangeItem *corev1.LimitRangeItem
	}{
		"under size": {
			size:        resource.MustParse("50Mi"),
			limitSize:   resource.MustParse("100Mi"),
			shouldAdmit: true,
		},
		"equal size": {
			size:        resource.MustParse("100Mi"),
			limitSize:   resource.MustParse("100Mi"),
			shouldAdmit: true,
		},
		"over size": {
			size:        resource.MustParse("101Mi"),
			limitSize:   resource.MustParse("100Mi"),
			shouldAdmit: false,
		},
		"non-applicable limit range item": {
			size: resource.MustParse("100Mi"),
			limitRangeItem: &corev1.LimitRangeItem{
				Type: corev1.LimitTypeContainer,
			},
			shouldAdmit: true,
		},
	}

	for k, v := range tests {
		limitRangeItem := v.limitRangeItem
		if limitRangeItem == nil {
			limitRangeItem = &corev1.LimitRangeItem{
				Type: imagev1.LimitTypeImage,
				Max: corev1.ResourceList{
					corev1.ResourceStorage: v.limitSize,
				},
			}
		}

		err := admitImage(v.size.Value(), *limitRangeItem)
		if v.shouldAdmit && err != nil {
			t.Errorf("%s expected to be admitted but received error %v", k, err)
		}

		if !v.shouldAdmit && err == nil {
			t.Errorf("%s expected to be denied but was admitted", k)
		}
	}
}

func TestLimitAppliestoImages(t *testing.T) {
	tests := map[string]struct {
		limitRange  *corev1.LimitRange
		shouldApply bool
	}{
		"good limit range": {
			limitRange: &corev1.LimitRange{
				Spec: corev1.LimitRangeSpec{
					Limits: []corev1.LimitRangeItem{
						{
							Type: imagev1.LimitTypeImage,
						},
					},
				},
			},
			shouldApply: true,
		},
		"bad limit range": {
			limitRange: &corev1.LimitRange{
				Spec: corev1.LimitRangeSpec{
					Limits: []corev1.LimitRangeItem{
						{
							Type: corev1.LimitTypeContainer,
						},
					},
				},
			},
			shouldApply: false,
		},
		"malformed range with no type": {
			limitRange: &corev1.LimitRange{
				Spec: corev1.LimitRangeSpec{
					Limits: []corev1.LimitRangeItem{},
				},
			},
			shouldApply: false,
		},
		"malformed range with no limits": {
			limitRange:  &corev1.LimitRange{},
			shouldApply: false,
		},
	}

	plugin, err := NewImageLimitRangerPlugin(nil)
	if err != nil {
		t.Fatalf("error creating plugin: %v", err)
	}
	ilr := plugin.(*imageLimitRangerPlugin)

	for k, v := range tests {
		supports := ilr.SupportsLimit(v.limitRange)
		if supports && !v.shouldApply {
			t.Errorf("%s expected limit range to not be applicable", k)
		}
		if !supports && v.shouldApply {
			t.Errorf("%s expected limit range to be applicable", k)
		}
	}
}

func TestHandles(t *testing.T) {
	plugin, err := NewImageLimitRangerPlugin(nil)
	if err != nil {
		t.Fatalf("error creating plugin: %v", err)
	}
	if !plugin.Handles(admission.Create) {
		t.Errorf("plugin is expected to handle create")
	}
	if plugin.Handles(admission.Update) {
		t.Errorf("plugin is not expected to handle update")
	}
	if plugin.Handles(admission.Delete) {
		t.Errorf("plugin is not expected to handle delete")
	}
	if plugin.Handles(admission.Connect) {
		t.Errorf("plugin is expected to handle connect")
	}
}

func TestSupports(t *testing.T) {
	resources := []string{"imagestreammappings"}
	plugin, err := NewImageLimitRangerPlugin(nil)
	if err != nil {
		t.Fatalf("error creating plugin: %v", err)
	}
	ilr := plugin.(*imageLimitRangerPlugin)
	for _, r := range resources {
		attr := admission.NewAttributesRecord(nil, nil, legacy.Kind("ImageStreamMapping").WithVersion(""), "ns", "name", legacy.Resource(r).WithVersion("version"), "", admission.Create, false, nil)
		if !ilr.SupportsAttributes(attr) {
			t.Errorf("plugin is expected to support %#v", r)
		}
	}

	badKinds := []string{"ImageStream", "Image", "Pod", "foo"}
	for _, k := range badKinds {
		attr := admission.NewAttributesRecord(nil, nil, legacy.Kind(k).WithVersion(""), "ns", "name", image.Resource("bar").WithVersion("version"), "", admission.Create, false, nil)
		if ilr.SupportsAttributes(attr) {
			t.Errorf("plugin is not expected to support %s", k)
		}
	}
}

func getBaseImageWith1Layer() imageapi.Image {
	return imageapi.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name:        testutil.BaseImageWith1LayerDigest,
			Annotations: map[string]string{imagev1.ManagedByOpenShiftAnnotation: "true"},
		},
		DockerImageReference: fmt.Sprintf("registry.example.org/%s/%s", "test", testutil.BaseImageWith1LayerDigest),
		DockerImageManifest:  testutil.BaseImageWith1Layer,
	}
}

func getLimitRange(limit string) *corev1.LimitRange {
	return &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-limit",
			Namespace: "test",
		},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				{
					Type: imagev1.LimitTypeImage,
					Max: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse(limit),
					},
				},
			},
		},
	}
}

func getImageStreamMapping() *imageapi.ImageStreamMapping {
	return &imageapi.ImageStreamMapping{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ism",
			Namespace: "test",
		},
		Image: getBaseImageWith1Layer(),
	}
}

func newHandlerForTest(c kubernetes.Interface) (admission.Interface, informers.SharedInformerFactory, error) {
	plugin, err := NewImageLimitRangerPlugin(nil)
	if err != nil {
		return nil, nil, err
	}
	f := informers.NewSharedInformerFactory(c, 5*time.Minute)
	castPlugin := plugin.(*imageLimitRangerPlugin)
	castPlugin.SetExternalKubeInformerFactory(f)
	castPlugin.SetExternalKubeClientSet(c)
	err = admission.ValidateInitialization(plugin)
	return plugin, f, err
}

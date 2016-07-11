package admission

import (
	"fmt"
	"testing"

	kadmission "k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/api/unversioned"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	clientsetfake "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	"github.com/openshift/origin/pkg/image/admission/testutil"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func TestAdmitImageStreamMapping(t *testing.T) {
	tests := map[string]struct {
		imageStreamMapping *imageapi.ImageStreamMapping
		limitRange         *kapi.LimitRange
		shouldAdmit        bool
		operation          kadmission.Operation
	}{
		"new ism, no limit range": {
			imageStreamMapping: getImageStreamMapping(),
			operation:          kadmission.Create,
			shouldAdmit:        true,
		},
		"new ism, under limit range": {
			imageStreamMapping: getImageStreamMapping(),
			limitRange:         getLimitRange("1Ki"),
			operation:          kadmission.Create,
			shouldAdmit:        true,
		},
		"new ism, over limit range": {
			imageStreamMapping: getImageStreamMapping(),
			limitRange:         getLimitRange("0Ki"),
			operation:          kadmission.Create,
			shouldAdmit:        false,
		},
	}

	for k, v := range tests {
		var fakeKubeClient clientset.Interface
		if v.limitRange != nil {
			fakeKubeClient = clientsetfake.NewSimpleClientset(v.limitRange)
		} else {
			fakeKubeClient = clientsetfake.NewSimpleClientset()
		}

		plugin, err := NewImageLimitRangerPlugin(fakeKubeClient, nil)
		if err != nil {
			t.Errorf("%s failed creating plugin %v", k, err)
			continue
		}

		attrs := kadmission.NewAttributesRecord(v.imageStreamMapping, nil,
			imageapi.Kind("ImageStreamMapping").WithVersion("version"),
			v.imageStreamMapping.Namespace,
			v.imageStreamMapping.Name,
			imageapi.Resource("imagestreammappings").WithVersion("version"),
			"",
			v.operation,
			nil)

		err = plugin.Admit(attrs)
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
		limitRangeItem *kapi.LimitRangeItem
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
			limitRangeItem: &kapi.LimitRangeItem{
				Type: kapi.LimitTypeContainer,
			},
			shouldAdmit: true,
		},
	}

	for k, v := range tests {
		limitRangeItem := v.limitRangeItem
		if limitRangeItem == nil {
			limitRangeItem = &kapi.LimitRangeItem{
				Type: imageapi.LimitTypeImage,
				Max: kapi.ResourceList{
					kapi.ResourceStorage: v.limitSize,
				},
			}
		}

		err := AdmitImage(v.size.Value(), *limitRangeItem)
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
		limitRange  *kapi.LimitRange
		shouldApply bool
	}{
		"good limit range": {
			limitRange: &kapi.LimitRange{
				Spec: kapi.LimitRangeSpec{
					Limits: []kapi.LimitRangeItem{
						{
							Type: imageapi.LimitTypeImage,
						},
					},
				},
			},
			shouldApply: true,
		},
		"bad limit range": {
			limitRange: &kapi.LimitRange{
				Spec: kapi.LimitRangeSpec{
					Limits: []kapi.LimitRangeItem{
						{
							Type: kapi.LimitTypeContainer,
						},
					},
				},
			},
			shouldApply: false,
		},
		"malformed range with no type": {
			limitRange: &kapi.LimitRange{
				Spec: kapi.LimitRangeSpec{
					Limits: []kapi.LimitRangeItem{},
				},
			},
			shouldApply: false,
		},
		"malformed range with no limits": {
			limitRange:  &kapi.LimitRange{},
			shouldApply: false,
		},
	}

	plugin, err := NewImageLimitRangerPlugin(clientsetfake.NewSimpleClientset(), nil)
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
	plugin, err := NewImageLimitRangerPlugin(clientsetfake.NewSimpleClientset(), nil)
	if err != nil {
		t.Fatalf("error creating plugin: %v", err)
	}
	if !plugin.Handles(kadmission.Create) {
		t.Errorf("plugin is expected to handle create")
	}
	if plugin.Handles(kadmission.Update) {
		t.Errorf("plugin is not expected to handle update")
	}
	if plugin.Handles(kadmission.Delete) {
		t.Errorf("plugin is not expected to handle delete")
	}
	if plugin.Handles(kadmission.Connect) {
		t.Errorf("plugin is expected to handle connect")
	}
}

func TestSupports(t *testing.T) {
	resources := []string{"imagestreammappings"}
	plugin, err := NewImageLimitRangerPlugin(clientsetfake.NewSimpleClientset(), nil)
	if err != nil {
		t.Fatalf("error creating plugin: %v", err)
	}
	ilr := plugin.(*imageLimitRangerPlugin)
	for _, r := range resources {
		attr := kadmission.NewAttributesRecord(nil, nil, unversioned.Kind("ImageStreamMapping").WithVersion("version"), "ns", "name", imageapi.Resource(r).WithVersion("version"), "", kadmission.Create, nil)
		if !ilr.SupportsAttributes(attr) {
			t.Errorf("plugin is expected to support %s", r)
		}
	}

	badKinds := []string{"ImageStream", "Image", "Pod", "foo"}
	for _, k := range badKinds {
		attr := kadmission.NewAttributesRecord(nil, nil, unversioned.Kind(k).WithVersion("version"), "ns", "name", imageapi.Resource("bar").WithVersion("version"), "", kadmission.Create, nil)
		if ilr.SupportsAttributes(attr) {
			t.Errorf("plugin is not expected to support %s", k)
		}
	}
}

func getBaseImageWith1Layer() imageapi.Image {
	return imageapi.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name:        testutil.BaseImageWith1LayerDigest,
			Annotations: map[string]string{imageapi.ManagedByOpenShiftAnnotation: "true"},
		},
		DockerImageReference: fmt.Sprintf("registry.example.org/%s/%s", "test", testutil.BaseImageWith1LayerDigest),
		DockerImageManifest:  testutil.BaseImageWith1Layer,
	}
}

func getLimitRange(limit string) *kapi.LimitRange {
	return &kapi.LimitRange{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "test-limit",
			Namespace: "test",
		},
		Spec: kapi.LimitRangeSpec{
			Limits: []kapi.LimitRangeItem{
				{
					Type: imageapi.LimitTypeImage,
					Max: kapi.ResourceList{
						kapi.ResourceStorage: resource.MustParse(limit),
					},
				},
			},
		},
	}
}

func getImageStreamMapping() *imageapi.ImageStreamMapping {
	return &imageapi.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "test-ism",
			Namespace: "test",
		},
		Image: getBaseImageWith1Layer(),
	}
}

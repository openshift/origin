package admission

import (
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kadmission "k8s.io/apiserver/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	informers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	kubeadmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"

	"github.com/openshift/origin/pkg/image/admission/testutil"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
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
		var fakeKubeClient kclientset.Interface
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
	plugin, err := NewImageLimitRangerPlugin(nil)
	if err != nil {
		t.Fatalf("error creating plugin: %v", err)
	}
	ilr := plugin.(*imageLimitRangerPlugin)
	for _, r := range resources {
		attr := kadmission.NewAttributesRecord(nil, nil, imageapi.LegacyKind("ImageStreamMapping").WithVersion(""), "ns", "name", imageapi.LegacyResource(r).WithVersion("version"), "", kadmission.Create, nil)
		if !ilr.SupportsAttributes(attr) {
			t.Errorf("plugin is expected to support %#v", r)
		}
	}

	badKinds := []string{"ImageStream", "Image", "Pod", "foo"}
	for _, k := range badKinds {
		attr := kadmission.NewAttributesRecord(nil, nil, imageapi.LegacyKind(k).WithVersion(""), "ns", "name", imageapi.Resource("bar").WithVersion("version"), "", kadmission.Create, nil)
		if ilr.SupportsAttributes(attr) {
			t.Errorf("plugin is not expected to support %s", k)
		}
	}
}

func getBaseImageWith1Layer() imageapi.Image {
	return imageapi.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name:        testutil.BaseImageWith1LayerDigest,
			Annotations: map[string]string{imageapi.ManagedByOpenShiftAnnotation: "true"},
		},
		DockerImageReference: fmt.Sprintf("registry.example.org/%s/%s", "test", testutil.BaseImageWith1LayerDigest),
		DockerImageManifest:  testutil.BaseImageWith1Layer,
	}
}

func getLimitRange(limit string) *kapi.LimitRange {
	return &kapi.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
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
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ism",
			Namespace: "test",
		},
		Image: getBaseImageWith1Layer(),
	}
}

func newHandlerForTest(c kclientset.Interface) (kadmission.Interface, informers.SharedInformerFactory, error) {
	plugin, err := NewImageLimitRangerPlugin(nil)
	if err != nil {
		return nil, nil, err
	}
	f := informers.NewSharedInformerFactory(c, 5*time.Minute)
	pluginInitializer := kubeadmission.NewPluginInitializer(c, nil, f, nil, nil, nil, nil)
	pluginInitializer.Initialize(plugin)
	err = kadmission.Validate(plugin)
	return plugin, f, err
}

package annotations

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	kapps "k8s.io/api/apps/v1beta1"
	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/client-go/util/jsonpath"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"

	triggerapi "github.com/openshift/origin/pkg/image/apis/image/v1/trigger"
)

type fakeTagResponse struct {
	Namespace string
	Name      string
	Ref       string
	RV        int64
}

type fakeTagRetriever []fakeTagResponse

func (r fakeTagRetriever) ImageStreamTag(namespace, name string) (string, int64, bool) {
	for _, resp := range r {
		if resp.Namespace != namespace || resp.Name != name {
			continue
		}
		return resp.Ref, resp.RV, true
	}
	return "", 0, false
}

type fakeUpdater struct {
	Object runtime.Object
	Err    error
}

func (u *fakeUpdater) Update(obj runtime.Object) error {
	u.Object = obj
	return u.Err
}

func testStatefulSet(params []triggerapi.ObjectFieldTrigger, containers map[string]string) *kapps.StatefulSet {
	obj := &kapps.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: kapps.StatefulSetSpec{
			Template: kapiv1.PodTemplateSpec{},
		},
	}
	data, _ := json.Marshal(params)
	obj.Annotations = map[string]string{triggerapi.TriggerAnnotationKey: string(data)}
	var names, initNames []string
	for k := range containers {
		if strings.HasPrefix(k, "-") {
			initNames = append(initNames, k[1:])
		} else {
			names = append(names, k)
		}
	}
	sort.Sort(sort.StringSlice(initNames))
	sort.Sort(sort.StringSlice(names))
	for _, name := range initNames {
		obj.Spec.Template.Spec.InitContainers = append(obj.Spec.Template.Spec.InitContainers, kapiv1.Container{Name: name, Image: containers["-"+name]})
	}
	for _, name := range names {
		obj.Spec.Template.Spec.Containers = append(obj.Spec.Template.Spec.Containers, kapiv1.Container{Name: name, Image: containers[name]})
	}
	return obj
}

func TestAnnotationJSONPath(t *testing.T) {
	_, err := jsonpath.Parse("field_path", "spec.template.spec.containers[?(@.name==\"test\")].image")
	if err != nil {
		t.Error(err)
	}
}

func TestAnnotationsReactor(t *testing.T) {
	testCases := []struct {
		tags        []fakeTagResponse
		obj         *kapps.StatefulSet
		response    *kapps.StatefulSet
		expected    *kapps.StatefulSet
		expectedErr bool
	}{
		{
			obj: &kapps.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			},
		},

		{
			// no container, expect error
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testStatefulSet([]triggerapi.ObjectFieldTrigger{
				{
					From:      triggerapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					FieldPath: "spec.template.spec.containers[?(@.name==\"test\")].image",
				},
			}, nil),
			expectedErr: true,
		},

		{
			// container, but path spec is wrong, expect error
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testStatefulSet([]triggerapi.ObjectFieldTrigger{
				{
					From:      triggerapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					FieldPath: "spec.template.spec.containers[?(@.name==\"test\")]",
				},
			}, map[string]string{"test": ""}),
			expectedErr: true,
		},
		{
			// container, but path spec is wrong, expect error
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testStatefulSet([]triggerapi.ObjectFieldTrigger{
				{
					From:      triggerapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					FieldPath: "spec.template.spec.containers[?(@.name==\"test\").image",
				},
			}, map[string]string{"test": ""}),
			expectedErr: true,
		},
		{
			// container, but path spec is wrong, expect error
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testStatefulSet([]triggerapi.ObjectFieldTrigger{
				{
					From:      triggerapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					FieldPath: "spec.template.spec.containers[@.name=test].image",
				},
			}, map[string]string{"test": ""}),
			expectedErr: true,
		},

		{
			// no ref, no change
			obj: testStatefulSet([]triggerapi.ObjectFieldTrigger{
				{
					From:      triggerapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					FieldPath: "spec.template.spec.containers[?(@.name==\"test\")].image",
				},
			}, map[string]string{"test": ""}),
		},

		{
			// resolved without a change in another namespace
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testStatefulSet([]triggerapi.ObjectFieldTrigger{
				{
					From:      triggerapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					FieldPath: "spec.template.spec.containers[?(@.name==\"test\")].image",
				},
			}, map[string]string{"test": ""}),
			response: &kapps.StatefulSet{},
			expected: testStatefulSet([]triggerapi.ObjectFieldTrigger{
				{
					From:      triggerapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					FieldPath: "spec.template.spec.containers[?(@.name==\"test\")].image",
				},
			}, map[string]string{"test": "image-lookup-1"}),
		},

		{
			// resolved for init containers
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testStatefulSet([]triggerapi.ObjectFieldTrigger{
				{
					From:      triggerapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					FieldPath: "spec.template.spec.initContainers[?(@.name==\"test\")].image",
				},
			}, map[string]string{"-test": ""}),
			response: &kapps.StatefulSet{},
			expected: testStatefulSet([]triggerapi.ObjectFieldTrigger{
				{
					From:      triggerapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					FieldPath: "spec.template.spec.initContainers[?(@.name==\"test\")].image",
				},
			}, map[string]string{"-test": "image-lookup-1"}),
		},

		{
			// will not resolve if not automatic
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testStatefulSet([]triggerapi.ObjectFieldTrigger{
				{
					Paused:    true,
					From:      triggerapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					FieldPath: "spec.template.spec.containers[?(@.name==\"test\")].image",
				},
			}, map[string]string{"test": ""}),
			response: &kapps.StatefulSet{},
		},

		{
			// will fire if only one trigger resolves
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testStatefulSet([]triggerapi.ObjectFieldTrigger{
				{
					From:      triggerapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					FieldPath: "spec.template.spec.containers[?(@.name==\"test\")].image",
				},
				{
					From:      triggerapi.ObjectReference{Name: "stream-2:1", Namespace: "other", Kind: "ImageStreamTag"},
					FieldPath: "spec.template.spec.containers[?(@.name==\"test2\")].image",
				},
			}, map[string]string{"test": "", "test2": ""}),
			response: &kapps.StatefulSet{},
			expected: testStatefulSet([]triggerapi.ObjectFieldTrigger{
				{
					From:      triggerapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					FieldPath: "spec.template.spec.containers[?(@.name==\"test\")].image",
				},
				{
					From:      triggerapi.ObjectReference{Name: "stream-2:1", Namespace: "other", Kind: "ImageStreamTag"},
					FieldPath: "spec.template.spec.containers[?(@.name==\"test2\")].image",
				},
			}, map[string]string{"test": "image-lookup-1", "test2": ""}),
		},

		{
			// will fire if a trigger has already been resolved before
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testStatefulSet([]triggerapi.ObjectFieldTrigger{
				{
					From:      triggerapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					FieldPath: "spec.template.spec.containers[?(@.name==\"test\")].image",
				},
				{
					From:      triggerapi.ObjectReference{Name: "stream-2:1", Namespace: "other", Kind: "ImageStreamTag"},
					FieldPath: "spec.template.spec.containers[?(@.name==\"test2\")].image",
				},
			}, map[string]string{"test": "", "test2": "old-image"}),
			response: &kapps.StatefulSet{},
			expected: testStatefulSet([]triggerapi.ObjectFieldTrigger{
				{
					From:      triggerapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					FieldPath: "spec.template.spec.containers[?(@.name==\"test\")].image",
				},
				{
					From:      triggerapi.ObjectReference{Name: "stream-2:1", Namespace: "other", Kind: "ImageStreamTag"},
					FieldPath: "spec.template.spec.containers[?(@.name==\"test2\")].image",
				},
			}, map[string]string{"test": "image-lookup-1", "test2": "old-image"}),
		},

		{
			// will fire if both triggers are resolved
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testStatefulSet([]triggerapi.ObjectFieldTrigger{
				{
					From:      triggerapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					FieldPath: "spec.template.spec.containers[?(@.name==\"test\")].image",
				},
				{
					From:      triggerapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					FieldPath: "spec.template.spec.containers[?(@.name==\"test2\")].image",
				},
			}, map[string]string{"test": "", "test2": ""}),
			response: &kapps.StatefulSet{},
			expected: testStatefulSet([]triggerapi.ObjectFieldTrigger{
				{
					From:      triggerapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					FieldPath: "spec.template.spec.containers[?(@.name==\"test\")].image",
				},
				{
					From:      triggerapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					FieldPath: "spec.template.spec.containers[?(@.name==\"test2\")].image",
				},
			}, map[string]string{"test": "image-lookup-1", "test2": "image-lookup-1"}),
		},
	}

	for i, test := range testCases {
		u := &fakeUpdater{}
		r := AnnotationReactor{Updater: u}
		initial := test.obj.DeepCopy()
		err := r.ImageChanged(test.obj, fakeTagRetriever(test.tags))
		if !kapihelper.Semantic.DeepEqual(initial, test.obj) {
			t.Errorf("%d: should not have mutated: %s", i, diff.ObjectReflectDiff(initial, test.obj))
		}
		switch {
		case err == nil && test.expectedErr, err != nil && !test.expectedErr:
			t.Errorf("%d: unexpected error: %v", i, err)
			continue
		case err != nil:
			continue
		}
		if test.expected != nil {
			if u.Object == nil {
				t.Errorf("%d: no response defined", i)
				continue
			}
			if !reflect.DeepEqual(test.expected, u.Object) {
				t.Errorf("%d: not equal: %s", i, diff.ObjectReflectDiff(test.expected, u.Object))
				continue
			}
		} else {
			if u.Object != nil {
				t.Errorf("%d: unexpected update: %v", i, u.Object)
				continue
			}
		}
	}
}

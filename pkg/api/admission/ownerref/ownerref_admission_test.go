package ownerref

import (
	"testing"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
)

func TestAdmission(t *testing.T) {
	testCases := []struct {
		name          string
		attributes    admission.Attributes
		expectFailure bool
	}{
		{
			name: "reject ownerref",
			attributes: admission.NewAttributesRecord(&kapi.Pod{ObjectMeta: kapi.ObjectMeta{OwnerReferences: []kapi.OwnerReference{{}}}},
				nil, unversioned.GroupVersionKind{}, "", "", unversioned.GroupVersionResource{}, "", admission.Create, nil),
			expectFailure: true,
		},
		{
			name: "reject finalizer",
			attributes: admission.NewAttributesRecord(&kapi.Pod{ObjectMeta: kapi.ObjectMeta{Finalizers: []string{""}}},
				nil, unversioned.GroupVersionKind{}, "", "", unversioned.GroupVersionResource{}, "", admission.Create, nil),
			expectFailure: true,
		},
		{
			name: "allow",
			attributes: admission.NewAttributesRecord(&kapi.Pod{},
				nil, unversioned.GroupVersionKind{}, "", "", unversioned.GroupVersionResource{}, "", admission.Create, nil),
		},
	}

	for _, tc := range testCases {
		admission, err := NewOwnerReferenceBlocker()
		if err != nil {
			t.Errorf("%v: unexpected error: %v", tc.name, err)
			continue
		}

		admissionErr := admission.Admit(tc.attributes)
		if admissionErr == nil && tc.expectFailure {
			t.Errorf("%v: missing error", tc.name)
			continue
		}
		if admissionErr != nil && !tc.expectFailure {
			t.Errorf("%v: unexpected error: %v", tc.name, admissionErr)
			continue
		}
	}
}

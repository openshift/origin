package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"
)

func TestEnsureOwnerRef(t *testing.T) {
	tests := []struct {
		name           string
		obj            *metav1.ObjectMeta
		newOwnerRef    metav1.OwnerReference
		expectedOwners []metav1.OwnerReference
		expectedReturn bool
	}{
		{
			name: "empty",
			obj:  &metav1.ObjectMeta{},
			newOwnerRef: metav1.OwnerReference{
				APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid"),
			},
			expectedOwners: []metav1.OwnerReference{
				{APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid")},
			},
			expectedReturn: true,
		},
		{
			name: "add",
			obj: &metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{APIVersion: "v1", Kind: "Foo", Name: "the-other", UID: types.UID("other-uid")},
				},
			},
			newOwnerRef: metav1.OwnerReference{
				APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid"),
			},
			expectedOwners: []metav1.OwnerReference{
				{APIVersion: "v1", Kind: "Foo", Name: "the-other", UID: types.UID("other-uid")},
				{APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid")},
			},
			expectedReturn: true,
		},
		{
			name: "skip",
			obj: &metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid")},
					{APIVersion: "v1", Kind: "Foo", Name: "the-other", UID: types.UID("other-uid")},
				},
			},
			newOwnerRef: metav1.OwnerReference{
				APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid"),
			},
			expectedOwners: []metav1.OwnerReference{
				{APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid")},
				{APIVersion: "v1", Kind: "Foo", Name: "the-other", UID: types.UID("other-uid")},
			},
			expectedReturn: false,
		},
		{
			name: "replace on uid",
			obj: &metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{APIVersion: "v1", Kind: "Foo", Name: "the-other", UID: types.UID("other-uid")},
					{APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("bad-uid")},
				},
			},
			newOwnerRef: metav1.OwnerReference{
				APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid"),
			},
			expectedOwners: []metav1.OwnerReference{
				{APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid")},
				{APIVersion: "v1", Kind: "Foo", Name: "the-other", UID: types.UID("other-uid")},
			},
			expectedReturn: true,
		},
		{
			name: "preserve controller",
			obj: &metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{APIVersion: "v1", Kind: "Foo", Name: "the-other", UID: types.UID("other-uid")},
					{APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid")},
				},
			},
			newOwnerRef: metav1.OwnerReference{
				APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid"), Controller: boolPtr(true),
			},
			expectedOwners: []metav1.OwnerReference{
				{APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid"), Controller: boolPtr(true)},
				{APIVersion: "v1", Kind: "Foo", Name: "the-other", UID: types.UID("other-uid")},
			},
			expectedReturn: true,
		},
		{
			name: "preserve block",
			obj: &metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{APIVersion: "v1", Kind: "Foo", Name: "the-other", UID: types.UID("other-uid")},
					{APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid")},
				},
			},
			newOwnerRef: metav1.OwnerReference{
				APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid"), BlockOwnerDeletion: boolPtr(false),
			},
			expectedOwners: []metav1.OwnerReference{
				{APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid"), BlockOwnerDeletion: boolPtr(false)},
				{APIVersion: "v1", Kind: "Foo", Name: "the-other", UID: types.UID("other-uid")},
			},
			expectedReturn: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actualReturn := EnsureOwnerRef(tc.obj, tc.newOwnerRef)
			if tc.expectedReturn != actualReturn {
				t.Errorf("expected %v, got %v", tc.expectedReturn, actualReturn)
				return
			}
			if !kapihelper.Semantic.DeepEqual(tc.expectedOwners, tc.obj.OwnerReferences) {
				t.Errorf("expected %v, got %v", tc.expectedOwners, tc.obj.OwnerReferences)
				return
			}
		})
	}
}

func boolPtr(in bool) *bool {
	return &in
}

func TestHasOwnerRef(t *testing.T) {
	tests := []struct {
		name     string
		haystack *metav1.ObjectMeta
		needle   metav1.OwnerReference
		expected bool
	}{
		{
			name:     "empty",
			haystack: &metav1.ObjectMeta{},
			needle: metav1.OwnerReference{
				APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid"),
			},
			expected: false,
		},
		{
			name: "exact",
			haystack: &metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid"),
				}},
			},
			needle: metav1.OwnerReference{
				APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid"),
			},
			expected: true,
		},
		{
			name: "not uid",
			haystack: &metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "v1", Kind: "Foo", Name: "the-name",
				}},
			},
			needle: metav1.OwnerReference{
				APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid"),
			},
			expected: false,
		},
		{
			name: "ignored controller",
			haystack: &metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid"),
				}},
			},
			needle: metav1.OwnerReference{
				APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid"), Controller: boolPtr(true),
			},
			expected: true,
		},
		{
			name: "ignored block",
			haystack: &metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid"),
				}},
			},
			needle: metav1.OwnerReference{
				APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid"), BlockOwnerDeletion: boolPtr(false),
			},
			expected: true,
		},
		{
			name: "ignored both",
			haystack: &metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid"), Controller: boolPtr(false),
				}},
			},
			needle: metav1.OwnerReference{
				APIVersion: "v1", Kind: "Foo", Name: "the-name", UID: types.UID("uid"), BlockOwnerDeletion: boolPtr(false),
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := HasOwnerRef(tc.haystack, tc.needle)
			if tc.expected != actual {
				t.Errorf("expected %v, got %v", tc.expected, actual)
				return
			}
		})
	}
}

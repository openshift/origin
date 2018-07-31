package buildapihelpers

import (
	"reflect"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildv1 "github.com/openshift/api/build/v1"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

func TestFilterBuilds_withEmptyArray(t *testing.T) {
	actual := FilterBuilds([]buildv1.Build{}, nil)
	assertThatArrayIsEmpty(t, actual)
}

func TestFilterBuilds_withAllElementsAccepted(t *testing.T) {
	expected := []buildv1.Build{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "build1-abc",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "build2-abc",
			},
		},
	}

	alwaysTruePredicate := func(arg interface{}) bool {
		return true
	}

	actual := FilterBuilds(expected, alwaysTruePredicate)
	assertThatArraysAreEquals(t, actual, expected)
}

func TestFilterBuilds_withFilteredElements(t *testing.T) {
	input := []buildv1.Build{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "skip1-abc",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "build2-abc",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "skip3-abc",
			},
		},
	}

	expected := []buildv1.Build{input[1]}

	skipByNamePrefixPredicate := func(arg interface{}) bool {
		return !strings.HasPrefix(arg.(buildv1.Build).Name, "skip")
	}

	actual := FilterBuilds(input, skipByNamePrefixPredicate)
	assertThatArraysAreEquals(t, actual, expected)
}

func TestByBuildConfigPredicate_withBuildConfigAnnotation(t *testing.T) {
	input := []buildv1.Build{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "build1-abc",
				Annotations: map[string]string{buildapi.BuildConfigAnnotation: "foo"},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "build2-abc",
				Labels: map[string]string{"bar": "baz"},
			},
		},
	}

	expected := []buildv1.Build{input[0]}

	actual := FilterBuilds(input, ByBuildConfigPredicate("foo"))
	assertThatArraysAreEquals(t, actual, expected)

	actual = FilterBuilds(input, ByBuildConfigPredicate("not-foo"))
	assertThatArrayIsEmpty(t, actual)
}

func TestByBuildConfigPredicate_withBuildConfigLabel(t *testing.T) {
	input := []buildv1.Build{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "build1-abc",
				Labels: map[string]string{buildapi.BuildConfigLabel: "foo"},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "build2-abc",
				Labels: map[string]string{"bar": "baz"},
			},
		},
	}

	expected := []buildv1.Build{input[0]}

	actual := FilterBuilds(input, ByBuildConfigPredicate("foo"))
	assertThatArraysAreEquals(t, actual, expected)

	actual = FilterBuilds(input, ByBuildConfigPredicate("not-foo"))
	assertThatArrayIsEmpty(t, actual)
}

func TestByBuildConfigPredicate_withBothBuildConfigLabels(t *testing.T) {
	input := []buildv1.Build{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "build1-abc",
				Labels: map[string]string{buildapi.BuildConfigLabel: "foo"},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "build2-abc",
				Labels: map[string]string{"bar": "baz"},
			},
		},
	}

	expected := []buildv1.Build{input[0]}

	actual := FilterBuilds(input, ByBuildConfigPredicate("foo"))
	assertThatArraysAreEquals(t, actual, expected)

	actual = FilterBuilds(input, ByBuildConfigPredicate("not-foo"))
	assertThatArrayIsEmpty(t, actual)
}

func TestByBuildConfigPredicate_withoutBuildConfigLabels(t *testing.T) {
	input := []buildv1.Build{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "build1-abc",
				Labels: map[string]string{"bar": "baz"},
			},
		},
	}

	actual := FilterBuilds(input, ByBuildConfigPredicate("not-foo"))
	assertThatArrayIsEmpty(t, actual)
}

func assertThatArraysAreEquals(t *testing.T, expected, actual []buildv1.Build) {
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected: %v\ngot: %v", expected, actual)
	}
}

func assertThatArrayIsEmpty(t *testing.T, array []buildv1.Build) {
	if len(array) != 0 {
		t.Errorf("expected empty array, got %v", array)
	}
}

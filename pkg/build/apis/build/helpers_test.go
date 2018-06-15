package build

import (
	"reflect"
	"testing"

	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFilterBuilds_withEmptyArray(t *testing.T) {
	actual := FilterBuilds([]Build{}, nil)
	assertThatArrayIsEmpty(t, actual)
}

func TestFilterBuilds_withAllElementsAccepted(t *testing.T) {
	expected := []Build{
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
	input := []Build{
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

	expected := []Build{input[1]}

	skipByNamePrefixPredicate := func(arg interface{}) bool {
		return !strings.HasPrefix(arg.(Build).Name, "skip")
	}

	actual := FilterBuilds(input, skipByNamePrefixPredicate)
	assertThatArraysAreEquals(t, actual, expected)
}

func TestByBuildConfigPredicate_withBuildConfigAnnotation(t *testing.T) {
	input := []Build{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "build1-abc",
				Annotations: map[string]string{BuildConfigAnnotation: "foo"},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "build2-abc",
				Labels: map[string]string{"bar": "baz"},
			},
		},
	}

	expected := []Build{input[0]}

	actual := FilterBuilds(input, ByBuildConfigPredicate("foo"))
	assertThatArraysAreEquals(t, actual, expected)

	actual = FilterBuilds(input, ByBuildConfigPredicate("not-foo"))
	assertThatArrayIsEmpty(t, actual)
}

func TestByBuildConfigPredicate_withBuildConfigLabel(t *testing.T) {
	input := []Build{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "build1-abc",
				Labels: map[string]string{BuildConfigLabel: "foo"},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "build2-abc",
				Labels: map[string]string{"bar": "baz"},
			},
		},
	}

	expected := []Build{input[0]}

	actual := FilterBuilds(input, ByBuildConfigPredicate("foo"))
	assertThatArraysAreEquals(t, actual, expected)

	actual = FilterBuilds(input, ByBuildConfigPredicate("not-foo"))
	assertThatArrayIsEmpty(t, actual)
}

func TestByBuildConfigPredicate_withBuildConfigLabelDeprecated(t *testing.T) {
	input := []Build{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "build1-abc",
				Labels: map[string]string{BuildConfigLabelDeprecated: "foo"},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "build2-abc",
				Labels: map[string]string{"bar": "baz"},
			},
		},
	}

	expected := []Build{input[0]}

	actual := FilterBuilds(input, ByBuildConfigPredicate("foo"))
	assertThatArraysAreEquals(t, actual, expected)

	actual = FilterBuilds(input, ByBuildConfigPredicate("not-foo"))
	assertThatArrayIsEmpty(t, actual)
}

func TestByBuildConfigPredicate_withBothBuildConfigLabels(t *testing.T) {
	input := []Build{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "build1-abc",
				Labels: map[string]string{BuildConfigLabel: "foo"},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "build2-abc",
				Labels: map[string]string{"bar": "baz"},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "build3-abc",
				Labels: map[string]string{BuildConfigLabelDeprecated: "foo"},
			},
		},
	}

	expected := []Build{input[0], input[2]}

	actual := FilterBuilds(input, ByBuildConfigPredicate("foo"))
	assertThatArraysAreEquals(t, actual, expected)

	actual = FilterBuilds(input, ByBuildConfigPredicate("not-foo"))
	assertThatArrayIsEmpty(t, actual)
}

func TestByBuildConfigPredicate_withoutBuildConfigLabels(t *testing.T) {
	input := []Build{
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

func assertThatArraysAreEquals(t *testing.T, expected, actual []Build) {
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected: %v\ngot: %v", expected, actual)
	}
}

func assertThatArrayIsEmpty(t *testing.T, array []Build) {
	if len(array) != 0 {
		t.Errorf("expected empty array, got %v", array)
	}
}

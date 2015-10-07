package namer

import (
	"math/rand"
	"testing"

	"k8s.io/kubernetes/pkg/util"
)

func TestGetName(t *testing.T) {

	for i := 0; i < 10; i++ {
		shortName := randSeq(rand.Intn(util.DNS1123SubdomainMaxLength-1) + 1)
		longName := randSeq(util.DNS1123SubdomainMaxLength + rand.Intn(100))

		tests := []struct {
			base, suffix, expected string
		}{
			{
				base:     shortName,
				suffix:   "deploy",
				expected: shortName + "-deploy",
			},
			{
				base:     longName,
				suffix:   "deploy",
				expected: longName[:util.DNS1123SubdomainMaxLength-16] + "-" + hash(longName) + "-deploy",
			},
			{
				base:     shortName,
				suffix:   longName,
				expected: shortName + "-" + hash(shortName+"-"+longName),
			},
			{
				base:     "",
				suffix:   shortName,
				expected: "-" + shortName,
			},
			{
				base:     "",
				suffix:   longName,
				expected: "-" + hash("-"+longName),
			},
			{
				base:     shortName,
				suffix:   "",
				expected: shortName + "-",
			},
			{
				base:     longName,
				suffix:   "",
				expected: longName[:util.DNS1123SubdomainMaxLength-10] + "-" + hash(longName) + "-",
			},
		}

		for _, test := range tests {
			result := GetName(test.base, test.suffix, util.DNS1123SubdomainMaxLength)
			if result != test.expected {
				t.Errorf("Got unexpected result. Expected: %s Got: %s", test.expected, result)
			}
		}
	}
}

func TestGetNameIsDifferent(t *testing.T) {
	shortName := randSeq(32)
	deployerName := GetName(shortName, "deploy", util.DNS1123SubdomainMaxLength)
	builderName := GetName(shortName, "build", util.DNS1123SubdomainMaxLength)
	if deployerName == builderName {
		t.Errorf("Expecting names to be different: %s\n", deployerName)
	}
	longName := randSeq(util.DNS1123SubdomainMaxLength + 10)
	deployerName = GetName(longName, "deploy", util.DNS1123SubdomainMaxLength)
	builderName = GetName(longName, "build", util.DNS1123SubdomainMaxLength)
	if deployerName == builderName {
		t.Errorf("Expecting names to be different: %s\n", deployerName)
	}
}

func TestLimitLength(t *testing.T) {
	tests := []struct {
		s      string
		maxLen int
		want   string
	}{
		{"helloworld", 0, ""},
		{"helloworld", 3, "3b9"},         // only part of the hash
		{"helloworld", 8, "3b9f5c61"},    // the whole hash
		{"helloworld", 9, "h3b9f5c61"},   // first char + hash
		{"helloworld", 10, "helloworld"}, // s fits in maxLen
	}
	for _, test := range tests {
		got := LimitLength(test.s, test.maxLen)
		if got != test.want {
			t.Errorf("LimitLength(%q, %d) = %q; want %q", test.s, test.maxLen, got, test.want)
		}
	}
}

func TestLimitLengthReturnShortNames(t *testing.T) {
	s := randSeq(32)
	for i := 0; i < len(s)+2; i++ {
		got := LimitLength(s, i)
		if len(got) > i {
			t.Errorf("len(LimitLength(%[1]q, %[2]d)) = len(%[3]q) = %[4]d; want %[2]d", s, i, got, len(got))
		}
	}
}

// From k8s.io/kubernetes/pkg/api/generator.go
var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789-")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

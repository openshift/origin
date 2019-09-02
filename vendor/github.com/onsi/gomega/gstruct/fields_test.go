package gstruct_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Struct", func() {
	allFields := struct{ A, B string }{"a", "b"}
	missingFields := struct{ A string }{"a"}
	extraFields := struct{ A, B, C string }{"a", "b", "c"}
	emptyFields := struct{ A, B string }{}

	It("should strictly match all fields", func() {
		m := MatchAllFields(Fields{
			"B": Equal("b"),
			"A": Equal("a"),
		})
		Expect(allFields).Should(m, "should match all fields")
		Expect(missingFields).ShouldNot(m, "should fail with missing fields")
		Expect(extraFields).ShouldNot(m, "should fail with extra fields")
		Expect(emptyFields).ShouldNot(m, "should fail with empty fields")

		m = MatchAllFields(Fields{
			"A": Equal("a"),
			"B": Equal("fail"),
		})
		Expect(allFields).ShouldNot(m, "should run nested matchers")
	})

	It("should handle empty structs", func() {
		m := MatchAllFields(Fields{})
		Expect(struct{}{}).Should(m, "should handle empty structs")
		Expect(allFields).ShouldNot(m, "should fail with extra fields")
	})

	It("should ignore missing fields", func() {
		m := MatchFields(IgnoreMissing, Fields{
			"B": Equal("b"),
			"A": Equal("a"),
		})
		Expect(allFields).Should(m, "should match all fields")
		Expect(missingFields).Should(m, "should ignore missing fields")
		Expect(extraFields).ShouldNot(m, "should fail with extra fields")
		Expect(emptyFields).ShouldNot(m, "should fail with empty fields")
	})

	It("should ignore extra fields", func() {
		m := MatchFields(IgnoreExtras, Fields{
			"B": Equal("b"),
			"A": Equal("a"),
		})
		Expect(allFields).Should(m, "should match all fields")
		Expect(missingFields).ShouldNot(m, "should fail with missing fields")
		Expect(extraFields).Should(m, "should ignore extra fields")
		Expect(emptyFields).ShouldNot(m, "should fail with empty fields")
	})

	It("should ignore missing and extra fields", func() {
		m := MatchFields(IgnoreMissing|IgnoreExtras, Fields{
			"B": Equal("b"),
			"A": Equal("a"),
		})
		Expect(allFields).Should(m, "should match all fields")
		Expect(missingFields).Should(m, "should ignore missing fields")
		Expect(extraFields).Should(m, "should ignore extra fields")
		Expect(emptyFields).ShouldNot(m, "should fail with empty fields")

		m = MatchFields(IgnoreMissing|IgnoreExtras, Fields{
			"A": Equal("a"),
			"B": Equal("fail"),
		})
		Expect(allFields).ShouldNot(m, "should run nested matchers")
	})

	It("should produce sensible error messages", func() {
		m := MatchAllFields(Fields{
			"B": Equal("b"),
			"A": Equal("a"),
		})

		actual := struct{ A, C string }{A: "b", C: "c"}

		//Because the order of the constituent errors can't be guaranteed,
		//we do a number of checks to make sure everything's included
		m.Match(actual)
		Expect(m.FailureMessage(actual)).Should(HavePrefix(
			"Expected\n    <string>: \nto match fields: {\n",
		))
		Expect(m.FailureMessage(actual)).Should(ContainSubstring(
			".A:\n	Expected\n	    <string>: b\n	to equal\n	    <string>: a\n",
		))
		Expect(m.FailureMessage(actual)).Should(ContainSubstring(
			"missing expected field B\n",
		))
		Expect(m.FailureMessage(actual)).Should(ContainSubstring(
			".C:\n	unexpected field C: {A:b C:c}",
		))
	})
})

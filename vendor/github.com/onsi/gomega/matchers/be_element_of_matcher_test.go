package matchers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
)

var _ = Describe("BeElementOf", func() {
	Context("when passed a supported type", func() {
		It("should do the right thing", func() {
			Expect(2).Should(BeElementOf([2]int{1, 2}))
			Expect(3).ShouldNot(BeElementOf([2]int{1, 2}))

			Expect(2).Should(BeElementOf([]int{1, 2}))
			Expect(3).ShouldNot(BeElementOf([]int{1, 2}))

			Expect(2).Should(BeElementOf(1, 2))
			Expect(3).ShouldNot(BeElementOf(1, 2))

			Expect("abc").Should(BeElementOf("abc"))
			Expect("abc").ShouldNot(BeElementOf("def"))

			Expect("abc").ShouldNot(BeElementOf())
			Expect(7).ShouldNot(BeElementOf(nil))

			arr := make([]myCustomType, 2)
			arr[0] = myCustomType{s: "foo", n: 3, f: 2.0, arr: []string{"a", "b"}}
			arr[1] = myCustomType{s: "foo", n: 3, f: 2.0, arr: []string{"a", "c"}}
			Expect(myCustomType{s: "foo", n: 3, f: 2.0, arr: []string{"a", "b"}}).Should(BeElementOf(arr))
			Expect(myCustomType{s: "foo", n: 3, f: 2.0, arr: []string{"b", "c"}}).ShouldNot(BeElementOf(arr))
		})
	})

	Context("when passed a correctly typed nil", func() {
		It("should operate succesfully on the passed in value", func() {
			var nilSlice []int
			Expect(1).ShouldNot(BeElementOf(nilSlice))

			var nilMap map[int]string
			Expect("foo").ShouldNot(BeElementOf(nilMap))
		})
	})

	Context("when passed an unsupported type", func() {
		It("should error", func() {
			success, err := (&BeElementOfMatcher{Elements: []interface{}{0}}).Match(nil)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			success, err = (&BeElementOfMatcher{Elements: nil}).Match(nil)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})

	It("builds failure message", func() {
		actual := BeElementOf(1, 2).FailureMessage(123)
		Expect(actual).To(Equal("Expected\n    <int>: 123\nto be an element of\n    <[]interface {} | len:2, cap:2>: [1, 2]"))
	})

	It("builds negated failure message", func() {
		actual := BeElementOf(1, 2).NegatedFailureMessage(123)
		Expect(actual).To(Equal("Expected\n    <int>: 123\nnot to be an element of\n    <[]interface {} | len:2, cap:2>: [1, 2]"))
	})
})

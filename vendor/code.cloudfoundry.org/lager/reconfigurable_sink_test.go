package lager_test

import (
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ReconfigurableSink", func() {
	var (
		testSink *lagertest.TestSink

		sink *lager.ReconfigurableSink
	)

	BeforeEach(func() {
		testSink = lagertest.NewTestSink()

		sink = lager.NewReconfigurableSink(testSink, lager.INFO)
	})

	It("returns the current level", func() {
		Expect(sink.GetMinLevel()).To(Equal(lager.INFO))
	})

	Context("when logging above the minimum log level", func() {
		var log lager.LogFormat

		BeforeEach(func() {
			log = lager.LogFormat{LogLevel: lager.INFO, Message: "hello world"}
			sink.Log(log)
		})

		It("writes to the given sink", func() {
			Expect(testSink.Buffer().Contents()).To(MatchJSON(log.ToJSON()))
		})
	})

	Context("when logging below the minimum log level", func() {
		BeforeEach(func() {
			sink.Log(lager.LogFormat{LogLevel: lager.DEBUG, Message: "hello world"})
		})

		It("does not write to the given writer", func() {
			Expect(testSink.Buffer().Contents()).To(BeEmpty())
		})
	})

	Context("when reconfigured to a new log level", func() {
		BeforeEach(func() {
			sink.SetMinLevel(lager.DEBUG)
		})

		It("writes logs above the new log level", func() {
			log := lager.LogFormat{LogLevel: lager.DEBUG, Message: "hello world"}
			sink.Log(log)
			Expect(testSink.Buffer().Contents()).To(MatchJSON(log.ToJSON()))
		})

		It("returns the newly updated level", func() {
			Expect(sink.GetMinLevel()).To(Equal(lager.DEBUG))
		})
	})
})

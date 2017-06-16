package ginkgoreporter_test

import (
	"bytes"
	"encoding/json"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/chug"
	. "code.cloudfoundry.org/lager/ginkgoreporter"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
	"github.com/onsi/ginkgo/types"
	. "github.com/onsi/gomega"
)

var _ = Describe("Ginkgoreporter", func() {
	var (
		reporter reporters.Reporter
		buffer   *bytes.Buffer
	)

	BeforeEach(func() {
		buffer = &bytes.Buffer{}
		reporter = New(buffer)
	})

	fetchLogs := func() []chug.LogEntry {
		out := make(chan chug.Entry, 1000)
		chug.Chug(buffer, out)
		logs := []chug.LogEntry{}
		for entry := range out {
			if entry.IsLager {
				logs = append(logs, entry.Log)
			}
		}
		return logs
	}

	jsonRoundTrip := func(object interface{}) interface{} {
		jsonEncoded, err := json.Marshal(object)
		Expect(err).NotTo(HaveOccurred())
		var out interface{}
		err = json.Unmarshal(jsonEncoded, &out)
		Expect(err).NotTo(HaveOccurred())
		return out
	}

	Describe("Announcing specs", func() {
		var summary *types.SpecSummary
		BeforeEach(func() {
			summary = &types.SpecSummary{
				ComponentTexts: []string{"A", "B"},
				ComponentCodeLocations: []types.CodeLocation{
					{
						FileName:       "file/a",
						LineNumber:     3,
						FullStackTrace: "some-stack-trace",
					},
					{
						FileName:       "file/b",
						LineNumber:     4,
						FullStackTrace: "some-stack-trace",
					},
				},
				RunTime: time.Minute,
				State:   types.SpecStatePassed,
			}
		})

		Context("when running in parallel", func() {
			It("should include the node # in the session and message", func() {
				configType := config.GinkgoConfigType{
					ParallelTotal: 3,
					ParallelNode:  2,
				}
				suiteSummary := &types.SuiteSummary{}
				reporter.SpecSuiteWillBegin(configType, suiteSummary)

				reporter.SpecWillRun(summary)
				reporter.SpecDidComplete(summary)
				reporter.SpecWillRun(summary)
				reporter.SpecDidComplete(summary)

				logs := fetchLogs()
				Expect(logs[0].Session).To(Equal("2.1"))
				Expect(logs[0].Message).To(Equal("ginkgo.node-2.spec.start"))
				Expect(logs[1].Session).To(Equal("2.1"))
				Expect(logs[1].Message).To(Equal("ginkgo.node-2.spec.end"))
				Expect(logs[2].Session).To(Equal("2.2"))
				Expect(logs[0].Message).To(Equal("ginkgo.node-2.spec.start"))
				Expect(logs[3].Session).To(Equal("2.2"))
				Expect(logs[1].Message).To(Equal("ginkgo.node-2.spec.end"))
			})
		})

		Describe("incrementing sessions", func() {
			It("should increment the session counter as specs run", func() {
				reporter.SpecWillRun(summary)
				reporter.SpecDidComplete(summary)
				reporter.SpecWillRun(summary)
				reporter.SpecDidComplete(summary)

				logs := fetchLogs()
				Expect(logs[0].Session).To(Equal("1"))
				Expect(logs[1].Session).To(Equal("1"))
				Expect(logs[2].Session).To(Equal("2"))
				Expect(logs[3].Session).To(Equal("2"))
			})
		})

		Context("when a spec starts", func() {
			BeforeEach(func() {
				reporter.SpecWillRun(summary)
			})

			It("should log about the spec starting", func() {
				log := fetchLogs()[0]
				Expect(log.LogLevel).To(Equal(lager.INFO))
				Expect(log.Source).To(Equal("ginkgo"))
				Expect(log.Message).To(Equal("ginkgo.spec.start"))
				Expect(log.Session).To(Equal("1"))
				Expect(log.Data["summary"]).To(Equal(jsonRoundTrip(SpecSummary{
					Name:     []string{"A", "B"},
					Location: "file/b:4",
				})))

			})

			Context("when the spec succeeds", func() {
				It("should info", func() {
					reporter.SpecDidComplete(summary)
					log := fetchLogs()[1]
					Expect(log.LogLevel).To(Equal(lager.INFO))
					Expect(log.Source).To(Equal("ginkgo"))
					Expect(log.Message).To(Equal("ginkgo.spec.end"))
					Expect(log.Session).To(Equal("1"))
					Expect(log.Data["summary"]).To(Equal(jsonRoundTrip(SpecSummary{
						Name:     []string{"A", "B"},
						Location: "file/b:4",
						State:    "PASSED",
						Passed:   true,
						RunTime:  time.Minute,
					})))

				})
			})

			Context("when the spec fails", func() {
				BeforeEach(func() {
					summary.State = types.SpecStateFailed
					summary.Failure = types.SpecFailure{
						Message: "something failed!",
						Location: types.CodeLocation{
							FileName:       "some/file",
							LineNumber:     3,
							FullStackTrace: "some-stack-trace",
						},
					}
				})

				It("should error", func() {
					reporter.SpecDidComplete(summary)
					log := fetchLogs()[1]
					Expect(log.LogLevel).To(Equal(lager.ERROR))
					Expect(log.Source).To(Equal("ginkgo"))
					Expect(log.Message).To(Equal("ginkgo.spec.end"))
					Expect(log.Session).To(Equal("1"))
					Expect(log.Error.Error()).To(Equal("something failed!\nsome/file:3"))
					Expect(log.Data["summary"]).To(Equal(jsonRoundTrip(SpecSummary{
						Name:       []string{"A", "B"},
						Location:   "file/b:4",
						State:      "FAILED",
						Passed:     false,
						RunTime:    time.Minute,
						StackTrace: "some-stack-trace",
					})))

				})
			})
		})
	})
})

package lager_test

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Logger", func() {
	var logger lager.Logger
	var testSink *lagertest.TestSink

	var component = "my-component"
	var action = "my-action"
	var logData = lager.Data{
		"foo":      "bar",
		"a-number": 7,
	}
	var anotherLogData = lager.Data{
		"baz":      "quux",
		"b-number": 43,
	}

	BeforeEach(func() {
		logger = lager.NewLogger(component)
		testSink = lagertest.NewTestSink()
		logger.RegisterSink(testSink)
	})

	var TestCommonLogFeatures = func(level lager.LogLevel) {
		var log lager.LogFormat

		BeforeEach(func() {
			log = testSink.Logs()[0]
		})

		It("writes a log to the sink", func() {
			Expect(testSink.Logs()).To(HaveLen(1))
		})

		It("records the source component", func() {
			Expect(log.Source).To(Equal(component))
		})

		It("outputs a properly-formatted message", func() {
			Expect(log.Message).To(Equal(fmt.Sprintf("%s.%s", component, action)))
		})

		It("has a timestamp", func() {
			expectedTime := float64(time.Now().UnixNano()) / 1e9
			parsedTimestamp, err := strconv.ParseFloat(log.Timestamp, 64)
			Expect(err).NotTo(HaveOccurred())
			Expect(parsedTimestamp).To(BeNumerically("~", expectedTime, 1.0))
		})

		It("sets the proper output level", func() {
			Expect(log.LogLevel).To(Equal(level))
		})
	}

	var TestLogData = func() {
		var log lager.LogFormat

		BeforeEach(func() {
			log = testSink.Logs()[0]
		})

		It("data contains custom user data", func() {
			Expect(log.Data["foo"]).To(Equal("bar"))
			Expect(log.Data["a-number"]).To(BeNumerically("==", 7))
			Expect(log.Data["baz"]).To(Equal("quux"))
			Expect(log.Data["b-number"]).To(BeNumerically("==", 43))
		})
	}

	Describe("Session", func() {
		var session lager.Logger

		BeforeEach(func() {
			session = logger.Session("sub-action")
		})

		Describe("the returned logger", func() {
			JustBeforeEach(func() {
				session.Debug("some-debug-action", lager.Data{"level": "debug"})
				session.Info("some-info-action", lager.Data{"level": "info"})
				session.Error("some-error-action", errors.New("oh no!"), lager.Data{"level": "error"})

				defer func() {
					recover()
				}()

				session.Fatal("some-fatal-action", errors.New("oh no!"), lager.Data{"level": "fatal"})
			})

			It("logs with a shared session id in the data", func() {
				Expect(testSink.Logs()[0].Data["session"]).To(Equal("1"))
				Expect(testSink.Logs()[1].Data["session"]).To(Equal("1"))
				Expect(testSink.Logs()[2].Data["session"]).To(Equal("1"))
				Expect(testSink.Logs()[3].Data["session"]).To(Equal("1"))
			})

			It("logs with the task added to the message", func() {
				Expect(testSink.Logs()[0].Message).To(Equal("my-component.sub-action.some-debug-action"))
				Expect(testSink.Logs()[1].Message).To(Equal("my-component.sub-action.some-info-action"))
				Expect(testSink.Logs()[2].Message).To(Equal("my-component.sub-action.some-error-action"))
				Expect(testSink.Logs()[3].Message).To(Equal("my-component.sub-action.some-fatal-action"))
			})

			It("logs with the original data", func() {
				Expect(testSink.Logs()[0].Data["level"]).To(Equal("debug"))
				Expect(testSink.Logs()[1].Data["level"]).To(Equal("info"))
				Expect(testSink.Logs()[2].Data["level"]).To(Equal("error"))
				Expect(testSink.Logs()[3].Data["level"]).To(Equal("fatal"))
			})

			Context("with data", func() {
				BeforeEach(func() {
					session = logger.Session("sub-action", lager.Data{"foo": "bar"})
				})

				It("logs with the data added to the message", func() {
					Expect(testSink.Logs()[0].Data["foo"]).To(Equal("bar"))
					Expect(testSink.Logs()[1].Data["foo"]).To(Equal("bar"))
					Expect(testSink.Logs()[2].Data["foo"]).To(Equal("bar"))
					Expect(testSink.Logs()[3].Data["foo"]).To(Equal("bar"))
				})

				It("keeps the original data", func() {
					Expect(testSink.Logs()[0].Data["level"]).To(Equal("debug"))
					Expect(testSink.Logs()[1].Data["level"]).To(Equal("info"))
					Expect(testSink.Logs()[2].Data["level"]).To(Equal("error"))
					Expect(testSink.Logs()[3].Data["level"]).To(Equal("fatal"))
				})
			})

			Context("with another session", func() {
				BeforeEach(func() {
					session = logger.Session("next-sub-action")
				})

				It("logs with a shared session id in the data", func() {
					Expect(testSink.Logs()[0].Data["session"]).To(Equal("2"))
					Expect(testSink.Logs()[1].Data["session"]).To(Equal("2"))
					Expect(testSink.Logs()[2].Data["session"]).To(Equal("2"))
					Expect(testSink.Logs()[3].Data["session"]).To(Equal("2"))
				})

				It("logs with the task added to the message", func() {
					Expect(testSink.Logs()[0].Message).To(Equal("my-component.next-sub-action.some-debug-action"))
					Expect(testSink.Logs()[1].Message).To(Equal("my-component.next-sub-action.some-info-action"))
					Expect(testSink.Logs()[2].Message).To(Equal("my-component.next-sub-action.some-error-action"))
					Expect(testSink.Logs()[3].Message).To(Equal("my-component.next-sub-action.some-fatal-action"))
				})
			})

			Describe("WithData", func() {
				BeforeEach(func() {
					session = logger.WithData(lager.Data{"foo": "bar"})
				})

				It("returns a new logger with the given data", func() {
					Expect(testSink.Logs()[0].Data["foo"]).To(Equal("bar"))
					Expect(testSink.Logs()[1].Data["foo"]).To(Equal("bar"))
					Expect(testSink.Logs()[2].Data["foo"]).To(Equal("bar"))
					Expect(testSink.Logs()[3].Data["foo"]).To(Equal("bar"))
				})

				It("does not append to the logger's task", func() {
					Expect(testSink.Logs()[0].Message).To(Equal("my-component.some-debug-action"))
				})
			})

			Context("with a nested session", func() {
				BeforeEach(func() {
					session = session.Session("sub-sub-action")
				})

				It("logs with a shared session id in the data", func() {
					Expect(testSink.Logs()[0].Data["session"]).To(Equal("1.1"))
					Expect(testSink.Logs()[1].Data["session"]).To(Equal("1.1"))
					Expect(testSink.Logs()[2].Data["session"]).To(Equal("1.1"))
					Expect(testSink.Logs()[3].Data["session"]).To(Equal("1.1"))
				})

				It("logs with the task added to the message", func() {
					Expect(testSink.Logs()[0].Message).To(Equal("my-component.sub-action.sub-sub-action.some-debug-action"))
					Expect(testSink.Logs()[1].Message).To(Equal("my-component.sub-action.sub-sub-action.some-info-action"))
					Expect(testSink.Logs()[2].Message).To(Equal("my-component.sub-action.sub-sub-action.some-error-action"))
					Expect(testSink.Logs()[3].Message).To(Equal("my-component.sub-action.sub-sub-action.some-fatal-action"))
				})
			})
		})
	})

	Describe("Debug", func() {
		Context("with log data", func() {
			BeforeEach(func() {
				logger.Debug(action, logData, anotherLogData)
			})

			TestCommonLogFeatures(lager.DEBUG)
			TestLogData()
		})

		Context("with no log data", func() {
			BeforeEach(func() {
				logger.Debug(action)
			})

			TestCommonLogFeatures(lager.DEBUG)
		})
	})

	Describe("Info", func() {
		Context("with log data", func() {
			BeforeEach(func() {
				logger.Info(action, logData, anotherLogData)
			})

			TestCommonLogFeatures(lager.INFO)
			TestLogData()
		})

		Context("with no log data", func() {
			BeforeEach(func() {
				logger.Info(action)
			})

			TestCommonLogFeatures(lager.INFO)
		})
	})

	Describe("Error", func() {
		var err = errors.New("oh noes!")
		Context("with log data", func() {
			BeforeEach(func() {
				logger.Error(action, err, logData, anotherLogData)
			})

			TestCommonLogFeatures(lager.ERROR)
			TestLogData()

			It("data contains error message", func() {
				Expect(testSink.Logs()[0].Data["error"]).To(Equal(err.Error()))
			})
		})

		Context("with no log data", func() {
			BeforeEach(func() {
				logger.Error(action, err)
			})

			TestCommonLogFeatures(lager.ERROR)

			It("data contains error message", func() {
				Expect(testSink.Logs()[0].Data["error"]).To(Equal(err.Error()))
			})
		})

		Context("with no error", func() {
			BeforeEach(func() {
				logger.Error(action, nil)
			})

			TestCommonLogFeatures(lager.ERROR)

			It("does not contain the error message", func() {
				Expect(testSink.Logs()[0].Data).NotTo(HaveKey("error"))
			})
		})
	})

	Describe("Fatal", func() {
		var err = errors.New("oh noes!")
		var fatalErr interface{}

		Context("with log data", func() {
			BeforeEach(func() {
				defer func() {
					fatalErr = recover()
				}()

				logger.Fatal(action, err, logData, anotherLogData)
			})

			TestCommonLogFeatures(lager.FATAL)
			TestLogData()

			It("data contains error message", func() {
				Expect(testSink.Logs()[0].Data["error"]).To(Equal(err.Error()))
			})

			It("data contains stack trace", func() {
				Expect(testSink.Logs()[0].Data["trace"]).NotTo(BeEmpty())
			})

			It("panics with the provided error", func() {
				Expect(fatalErr).To(Equal(err))
			})
		})

		Context("with no log data", func() {
			BeforeEach(func() {
				defer func() {
					fatalErr = recover()
				}()

				logger.Fatal(action, err)
			})

			TestCommonLogFeatures(lager.FATAL)

			It("data contains error message", func() {
				Expect(testSink.Logs()[0].Data["error"]).To(Equal(err.Error()))
			})

			It("data contains stack trace", func() {
				Expect(testSink.Logs()[0].Data["trace"]).NotTo(BeEmpty())
			})

			It("panics with the provided error", func() {
				Expect(fatalErr).To(Equal(err))
			})
		})

		Context("with no error", func() {
			BeforeEach(func() {
				defer func() {
					fatalErr = recover()
				}()

				logger.Fatal(action, nil)
			})

			TestCommonLogFeatures(lager.FATAL)

			It("does not contain the error message", func() {
				Expect(testSink.Logs()[0].Data).NotTo(HaveKey("error"))
			})

			It("data contains stack trace", func() {
				Expect(testSink.Logs()[0].Data["trace"]).NotTo(BeEmpty())
			})

			It("panics with the provided error (i.e. nil)", func() {
				Expect(fatalErr).To(BeNil())
			})
		})
	})
})

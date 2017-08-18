package lager_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"code.cloudfoundry.org/lager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RedactingWriterSink", func() {
	const MaxThreads = 100

	var sink lager.Sink
	var writer *copyWriter

	BeforeEach(func() {
		writer = NewCopyWriter()
		var err error
		sink, err = lager.NewRedactingWriterSink(writer, lager.INFO, nil, nil)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when logging above the minimum log level", func() {
		BeforeEach(func() {
			sink.Log(lager.LogFormat{LogLevel: lager.INFO, Message: "hello world", Data: lager.Data{"password": "abcd"}})
		})

		It("writes to the given writer", func() {
			Expect(writer.Copy()).To(MatchJSON(`{"message":"hello world","log_level":1,"timestamp":"","source":"","data":{"password":"*REDACTED*"}}`))
		})
	})

	Context("when a unserializable object is passed into data", func() {
		BeforeEach(func() {
			sink.Log(lager.LogFormat{LogLevel: lager.INFO, Message: "hello world", Data: map[string]interface{}{"some_key": func() {}}})
		})

		It("logs the serialization error", func() {
			message := map[string]interface{}{}
			json.Unmarshal(writer.Copy(), &message)
			Expect(message["message"]).To(Equal("hello world"))
			Expect(message["log_level"]).To(Equal(float64(1)))
			Expect(message["data"].(map[string]interface{})["lager serialisation error"]).To(Equal("json: unsupported type: func()"))
			Expect(message["data"].(map[string]interface{})["data_dump"]).ToNot(BeEmpty())
		})

		Measure("should be efficient", func(b Benchmarker) {
			runtime := b.Time("runtime", func() {
				for i := 0; i < 5000; i++ {
					sink.Log(lager.LogFormat{LogLevel: lager.INFO, Message: "hello world", Data: map[string]interface{}{"some_key": func() {}}})
					Expect(writer.Copy()).ToNot(BeEmpty())
				}
			})

			Expect(runtime.Seconds()).To(BeNumerically("<", 1), "logging shouldn't take too long.")
		}, 1)
	})

	Context("when logging below the minimum log level", func() {
		BeforeEach(func() {
			sink.Log(lager.LogFormat{LogLevel: lager.DEBUG, Message: "hello world"})
		})

		It("does not write to the given writer", func() {
			Expect(writer.Copy()).To(Equal([]byte{}))
		})
	})

	Context("when logging from multiple threads", func() {
		var content = "abcdefg "

		BeforeEach(func() {
			wg := new(sync.WaitGroup)
			for i := 0; i < MaxThreads; i++ {
				wg.Add(1)
				go func() {
					sink.Log(lager.LogFormat{LogLevel: lager.INFO, Message: content})
					wg.Done()
				}()
			}
			wg.Wait()
		})

		It("writes to the given writer", func() {
			lines := strings.Split(string(writer.Copy()), "\n")
			for _, line := range lines {
				if line == "" {
					continue
				}
				Expect(line).To(MatchJSON(fmt.Sprintf(`{"message":"%s","log_level":1,"timestamp":"","source":"","data":null}`, content)))
			}
		})
	})
})

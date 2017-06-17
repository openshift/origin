package auth_test

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/brokerapi/auth"
)

var _ = Describe("Auth Wrapper", func() {
	var (
		username     string
		password     string
		httpRecorder *httptest.ResponseRecorder
	)

	newRequest := func(username, password string) *http.Request {
		request, err := http.NewRequest("GET", "", nil)
		Expect(err).NotTo(HaveOccurred())
		request.SetBasicAuth(username, password)
		return request
	}

	BeforeEach(func() {
		username = "username"
		password = "password"
		httpRecorder = httptest.NewRecorder()
	})

	Describe("wrapped handler", func() {
		var wrappedHandler http.Handler

		BeforeEach(func() {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusCreated)
			})
			wrappedHandler = auth.NewWrapper(username, password).Wrap(handler)
		})

		It("works when the credentials are correct", func() {
			request := newRequest(username, password)
			wrappedHandler.ServeHTTP(httpRecorder, request)
			Expect(httpRecorder.Code).To(Equal(http.StatusCreated))
		})

		It("fails when the username is empty", func() {
			request := newRequest("", password)
			wrappedHandler.ServeHTTP(httpRecorder, request)
			Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
		})

		It("fails when the password is empty", func() {
			request := newRequest(username, "")
			wrappedHandler.ServeHTTP(httpRecorder, request)
			Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
		})

		It("fails when the credentials are wrong", func() {
			request := newRequest("thats", "apar")
			wrappedHandler.ServeHTTP(httpRecorder, request)
			Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
		})
	})

	Describe("wrapped handlerFunc", func() {
		var wrappedHandlerFunc http.HandlerFunc

		BeforeEach(func() {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusCreated)
			})
			wrappedHandlerFunc = auth.NewWrapper(username, password).WrapFunc(handler)
		})

		It("works when the credentials are correct", func() {
			request := newRequest(username, password)
			wrappedHandlerFunc.ServeHTTP(httpRecorder, request)
			Expect(httpRecorder.Code).To(Equal(http.StatusCreated))
		})

		It("fails when the username is empty", func() {
			request := newRequest("", password)
			wrappedHandlerFunc.ServeHTTP(httpRecorder, request)
			Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
		})

		It("fails when the password is empty", func() {
			request := newRequest(username, "")
			wrappedHandlerFunc.ServeHTTP(httpRecorder, request)
			Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
		})

		It("fails when the credentials are wrong", func() {
			request := newRequest("thats", "apar")
			wrappedHandlerFunc.ServeHTTP(httpRecorder, request)
			Expect(httpRecorder.Code).To(Equal(http.StatusUnauthorized))
		})
	})
})

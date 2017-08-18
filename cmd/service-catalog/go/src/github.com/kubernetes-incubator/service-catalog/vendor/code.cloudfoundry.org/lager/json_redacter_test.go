package lager_test


import (
	"code.cloudfoundry.org/lager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("JSON Redacter", func() {
	var (
	  resp []byte
	  err error
	  jsonRedacter *lager.JSONRedacter
	)

	BeforeEach(func() {
		jsonRedacter, err = lager.NewJSONRedacter(nil, []string{`amazonkey`, `AKIA[A-Z0-9]{16}`})
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when called with normal (non-secret) json", func() {
		BeforeEach(func(){
			resp = jsonRedacter.Redact([]byte(`{"foo":"bar"}`))
		})
		It("should return the same text", func(){
			Expect(resp).To(Equal([]byte(`{"foo":"bar"}`)))
		})
	})

	Context("when called with secrets inside the json", func() {
		BeforeEach(func() {
			resp = jsonRedacter.Redact([]byte(`{"foo":"fooval","password":"secret!","something":"AKIA1234567890123456"}`))
		})

		It("should redact the secrets", func() {
			Expect(resp).To(Equal([]byte(`{"foo":"fooval","password":"*REDACTED*","something":"*REDACTED*"}`)))
		})
	})

	Context("when a password flag is specified", func() {
		BeforeEach(func() {
			resp = jsonRedacter.Redact([]byte(`{"abcPwd":"abcd","password":"secret!","userpass":"fooval"}`))
		})

		It("should redact the secrets", func() {
			Expect(resp).To(Equal([]byte(`{"abcPwd":"*REDACTED*","password":"*REDACTED*","userpass":"*REDACTED*"}`)))
		})
	})

	Context("when called with an array of objects with a secret", func() {
		BeforeEach(func() {
			resp = jsonRedacter.Redact([]byte(`[{"foo":"fooval","password":"secret!","something":"amazonkey"}]`))
		})

		It("should redact the secrets", func() {
			Expect(resp).To(Equal([]byte(`[{"foo":"fooval","password":"*REDACTED*","something":"*REDACTED*"}]`)))
		})
	})

	Context("when called with a private key inside an array of strings", func() {
		BeforeEach(func() {
			resp = jsonRedacter.Redact([]byte(`["foo", "secret!", "amazonkey"]`))
		})

		It("should redact the amazonkey", func() {
			Expect(resp).To(Equal([]byte(`["foo","secret!","*REDACTED*"]`)))
		})
	})

	Context("when called with a private key inside a nested object", func() {
		BeforeEach(func() {
			resp = jsonRedacter.Redact([]byte(`{"foo":"fooval", "secret_stuff": {"password": "secret!"}}`))
		})

		It("should redact the amazonkey", func() {
			Expect(resp).To(Equal([]byte(`{"foo":"fooval","secret_stuff":{"password":"*REDACTED*"}}`)))
		})
	})
})

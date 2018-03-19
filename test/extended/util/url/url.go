package url

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	o "github.com/onsi/gomega"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kclientset "k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type Tester struct {
	client    kclientset.Interface
	namespace string
	podName   string
}

func NewTester(client kclientset.Interface, ns string) *Tester {
	return &Tester{client: client, namespace: ns}
}

func (ut *Tester) Close() {
	if err := ut.client.Core().Pods(ut.namespace).Delete(ut.podName, metav1.NewDeleteOptions(1)); err != nil {
		e2e.Logf("Failed to delete exec pod %s: %v", ut.podName, err)
	}
	ut.podName = ""
}

func (ut *Tester) Responses(tests ...*Test) []*Response {
	script := testsToScript(tests)
	if len(ut.podName) == 0 {
		name, err := createExecPod(ut.client, ut.namespace, "execpod")
		o.Expect(err).NotTo(o.HaveOccurred())
		ut.podName = name
	}
	output, err := e2e.RunHostCmd(ut.namespace, ut.podName, script)
	o.Expect(err).NotTo(o.HaveOccurred())
	responses, err := parseResponses(output)
	o.Expect(err).NotTo(o.HaveOccurred())
	if len(responses) != len(tests) {
		o.Expect(fmt.Errorf("number of tests did not match number of responses: %d and %d", len(responses), len(tests))).NotTo(o.HaveOccurred())
	}
	return responses
}

func (ut *Tester) Within(t time.Duration, tests ...*Test) {
	var errs []error
	failing := tests
	err := wait.PollImmediate(time.Second, t, func() (bool, error) {
		errs = errs[:0]
		responses := ut.Responses(failing...)
		var next []*Test
		for i, res := range responses {
			if err := failing[i].Test(i, res); err != nil {
				next = append(next, failing[i])
				errs = append(errs, err)
			}
		}
		e2e.Logf("%d/%d failed out of %d", len(errs), len(failing), len(tests))
		// perform one more loop if we haven't seen all tests pass at the same time
		if len(next) == 0 && len(failing) != len(tests) {
			failing = tests
			return false, nil
		}
		failing = next
		return len(errs) == 0, nil
	})
	if len(errs) > 0 {
		o.Expect(fmt.Errorf("%d/%d tests failed after %s: %v", len(errs), len(tests), t, errs))
	}
	o.Expect(err).ToNot(o.HaveOccurred())
}

// createExecPod creates a simple centos:7 pod in a sleep loop used as a
// vessel for kubectl exec commands.
// Returns the name of the created pod.
func createExecPod(clientset kclientset.Interface, ns, name string) (string, error) {
	e2e.Logf("Creating new exec pod")
	immediate := int64(0)
	execPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Command:         []string{"/bin/bash", "-c", "exec sleep 10000"},
					Name:            "hostexec",
					Image:           "centos:7",
					ImagePullPolicy: v1.PullIfNotPresent,
				},
			},
			HostNetwork:                   true,
			TerminationGracePeriodSeconds: &immediate,
		},
	}
	client := clientset.CoreV1()
	created, err := client.Pods(ns).Create(execPod)
	if err != nil {
		return "", err
	}
	err = wait.PollImmediate(e2e.Poll, 5*time.Minute, func() (bool, error) {
		retrievedPod, err := client.Pods(execPod.Namespace).Get(created.Name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return retrievedPod.Status.Phase == v1.PodRunning, nil
	})
	if err != nil {
		return "", err
	}
	return created.Name, nil
}

func testsToScript(tests []*Test) string {
	testScripts := []string{
		"set -euo pipefail",
		`function json_escape() {`,
		`  python -c 'import json,sys; print json.dumps(sys.stdin.read())'`,
		`}`,
	}
	for i, test := range tests {
		testScripts = append(testScripts, test.ToShell(i))
	}
	script := strings.Join(testScripts, "\n")
	return script
}

func parseResponses(out string) ([]*Response, error) {
	var responses []*Response
	d := json.NewDecoder(bytes.NewReader([]byte(out)))
	for i := 0; ; i++ {
		r := &Response{}
		if err := d.Decode(r); err != nil {
			if err == io.EOF {
				return responses, nil
			}
			return nil, fmt.Errorf("response %d could not be decoded: %v", i, err)
		}

		if i != r.Test {
			return nil, fmt.Errorf("response %d does not match test body %d", i, r.Test)
		}

		// parse the HTTP response
		res, err := http.ReadResponse(bufio.NewReader(bytes.NewBufferString(r.Headers)), nil)
		if err != nil {
			return nil, fmt.Errorf("response %d was unparseable: %v\n%s", i, err, r.Headers)
		}
		if res.StatusCode != r.CURL.Code {
			return nil, fmt.Errorf("response %d returned a different status code than was encoded in the headers:\n%s", i, r.Headers)
		}
		res.Body = ioutil.NopCloser(bytes.NewBuffer(r.Body))
		r.Response = res

		responses = append(responses, r)
	}
}

type Response struct {
	Test       int    `json:"test"`
	ReturnCode int    `json:"rc"`
	Error      string `json:"error"`

	CURL    CURL   `json:"curl"`
	Body    []byte `json:"body"`
	Headers string `json:"headers"`

	Response *http.Response
}

type CURL struct {
	Code int `json:"code"`
}

type Test struct {
	Name       string
	Req        *http.Request
	SkipVerify bool

	Wants []func(*http.Response) error
}

func Expect(method, url string) *Test {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		panic(err)
	}
	return &Test{
		Req: req,
	}
}

func (ut *Test) Through(addr string) *Test {
	ut.Req.Header.Set("Host", ut.Req.URL.Host)
	ut.Req.URL.Host = addr
	return ut
}

func (ut *Test) HasStatusCode(codes ...int) *Test {
	ut.Wants = append(ut.Wants, func(res *http.Response) error {
		for _, code := range codes {
			if res.StatusCode == code {
				return nil
			}
		}
		return fmt.Errorf("status code %d not in %v", res.StatusCode, codes)
	})
	return ut
}

func (ut *Test) RedirectsTo(url string, codes ...int) *Test {
	if len(codes) == 0 {
		codes = []int{http.StatusFound, http.StatusPermanentRedirect, http.StatusTemporaryRedirect}
	}
	ut.HasStatusCode(codes...)
	ut.Wants = append(ut.Wants, func(res *http.Response) error {
		location := res.Header.Get("Location")
		if location != url {
			return fmt.Errorf("Location header was %q, not %q", location, url)
		}
		return nil
	})
	return ut
}

func (ut *Test) SkipTLSVerification() *Test {
	ut.SkipVerify = true
	return ut
}

func (ut *Test) Test(i int, res *Response) error {
	if len(res.Error) > 0 || res.ReturnCode != 0 {
		return fmt.Errorf("test %d was not successful: %d %s", i, res.ReturnCode, res.Error)
	}
	for _, fn := range ut.Wants {
		if err := fn(res.Response); err != nil {
			return fmt.Errorf("test %d was not successful: %v", i, err)
		}
	}
	if len(ut.Wants) == 0 {
		if res.Response.StatusCode < 200 || res.Response.StatusCode >= 300 {
			return fmt.Errorf("test %d did not return a 2xx status code: %d", i, res.Response.StatusCode)
		}
	}
	return nil
}

func (ut *Test) ToShell(i int) string {
	var lines []string
	if len(ut.Name) > 0 {
		lines = append(lines, fmt.Sprintf("# Test: %s (%d)", ut.Name, i))
	} else {
		lines = append(lines, fmt.Sprintf("# Test: %d", i))
	}
	var headers []string
	for k, values := range ut.Req.Header {
		for _, v := range values {
			headers = append(headers, fmt.Sprintf("-H %q", k+":"+v))
		}
	}
	lines = append(lines, `rc=0`)
	cmd := fmt.Sprintf(`curl -X %s %s -s -S -o /tmp/body -D /tmp/headers %q`, ut.Req.Method, strings.Join(headers, " "), ut.Req.URL)
	cmd += ` -w '{"code":%{http_code}}'`
	if ut.SkipVerify {
		cmd += ` -k`
	}
	cmd += " 2>/tmp/error 1>/tmp/output || rc=$?"
	lines = append(lines, cmd)
	lines = append(lines, fmt.Sprintf(`echo "{\"test\":%d,\"rc\":$(echo $rc),\"curl\":$(cat /tmp/output),\"error\":$(cat /tmp/error | json_escape),\"body\":\"$(cat /tmp/body | base64 -w 0 -)\",\"headers\":$(cat /tmp/headers | json_escape)}"`, i))
	return strings.Join(lines, "\n")
}

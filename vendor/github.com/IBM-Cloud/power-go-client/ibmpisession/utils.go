package ibmpisession

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/IBM-Cloud/power-go-client/power/client"
	"github.com/IBM/go-sdk-core/v5/core"
	rc "github.com/IBM/platform-services-go-sdk/resourcecontrollerv2"
	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
)

const (
	SCHEME_HTTPS = "https"
	SCHEME_HTTP  = "http"
)

// fetchAuthorizationData Fetch Authorization token using the Authenticator
func fetchAuthorizationData(a core.Authenticator) (string, error) {
	req := &http.Request{
		Header: make(http.Header),
	}
	if err := a.Authenticate(req); err != nil {
		return "", err
	}
	return req.Header.Get("Authorization"), nil
}

// crnBuilder Return string format to create CRN using the cloud instance id
// The result string will have crn data with a string placeholder to set the cloud instance id
// Usage:
// `crn := fmt.Sprintf(crnBuilder(useraccount, regionZone, host), <cloudInstanceID>)`
func crnBuilder(useraccount, zone, host string) string {
	// TODO: after combined services, review this code to remove ppc-aas references
	var service string
	if strings.Contains(host, ".power-iaas.cloud.ibm.com") || strings.Contains(host, ".ppc-aas.cloud.ibm.com") {
		service = "bluemix"
	} else {
		service = "staging"
	}

	// Return crn
	if strings.Contains(host, "ppc-aas") {
		return fmt.Sprintf("crn:v1:%s:public:ppc-aas:%s:a/%s:", service, zone, useraccount) + "%s::"
	}
	return fmt.Sprintf("crn:v1:%s:public:power-iaas:%s:a/%s:", service, zone, useraccount) + "%s::"
}

func powerJSONConsumer() runtime.Consumer {
	return runtime.ConsumerFunc(func(reader io.Reader, data interface{}) error {
		buf := new(bytes.Buffer)
		_, err := buf.ReadFrom(reader)
		if err != nil {
			return err
		}
		b := buf.Bytes()
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.UseNumber() // preserve number formats
		return dec.Decode(data)
	})
}

// getPIClient generates a PowerIaas client
func getPIClient(debug bool, host string, scheme string) *client.PowerIaasAPI {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	}
	if scheme == "" {
		scheme = SCHEME_HTTPS
	}
	transport := httptransport.New(host, "/", []string{scheme})
	transport.Debug = debug
	transport.SetLogger(IBMPILogger{})
	transport.Consumers[runtime.JSONMime] = powerJSONConsumer()
	return client.New(transport, nil)
}

// costructRegionFromZone Calculate region based on location/zone
func costructRegionFromZone(zone string) string {
	var regex string
	if strings.Contains(zone, "-") {
		// it's a region or AZ
		regex = "-[0-9]+$"
	} else {
		// it's a datacenter
		regex = "[0-9]+$"
	}

	reg, _ := regexp.Compile(regex)
	return reg.ReplaceAllString(zone, "")
}

// SDKFailWithAPIError returns a custom error message if a HTTP error response 500 or greater is found
func SDKFailWithAPIError(err error, origErr error) error {
	if apierr, ok := err.(*runtime.APIError); ok {
		if apierr.Code >= 500 {
			return fmt.Errorf("error: %w The server has encountered an unexpected error and is unable to fulfill the request", err)
		}
	}
	return origErr
}

// CreateResourceControllerV2  returns a resourceControllerV2
func CreateResourceControllerV2(url string, authenticator core.Authenticator) (service *rc.ResourceControllerV2, err error) {
	env := ""
	if strings.Contains(url, "test") {
		env = ".test"
	}
	rcUrl := "https://resource-controller" + env + ".cloud.ibm.com"

	resourceController, err := rc.NewResourceControllerV2(&rc.ResourceControllerV2Options{
		Authenticator: authenticator,
		URL:           rcUrl,
	})
	return resourceController, err
}

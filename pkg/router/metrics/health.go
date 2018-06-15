package metrics

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/golang/glog"

	templateplugin "github.com/openshift/origin/pkg/router/template"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/kubernetes/pkg/probe"
	probehttp "k8s.io/kubernetes/pkg/probe/http"
)

var errBackend = fmt.Errorf("backend reported failure")

// HTTPBackendAvailable returns a healthz check that verifies a backend responds to a GET to
// the provided URL with 2xx or 3xx response.
func HTTPBackendAvailable(u *url.URL) healthz.HealthzChecker {
	p := probehttp.New()
	return healthz.NamedCheck("backend-http", func(r *http.Request) error {
		result, _, err := p.Probe(u, nil, 2*time.Second)
		if err != nil {
			return err
		}
		if result != probe.Success {
			return errBackend
		}
		return nil
	})
}

// HasSynced returns a healthz check that verifies the router has been synced at least
// once.
// routerPtr is a pointer because it may not yet be defined (there's a chicken-and-egg problem
//   with when the health checker and router object are set up).
func HasSynced(routerPtr **templateplugin.TemplatePlugin) (healthz.HealthzChecker, error) {
	if routerPtr == nil {
		return nil, fmt.Errorf("Nil routerPtr passed to HasSynced")
	}

	return healthz.NamedCheck("has-synced", func(r *http.Request) error {
		if *routerPtr == nil || !(*routerPtr).Router.SyncedAtLeastOnce() {
			return fmt.Errorf("Router not synced")
		}
		return nil
	}), nil
}

func ControllerLive() healthz.HealthzChecker {
	return healthz.NamedCheck("controller", func(r *http.Request) error {
		return nil
	})

}

// ProxyProtocolHTTPBackendAvailable returns a healthz check that verifies a backend supporting
// the HAProxy PROXY protocol responds to a GET to the provided URL with 2xx or 3xx response.
func ProxyProtocolHTTPBackendAvailable(u *url.URL) healthz.HealthzChecker {
	dialer := &net.Dialer{
		Timeout:   2 * time.Second,
		DualStack: true,
	}
	return healthz.NamedCheck("backend-proxy-http", func(r *http.Request) error {
		conn, err := dialer.Dial("tcp", u.Host)
		if err != nil {
			return err
		}
		conn.SetDeadline(time.Now().Add(2 * time.Second))
		br := bufio.NewReader(conn)
		if _, err := conn.Write([]byte("PROXY UNKNOWN\r\n")); err != nil {
			return err
		}
		req := &http.Request{Method: "GET", URL: u, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1}
		if err := req.Write(conn); err != nil {
			return err
		}
		res, err := http.ReadResponse(br, req)
		if err != nil {
			return err
		}

		// read full body
		defer res.Body.Close()
		if _, err := io.Copy(ioutil.Discard, res.Body); err != nil {
			glog.V(4).Infof("Error discarding probe body contents: %v", err)
		}

		if res.StatusCode < http.StatusOK && res.StatusCode >= http.StatusBadRequest {
			glog.V(4).Infof("Probe failed for %s, Response: %v", u.String(), res)
			return errBackend
		}
		glog.V(4).Infof("Probe succeeded for %s, Response: %v", u.String(), res)
		return nil
	})
}

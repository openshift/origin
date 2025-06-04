/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package server

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	goatomic "sync/atomic"
	"time"

	"go.uber.org/atomic"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/audit"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"
	netutils "k8s.io/utils/net"
)

// EventSink allows to create events.
type EventSink interface {
	Create(event *corev1.Event) (*corev1.Event, error)
	Destroy()
}

type OpenShiftGenericAPIServerPatch struct {
	// EventSink creates events.
	eventSink EventSink
	eventRef  *corev1.ObjectReference

	// when we emit the lifecycle events, we store the event ID of the first
	// shutdown event "ShutdownInitiated" emitted so we can correlate it to
	// the other shutdown events for a particular apiserver restart.
	// This provides a more deterministic way to determine the shutdown
	// duration for an apiserver restart
	eventLock                sync.Mutex
	shutdownInitiatedEventID types.UID
}

// Eventf creates an event with the API server as source, either in default namespace against default namespace, or
// if POD_NAME/NAMESPACE are set against that pod.
func (s *GenericAPIServer) Eventf(eventType, reason, messageFmt string, args ...interface{}) {
	t := metav1.NewTime(time.Now())
	host, _ := os.Hostname() // expicitly ignore error. Empty host is fine

	ref := *s.eventRef
	if len(ref.Namespace) == 0 {
		ref.Namespace = "default" // TODO: event broadcaster sets event ns to default. We have to match. Odd.
	}

	e := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v.%x", ref.Name, t.UnixNano()),
			Namespace: ref.Namespace,
		},
		FirstTimestamp: t,
		LastTimestamp:  t,
		Count:          1,
		InvolvedObject: ref,
		Reason:         reason,
		Message:        fmt.Sprintf(messageFmt, args...),
		Type:           eventType,
		Source:         corev1.EventSource{Component: "apiserver", Host: host},
	}

	func() {
		s.eventLock.Lock()
		defer s.eventLock.Unlock()
		if len(s.shutdownInitiatedEventID) != 0 {
			e.Related = &corev1.ObjectReference{
				UID: s.shutdownInitiatedEventID,
			}
		}
	}()

	klog.V(2).Infof("Event(%#v): type: '%v' reason: '%v' %v", e.InvolvedObject, e.Type, e.Reason, e.Message)

	ev, err := s.eventSink.Create(e)
	if err != nil {
		klog.Warningf("failed to create event %s/%s: %v", e.Namespace, e.Name, err)
		return
	}

	if ev != nil && ev.Reason == "ShutdownInitiated" {
		// we have successfully created the shutdown initiated event,
		// all consecutive shutdown events we are going to write for
		// this restart can be tied to this initiated event
		s.eventLock.Lock()
		defer s.eventLock.Unlock()
		if len(s.shutdownInitiatedEventID) == 0 {
			s.shutdownInitiatedEventID = ev.GetUID()
		}
	}
}

func eventReference() (*corev1.ObjectReference, error) {
	ns := os.Getenv("POD_NAMESPACE")
	pod := os.Getenv("POD_NAME")
	if len(ns) == 0 && len(pod) > 0 {
		serviceAccountNamespaceFile := "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
		if _, err := os.Stat(serviceAccountNamespaceFile); err == nil {
			bs, err := ioutil.ReadFile(serviceAccountNamespaceFile)
			if err != nil {
				return nil, err
			}
			ns = string(bs)
		}
	}
	if len(ns) == 0 {
		pod = ""
		ns = "openshift-kube-apiserver"
	}
	if len(pod) == 0 {
		return &corev1.ObjectReference{
			Kind:       "Namespace",
			Name:       ns,
			APIVersion: "v1",
		}, nil
	}

	return &corev1.ObjectReference{
		Kind:       "Pod",
		Namespace:  ns,
		Name:       pod,
		APIVersion: "v1",
	}, nil
}

// terminationLoggingListener wraps the given listener to mark late connections
// as such, identified by the remote address. In parallel, we have a filter that
// logs bad requests through these connections. We need this filter to get
// access to the http path in order to filter out healthz or readyz probes that
// are allowed at any point during termination.
//
// Connections are late after the lateStopCh has been closed.
type terminationLoggingListener struct {
	net.Listener
	lateStopCh <-chan struct{}
}

type eventfFunc func(eventType, reason, messageFmt string, args ...interface{})

var (
	lateConnectionRemoteAddrsLock sync.RWMutex
	lateConnectionRemoteAddrs     = map[string]bool{}

	unexpectedRequestsEventf goatomic.Value
)

func (l *terminationLoggingListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	select {
	case <-l.lateStopCh:
		lateConnectionRemoteAddrsLock.Lock()
		defer lateConnectionRemoteAddrsLock.Unlock()
		lateConnectionRemoteAddrs[c.RemoteAddr().String()] = true
	default:
	}

	return c, nil
}

// WithLateConnectionFilter logs every non-probe request that comes through a late connection identified by remote address.
func WithLateConnectionFilter(handler http.Handler) http.Handler {
	var lateRequestReceived atomic.Bool

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lateConnectionRemoteAddrsLock.RLock()
		late := lateConnectionRemoteAddrs[r.RemoteAddr]
		lateConnectionRemoteAddrsLock.RUnlock()

		if late {
			if pth := "/" + strings.TrimLeft(r.URL.Path, "/"); pth != "/readyz" && pth != "/healthz" && pth != "/livez" {
				if isLocal(r) {
					audit.AddAuditAnnotation(r.Context(), "openshift.io/during-graceful", fmt.Sprintf("loopback=true,%v,readyz=false", r.URL.Host))
					klog.V(4).Infof("Loopback request to %q (user agent %q) through connection created very late in the graceful termination process (more than 80%% has passed). This client probably does not watch /readyz and might get failures when termination is over.", r.URL.Path, r.UserAgent())
				} else {
					audit.AddAuditAnnotation(r.Context(), "openshift.io/during-graceful", fmt.Sprintf("loopback=false,%v,readyz=false", r.URL.Host))
					klog.Warningf("Request to %q (source IP %s, user agent %q) through a connection created very late in the graceful termination process (more than 80%% has passed), possibly a sign for a broken load balancer setup.", r.URL.Path, r.RemoteAddr, r.UserAgent())

					// create only one event to avoid event spam.
					var eventf eventfFunc
					eventf, _ = unexpectedRequestsEventf.Load().(eventfFunc)
					if swapped := lateRequestReceived.CAS(false, true); swapped && eventf != nil {
						eventf(corev1.EventTypeWarning, "LateConnections", "The apiserver received connections (e.g. from %q, user agent %q) very late in the graceful termination process, possibly a sign for a broken load balancer setup.", r.RemoteAddr, r.UserAgent())
					}
				}
			}
		}

		handler.ServeHTTP(w, r)
	})
}

// WithNonReadyRequestLogging rejects the request until the process has been ready once.
func WithNonReadyRequestLogging(handler http.Handler, hasBeenReadySignal lifecycleSignal) http.Handler {
	if hasBeenReadySignal == nil {
		return handler
	}

	var nonReadyRequestReceived atomic.Bool

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-hasBeenReadySignal.Signaled():
			handler.ServeHTTP(w, r)
			return
		default:
		}

		// ignore connections to local IP. Those clients better know what they are doing.
		if pth := "/" + strings.TrimLeft(r.URL.Path, "/"); pth != "/readyz" && pth != "/healthz" && pth != "/livez" {
			if isLocal(r) {
				if !isKubeApiserverLoopBack(r) {
					audit.AddAuditAnnotation(r.Context(), "openshift.io/unready", fmt.Sprintf("loopback=true,%v,readyz=false", r.URL.Host))
					klog.V(2).Infof("Loopback request to %q (user agent %q) before server is ready. This client probably does not watch /readyz and might get inconsistent answers.", r.URL.Path, r.UserAgent())
				}
			} else {
				audit.AddAuditAnnotation(r.Context(), "openshift.io/unready", fmt.Sprintf("loopback=false,%v,readyz=false", r.URL.Host))
				klog.Warningf("Request to %q (source IP %s, user agent %q) before server is ready, possibly a sign for a broken load balancer setup.", r.URL.Path, r.RemoteAddr, r.UserAgent())

				// create only one event to avoid event spam.
				var eventf eventfFunc
				eventf, _ = unexpectedRequestsEventf.Load().(eventfFunc)
				if swapped := nonReadyRequestReceived.CAS(false, true); swapped && eventf != nil {
					eventf(corev1.EventTypeWarning, "NonReadyRequests", "The kube-apiserver received requests (e.g. from %q, user agent %q, accessing %s) before it was ready, possibly a sign for a broken load balancer setup.", r.RemoteAddr, r.UserAgent(), r.URL.Path)
				}
			}
		}

		handler.ServeHTTP(w, r)
	})
}

func isLocal(req *http.Request) bool {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		// ignore error and keep going
	} else if ip := netutils.ParseIPSloppy(host); ip != nil {
		return ip.IsLoopback()
	}

	return false
}

func isKubeApiserverLoopBack(req *http.Request) bool {
	return strings.HasPrefix(req.UserAgent(), "kube-apiserver/")
}

type nullEventSink struct{}

func (nullEventSink) Create(event *corev1.Event) (*corev1.Event, error) {
	return nil, nil
}

func (nullEventSink) Destroy() {
}

type clientEventSink struct {
	*v1.EventSinkImpl
}

func (clientEventSink) Destroy() {
}

/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package unidler

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/proxy"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"

	"github.com/openshift/origin/pkg/proxy/userspace"
)

const (
	UDPBufferSize  = 4096 // 4KiB should be enough for most whole-packets
	NeedPodsReason = "NeedPods"
)

var endpointDialTimeout = []time.Duration{250 * time.Millisecond, 500 * time.Millisecond, 1 * time.Second, 2 * time.Second}

type connectionList struct {
	conns   []heldConn
	maxSize int

	tickSize       time.Duration
	timeSinceStart time.Duration
	timeout        time.Duration

	svcName string
}

type heldConn struct {
	net.Conn
	connectedAt time.Duration
}

func newConnectionList(maxSize int, tickSize time.Duration, timeout time.Duration, svcName string) *connectionList {
	return &connectionList{
		conns:          []heldConn{},
		maxSize:        maxSize,
		tickSize:       tickSize,
		timeSinceStart: 0,
		timeout:        timeout,
		svcName:        svcName,
	}
}

func (l *connectionList) Add(conn net.Conn) {
	if len(l.conns) >= l.maxSize {
		// TODO: look for closed connections
		utilruntime.HandleError(fmt.Errorf("max connections exceeded while waiting for idled service %s to awaken, dropping oldest", l.svcName))
		var oldConn net.Conn
		oldConn, l.conns = l.conns[0], l.conns[1:]
		oldConn.Close()
	}

	l.conns = append(l.conns, heldConn{conn, l.timeSinceStart})
}

func (l *connectionList) Tick() {
	l.timeSinceStart += l.tickSize
	l.cleanOldConnections()
}

func (l *connectionList) cleanOldConnections() {
	cleanInd := -1
	for i, conn := range l.conns {
		if l.timeSinceStart-conn.connectedAt < l.timeout {
			cleanInd = i
			break
		}
	}

	if cleanInd > 0 {
		oldConns := l.conns[:cleanInd]
		l.conns = l.conns[cleanInd:]
		utilruntime.HandleError(fmt.Errorf("timed out %v connections while waiting for idled service %s to awaken.", len(oldConns), l.svcName))

		for _, conn := range oldConns {
			conn.Close()
		}
	}
}

func (l *connectionList) GetConns() []net.Conn {
	conns := make([]net.Conn, len(l.conns))
	for i, conn := range l.conns {
		conns[i] = conn.Conn
	}
	return conns
}

func (l *connectionList) Len() int {
	return len(l.conns)
}

func (l *connectionList) Clear() {
	for _, conn := range l.conns {
		conn.Close()
	}

	l.conns = []heldConn{}
}

var (
	// MaxHeldConnections is the maximum number of TCP connections per service that
	// will be held by the unidler at once (new connections will cause older ones
	// to be dropped after the limit is reached)
	MaxHeldConnections = 16

	needPodsWaitTimeout = 30 * time.Second
	needPodsTickLen     = 5 * time.Second
)

func newUnidlerSocket(protocol api.Protocol, ip net.IP, port int, signaler NeedPodsSignaler) (userspace.ProxySocket, error) {
	host := ""
	if ip != nil {
		host = ip.String()
	}

	switch strings.ToUpper(string(protocol)) {
	case "TCP":
		listener, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
		if err != nil {
			return nil, err
		}
		return &tcpUnidlerSocket{Listener: listener, port: port, signaler: signaler}, nil
	case "UDP":
		addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, strconv.Itoa(port)))
		if err != nil {
			return nil, err
		}
		conn, err := net.ListenUDP("udp", addr)
		if err != nil {
			return nil, err
		}
		return &udpUnidlerSocket{UDPConn: conn, port: port, signaler: signaler}, nil
	}
	return nil, fmt.Errorf("unknown protocol %q", protocol)
}

// tcpUnidlerSocket implements proxySocket.  Close() is implemented by net.Listener.  When Close() is called,
// no new connections are allowed but existing connections are left untouched.
type tcpUnidlerSocket struct {
	net.Listener
	port     int
	signaler NeedPodsSignaler
}

func (tcp *tcpUnidlerSocket) ListenPort() int {
	return tcp.port
}

func (tcp *tcpUnidlerSocket) waitForEndpoints(ch chan<- interface{}, service proxy.ServicePortName, loadBalancer userspace.LoadBalancer) {
	defer close(ch)
	for {
		if loadBalancer.ServiceHasEndpoints(service) {
			// we have endpoints now, so we're finished
			return
		}

		// otherwise, wait a bit before checking for endpoints again
		time.Sleep(endpointDialTimeout[0])
	}
}

func (tcp *tcpUnidlerSocket) acceptConns(ch chan<- net.Conn, svcInfo *userspace.ServiceInfo) {
	defer close(ch)

	// Block until a connection is made.
	for {
		inConn, err := tcp.Accept()
		if err != nil {
			if isTooManyFDsError(err) {
				panic("Accept failed: " + err.Error())
			}

			// TODO: indicate errors here?
			if isClosedError(err) {
				return
			}
			if !svcInfo.IsAlive() {
				// Then the service port was just closed so the accept failure is to be expected.
				return
			}
			utilruntime.HandleError(fmt.Errorf("Accept failed: %v", err))
			continue
		}

		ch <- inConn
	}
}

// awaitAwakening collects new connections and signals once that pods are needed to fulfill them.  The function
// will return when the listening socket is closed, which indicates that endpoints have succesfully appeared
// (and thus the hybrid proxy has switched this service over to using the normal proxy).  Connections will
// be gradually timed out and dropped off the list of connections on a per-connection basis.  The list of current
// connections is returned, in addition to whether or not we should retry this method.
func (tcp *tcpUnidlerSocket) awaitAwakening(service proxy.ServicePortName, serviceRef api.ObjectReference, loadBalancer userspace.LoadBalancer, inConns <-chan net.Conn, endpointsAvail chan<- interface{}) (*connectionList, bool) {
	// collect connections and wait for endpoints to be available
	sent_need_pods := false
	timeout_started := false
	ticker := time.NewTicker(needPodsTickLen)
	defer ticker.Stop()
	svcName := fmt.Sprintf("%s/%s:%s", service.Namespace, service.Name, service.Port)
	allConns := newConnectionList(MaxHeldConnections, needPodsTickLen, needPodsWaitTimeout, svcName)

	for {
		select {
		case inConn, ok := <-inConns:
			if !ok {
				// the listen socket has been closed, so we're finished accepting connections
				return allConns, false
			}

			if !sent_need_pods && !loadBalancer.ServiceHasEndpoints(service) {
				glog.V(4).Infof("unidling TCP proxy sent unidle event to wake up service %s/%s:%s", service.Namespace, service.Name, service.Port)
				tcp.signaler.NeedPods(serviceRef, service.Port)

				// only send NeedPods once
				sent_need_pods = true
				timeout_started = true
			}

			if allConns.Len() == 0 {
				if !loadBalancer.ServiceHasEndpoints(service) {
					// notify us when endpoints are available
					go tcp.waitForEndpoints(endpointsAvail, service, loadBalancer)
				}
			}

			allConns.Add(inConn)
			glog.V(4).Infof("unidling TCP proxy has accumulated %v connections while waiting for service %s/%s:%s to unidle", allConns.Len(), service.Namespace, service.Name, service.Port)
		case <-ticker.C:
			if !timeout_started {
				continue
			}
			// TODO: timeout each connection (or group of connections) separately
			// timed out, close all waiting connections and reset the state
			allConns.Tick()
		}
	}
}

func (tcp *tcpUnidlerSocket) ProxyLoop(service proxy.ServicePortName, svcInfo *userspace.ServiceInfo, loadBalancer userspace.LoadBalancer) {
	if !svcInfo.IsAlive() {
		// The service port was closed or replaced.
		return
	}

	// accept connections asynchronously
	inConns := make(chan net.Conn)
	go tcp.acceptConns(inConns, svcInfo)

	endpointsAvail := make(chan interface{})
	var allConns *connectionList

	for {
		glog.V(4).Infof("unidling TCP proxy start/reset for service %s/%s:%s", service.Namespace, service.Name, service.Port)

		var cont bool
		if allConns, cont = tcp.awaitAwakening(service, svcInfo.ServiceRef, loadBalancer, inConns, endpointsAvail); !cont {
			break
		}
	}

	glog.V(4).Infof("unidling TCP proxy waiting for endpoints for service %s/%s:%s to become available with %v accumulated connections", service.Namespace, service.Name, service.Port, allConns.Len())
	// block until we have endpoints available
	select {
	case _, ok := <-endpointsAvail:
		if ok {
			close(endpointsAvail)
			// this shouldn't happen (ok should always be false)
		}
	case <-time.NewTimer(needPodsWaitTimeout).C:
		if allConns.Len() > 0 {
			utilruntime.HandleError(fmt.Errorf("timed out %v TCP connections while waiting for idled service %s/%s:%s to awaken.", allConns.Len(), service.Namespace, service.Name, service.Port))
			allConns.Clear()
		}
		return
	}
	glog.V(4).Infof("unidling TCP proxy got endpoints for service %s/%s:%s, connecting %v accumulated connections", service.Namespace, service.Name, service.Port, allConns.Len())

	for _, inConn := range allConns.GetConns() {
		glog.V(3).Infof("Accepted TCP connection from %v to %v", inConn.RemoteAddr(), inConn.LocalAddr())
		outConn, err := userspace.TryConnectEndpoints(service, inConn.(*net.TCPConn).RemoteAddr(), "tcp", loadBalancer)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Failed to connect to balancer: %v", err))
			inConn.Close()
			continue
		}
		// Spin up an async copy loop.
		go userspace.ProxyTCP(inConn.(*net.TCPConn), outConn.(*net.TCPConn))
	}
}

// udpUnidlerSocket implements proxySocket.  Close() is implemented by net.UDPConn.  When Close() is called,
// no new connections are allowed and existing connections are broken.
// TODO: We could lame-duck this ourselves, if it becomes important.
type udpUnidlerSocket struct {
	*net.UDPConn
	port     int
	signaler NeedPodsSignaler
}

func (udp *udpUnidlerSocket) ListenPort() int {
	return udp.port
}

func (udp *udpUnidlerSocket) Addr() net.Addr {
	return udp.LocalAddr()
}

// readFromSock tries to read from a socket, returning true if we should continue trying
// to read again, or false if no further reads should be made.
func (udp *udpUnidlerSocket) readFromSock(buffer []byte, svcInfo *userspace.ServiceInfo) bool {
	if !svcInfo.IsAlive() {
		// The service port was closed or replaced.
		return false
	}

	// Block until data arrives.
	// TODO: Accumulate a histogram of n or something, to fine tune the buffer size.
	_, _, err := udp.ReadFrom(buffer)
	if err != nil {
		if e, ok := err.(net.Error); ok {
			if e.Temporary() {
				glog.V(1).Infof("ReadFrom had a temporary failure: %v", err)
				return true
			}
		}
		utilruntime.HandleError(fmt.Errorf("ReadFrom failed, exiting ProxyLoop: %v", err))
		return false
	}

	return true
}

func (udp *udpUnidlerSocket) sendWakeup(svcPortName proxy.ServicePortName, svcInfo *userspace.ServiceInfo) *time.Timer {
	timeoutTimer := time.NewTimer(needPodsWaitTimeout)
	glog.V(4).Infof("unidling proxy sent unidle event to wake up service %s/%s:%s", svcPortName.Namespace, svcPortName.Name, svcPortName.Port)
	udp.signaler.NeedPods(svcInfo.ServiceRef, svcPortName.Port)

	return timeoutTimer
}

func (udp *udpUnidlerSocket) ProxyLoop(svcPortName proxy.ServicePortName, svcInfo *userspace.ServiceInfo, loadBalancer userspace.LoadBalancer) {
	// just drop the packets on the floor until we have endpoints
	var buffer [UDPBufferSize]byte

	glog.V(4).Infof("unidling proxy UDP proxy waiting for data for service %s/%s:%s", svcPortName.Namespace, svcPortName.Name, svcPortName.Port)

	if !udp.readFromSock(buffer[0:], svcInfo) {
		return
	}

	wakeupTimeoutTimer := udp.sendWakeup(svcPortName, svcInfo)

	for {
		if !udp.readFromSock(buffer[0:], svcInfo) {
			break
		}

		// reset the timer whenever we receive data, and only signal if we've
		// the timer had timed out when we reset it (so only send one even for
		// each "burst" of data we get).
		if active := wakeupTimeoutTimer.Reset(needPodsWaitTimeout); !active {
			wakeupTimeoutTimer = udp.sendWakeup(svcPortName, svcInfo)
		}
	}
}

func isTooManyFDsError(err error) bool {
	return strings.Contains(err.Error(), "too many open files")
}

func isClosedError(err error) bool {
	// A brief discussion about handling closed error here:
	// https://code.google.com/p/go/issues/detail?id=4373#c14
	// TODO: maybe create a stoppable TCP listener that returns a StoppedError
	return strings.HasSuffix(err.Error(), "use of closed network connection")
}

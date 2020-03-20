/*
Copyright 2019 The Kubernetes Authors.

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

package agentserver

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"

	"k8s.io/klog"
	"sigs.k8s.io/apiserver-network-proxy/konnectivity-client/proto/client"
)

// Tunnel implements Proxy based on HTTP Connect, which tunnels the traffic to
// the agent registered in ProxyServer.
type Tunnel struct {
	Server *ProxyServer
}

func (t *Tunnel) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	klog.Infof("Received %s request to %q", r.Method, r.Host)
	if r.TLS != nil {
		klog.Infof("TLS CommonName: %v", r.TLS.PeerCertificates[0].Subject.CommonName)
	}
	if r.Method != http.MethodConnect {
		http.Error(w, "this proxy only supports CONNECT passthrough", http.StatusMethodNotAllowed)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)

	conn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	random := rand.Int63()
	dialRequest := &client.Packet{
		Type: client.PacketType_DIAL_REQ,
		Payload: &client.Packet_DialRequest{
			DialRequest: &client.DialRequest{
				Protocol: "tcp",
				Address:  r.Host,
				Random:   random,
			},
		},
	}
	klog.Infof("Set pending(rand=%d) to %v", random, w)
	connected := make(chan struct{})
	connection := &ProxyClientConnection{
		Mode:      "http-connect",
		HTTP:      conn,
		connected: connected,
	}
	t.Server.PendingDial[random] = connection
	backend, err := t.Server.BackendManager.Backend()
	if err != nil {
		http.Error(w, fmt.Sprintf("currently no tunnels available: %v", err), http.StatusInternalServerError)
	}
	if err := backend.Send(dialRequest); err != nil {
		klog.Errorf("failed to tunnel dial request %v", err)
		return
	}
	ctxt := backend.Context()
	if ctxt.Err() != nil {
		klog.Errorf("context reports error %v", err)
	}

	select {
	case <-ctxt.Done():
		klog.Errorf("context reports done!!!")
	default:
	}

	select {
	case <-connection.connected: // Waiting for response before we begin full communication.
	}

	defer conn.Close()

	klog.Infof("Starting proxy to %q", r.Host)
	pkt := make([]byte, 1<<12)

	for {
		n, err := conn.Read(pkt[:])
		if err == io.EOF {
			klog.Warningf("EOF from %v", r.Host)
			break
		}
		if err != nil {
			klog.Errorf("Received error on connection %v", err)
			break
		}

		packet := &client.Packet{
			Type: client.PacketType_DATA,
			Payload: &client.Packet_Data{
				Data: &client.Data{
					ConnectID: connection.connectID,
					Data:      pkt[:n],
				},
			},
		}
		err = backend.Send(packet)
		if err != nil {
			klog.Errorf("error sending packet %v", err)
			continue
		}
		klog.Infof("Forwarding on tunnel, packet type: %s", packet.Type)
	}

	klog.Infof("Stopping transfer to %q", r.Host)
}

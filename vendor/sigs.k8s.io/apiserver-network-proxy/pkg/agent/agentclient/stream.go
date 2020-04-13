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

package agentclient

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/metadata"
	"k8s.io/klog"
	"sigs.k8s.io/apiserver-network-proxy/konnectivity-client/proto/client"
	"sigs.k8s.io/apiserver-network-proxy/proto/agent"
	"sigs.k8s.io/apiserver-network-proxy/proto/header"
)

const (
	defaultReconnectInterval = 5 * time.Second
)

type ReconnectError struct {
	internalErr error
	errChan     <-chan error
}

func (e *ReconnectError) Error() string {
	return "transient error: " + e.internalErr.Error()
}

func (e *ReconnectError) Wait() error {
	return <-e.errChan
}

type RedialableAgentClient struct {
	cs *ClientSet // the clientset that includes this RedialableAgentClient.

	stream agent.AgentService_ConnectClient

	agentID     string
	serverID    string // the id of the proxy server this client connects to.
	serverCount int    // the number of the proxy server instances.

	// connect opts
	address       string
	opts          []grpc.DialOption
	conn          *grpc.ClientConn
	stopCh        chan struct{}
	reconnOngoing bool
	reconnWaiters []chan error

	// locks
	sendLock   sync.Mutex
	recvLock   sync.Mutex
	reconnLock sync.Mutex

	reconnectInterval time.Duration // interval between recoonects
	probeInterval     time.Duration // interval between probe pings

	// file path contains service account token.
	// token's value is auto-rotated by kubernetes, based on projected volume configuration.
	serviceAccountTokenPath string
}

func copyRedialableAgentClient(in RedialableAgentClient) RedialableAgentClient {
	out := in
	out.stopCh = make(chan struct{})
	out.reconnOngoing = false
	out.reconnWaiters = nil
	out.sendLock = sync.Mutex{}
	out.recvLock = sync.Mutex{}
	out.reconnLock = sync.Mutex{}
	return out
}

func NewRedialableAgentClient(address, agentID string, cs *ClientSet, opts ...grpc.DialOption) (*RedialableAgentClient, error) {
	c := &RedialableAgentClient{
		cs:                      cs,
		address:                 address,
		agentID:                 agentID,
		opts:                    opts,
		probeInterval:           cs.probeInterval,
		reconnectInterval:       cs.reconnectInterval,
		stopCh:                  make(chan struct{}),
		serviceAccountTokenPath: cs.serviceAccountTokenPath,
	}
	serverID, err := c.Connect()
	if err != nil {
		return nil, err
	}
	c.serverID = serverID
	return c, nil
}

func (c *RedialableAgentClient) probe() {
	for {
		select {
		case <-c.stopCh:
			return
		case <-time.After(c.probeInterval):
			if c.conn == nil {
				continue
			}
			// health check
			if c.conn.GetState() == connectivity.Ready {
				continue
			} else {
				klog.Infof("Connection state %v", c.conn.GetState())
			}
		}

		klog.Info("probe failure: reconnect")
		if err := <-c.triggerReconnect(); err != nil {
			klog.Infof("probe reconnect failed: %v", err)
		}
	}
}

func (c *RedialableAgentClient) Send(pkt *client.Packet) error {
	c.sendLock.Lock()
	defer c.sendLock.Unlock()

	if err := c.stream.Send(pkt); err != nil {
		if err == io.EOF {
			return err
		}
		return &ReconnectError{
			internalErr: err,
			errChan:     c.triggerReconnect(),
		}
	}

	return nil
}

func (c *RedialableAgentClient) RetrySend(pkt *client.Packet) error {
	err := c.Send(pkt)
	if err == nil {
		return nil
	} else if err == io.EOF {
		return err
	}

	if err2, ok := err.(*ReconnectError); ok {
		err = err2.Wait()
	}
	if err != nil {
		return err
	}
	return c.RetrySend(pkt)
}

func (c *RedialableAgentClient) triggerReconnect() <-chan error {
	c.reconnLock.Lock()
	defer c.reconnLock.Unlock()

	errch := make(chan error)
	c.reconnWaiters = append(c.reconnWaiters, errch)

	if !c.reconnOngoing {
		go c.reconnect()
		c.reconnOngoing = true
	}

	return errch
}

func (c *RedialableAgentClient) doneReconnect(err error) {
	c.reconnLock.Lock()
	defer c.reconnLock.Unlock()

	for _, ch := range c.reconnWaiters {
		ch <- err
	}
	c.reconnOngoing = false
	c.reconnWaiters = nil
}

func (c *RedialableAgentClient) Recv() (*client.Packet, error) {
	c.recvLock.Lock()
	defer c.recvLock.Unlock()

	var pkt *client.Packet
	var err error

	if pkt, err = c.stream.Recv(); err != nil {
		if err == io.EOF {
			return pkt, err
		}
		return pkt, &ReconnectError{
			internalErr: err,
			errChan:     c.triggerReconnect(),
		}
	}

	return pkt, nil
}

// Connect makes the grpc dial to the proxy server. It returns the serverID
// it connects to.
func (c *RedialableAgentClient) Connect() (string, error) {
	var err error
	r, err := c.tryConnect()
	if err != nil {
		return "", err
	}
	c.serverID = r.serverID
	klog.Infof("Connect to server %s", r.serverID)
	c.serverCount = r.serverCount
	c.conn = r.grpcConn
	c.stream = r.agentServiceClient
	return c.serverID, nil
}

// The goal is to make the chance that client's Connect rpc call has never hit
// the wanted server after "retries" times to be lower than 10^-2.
func retryLimit(serverCount int) (retries int) {
	switch serverCount {
	case 1:
		return 3 // to overcome transient errors
	case 2:
		return 3 + 7
	case 3:
		return 3 + 12
	case 4:
		return 3 + 17
	case 5:
		return 3 + 21
	default:
		// we don't expect HA server with more than 5 instances.
		klog.Warningf("unexpected to handle %d proxy servers, the limit is 5.", serverCount)
		return 3 + 21
	}
}

func (c *RedialableAgentClient) reconnect() {
	klog.Info("start to reconnect...")

	var retry, limit int

	limit = retryLimit(c.serverCount)
	for retry < limit {
		r, err := c.tryConnect()
		if err != nil {
			retry++
			klog.Infof("Failed to connect to proxy server, retry %d in %v: %v", retry, c.reconnectInterval, err)
			time.Sleep(c.reconnectInterval)
			continue
		}
		switch {
		case r.serverID == c.serverID:
			klog.Infof("reconnected to %s", r.serverID)
			c.conn = r.grpcConn
			c.stream = r.agentServiceClient
			c.doneReconnect(nil)
			return

		case r.serverID != c.serverID && c.cs.HasID(r.serverID):
			// reset the connection
			err := r.grpcConn.Close()
			if err != nil {
				klog.Infof("failed to close connection to %s: %v", r.serverID, err)
			}
			retry++
			klog.Infof("Trying to reconnect to proxy server %s, got connected to proxy server %s, for which there is already a connection, retry %d in %v", c.serverID, r.serverID, retry, c.reconnectInterval)
			time.Sleep(c.reconnectInterval)
		case r.serverID != c.serverID && !c.cs.HasID(r.serverID):
			// create a new client
			cc := copyRedialableAgentClient(*c)
			cc.stream = r.agentServiceClient
			cc.conn = r.grpcConn
			cc.serverID = r.serverID
			ac := newAgentClientWithRedialableAgentClient(&cc)
			err := c.cs.AddClient(r.serverID, ac)
			if err != nil {
				klog.Infof("failed to add client for %s: %v", r.serverID, err)
			}
			go ac.Serve()
			retry++
			klog.Infof("Trying to reconnect to proxy server %s, got connected to proxy server %s. We will add this connection to the client set, but keep retrying connecting to proxy server %s, retry %d in %v", c.serverID, r.serverID, c.serverID, retry, c.reconnectInterval)
			time.Sleep(c.reconnectInterval)
		}
	}

	c.cs.RemoveClient(c.serverID)
	close(c.stopCh)
	c.doneReconnect(fmt.Errorf("Failed to connect to proxy server"))
}

func serverCount(stream agent.AgentService_ConnectClient) (int, error) {
	md, err := stream.Header()
	if err != nil {
		return 0, err
	}
	scounts := md.Get(header.ServerCount)
	if len(scounts) == 0 {
		return 0, fmt.Errorf("missing server count")
	}
	scount := scounts[0]
	return strconv.Atoi(scount)
}

func serverID(stream agent.AgentService_ConnectClient) (string, error) {
	// TODO: this is a blocking call. Add a timeout?
	md, err := stream.Header()
	if err != nil {
		return "", err
	}
	sids := md.Get(header.ServerID)
	if len(sids) != 1 {
		return "", fmt.Errorf("expected one server ID in the context, got %v", sids)
	}
	return sids[0], nil
}

type connectResult struct {
	serverID           string
	serverCount        int
	grpcConn           *grpc.ClientConn
	agentServiceClient agent.AgentService_ConnectClient
}

func (c *RedialableAgentClient) initializeAuthContext(ctx context.Context) (context.Context, error) {
	var err error
	var b []byte

	// load current service account's token value
	if b, err = ioutil.ReadFile(c.serviceAccountTokenPath); err != nil {
		klog.Errorf("Failed to read token from %q. err: %v", c.serviceAccountTokenPath, err)
		return nil, err
	}
	ctx = metadata.AppendToOutgoingContext(ctx, header.AuthenticationTokenContextKey, header.AuthenticationTokenContextSchemePrefix+string(b))

	return ctx, nil
}

// tryConnect makes the grpc dial to the proxy server. It returns the serverID
// it connects to, and the number of servers (1 if server is non-HA). It also
// updates c.stream.
func (c *RedialableAgentClient) tryConnect() (connectResult, error) {
	var err error

	conn, err := grpc.Dial(c.address, c.opts...)
	if err != nil {
		return connectResult{}, err
	}

	ctx := metadata.AppendToOutgoingContext(context.Background(), header.AgentID, c.agentID)
	if c.serviceAccountTokenPath != "" {
		if ctx, err = c.initializeAuthContext(ctx); err != nil {
			return connectResult{}, err
		}
	}
	stream, err := agent.NewAgentServiceClient(conn).Connect(ctx)
	if err != nil {
		return connectResult{}, err
	}
	sid, err := serverID(stream)
	if err != nil {
		return connectResult{}, err
	}
	count, err := serverCount(stream)
	if err != nil {
		return connectResult{}, err
	}
	r := connectResult{
		serverID:           sid,
		serverCount:        count,
		grpcConn:           conn,
		agentServiceClient: stream,
	}
	return r, err
}

func (c *RedialableAgentClient) Close() {
	if c.conn == nil {
		klog.Warning("Unexpected empty RedialableAgentClient.stream")
	}
	c.conn.Close()
}

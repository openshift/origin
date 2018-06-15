package ovsclient

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"time"
)

// Client is an RPC client for communicating with OVS.
type Client struct {
	*rpc.Client
	conn net.Conn
}

// New creates a new Client from a connection.
func New(conn net.Conn) *Client {
	return &Client{
		Client: jsonrpc.NewClient(conn),
		conn:   conn,
	}
}

// DialTimeout dials the provided network and address, and if it responds within
// timeout will return a valid Client.
func DialTimeout(network, addr string, timeout time.Duration) (*Client, error) {
	conn, err := net.DialTimeout(network, addr, timeout)
	if err != nil {
		return nil, err
	}
	return New(conn), nil
}

// Ping returns nil if the OVS server responded to an "echo" command.
func (c *Client) Ping() error {
	var result interface{}
	if err := c.Call("echo", []string{"hello"}, &result); err != nil {
		return err
	}
	return nil
}

// WaitForDisconnect will block until the provided connection is closed
// and return an error. This consumes the connection.
func (c *Client) WaitForDisconnect() error {
	n, err := io.Copy(ioutil.Discard, c.conn)
	if err != nil && err != io.EOF {
		return err
	}
	if n > 0 {
		return fmt.Errorf("unexpected bytes read waiting for disconnect: %d", n)
	}
	return nil
}

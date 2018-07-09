// Package haproxy provides a minimal client for communicating with, and issuing commands to, HAproxy over a network or file socket.
package haproxy

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

const (
	socketSchema = "unix://"
	tcpSchema    = "tcp://"
)

// HAProxyClient is the main structure of the library.
type HAProxyClient struct {
	Addr    string
	Timeout int
	conn    net.Conn
}

// RunCommand is the entrypoint to the client. Sends an arbitray command string to HAProxy.
func (h *HAProxyClient) RunCommand(cmd string) (*bytes.Buffer, error) {
	err := h.dial()
	if err != nil {
		return nil, err
	}
	defer h.conn.Close()

	result := bytes.NewBuffer(nil)

	_, err = h.conn.Write([]byte(cmd + "\n"))
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(result, h.conn)
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(result.String(), "Unknown command") {
		return nil, fmt.Errorf("Unknown command: %s", cmd)
	}

	return result, nil
}

func (h *HAProxyClient) dial() (err error) {
	if h.Timeout == 0 {
		h.Timeout = 30
	}

	timeout := time.Duration(h.Timeout) * time.Second

	switch h.schema() {
	case "socket":
		h.conn, err = net.DialTimeout("unix", strings.Replace(h.Addr, socketSchema, "", 1), timeout)
	case "tcp":
		h.conn, err = net.DialTimeout("tcp", strings.Replace(h.Addr, tcpSchema, "", 1), timeout)
	default:
		return fmt.Errorf("unknown schema")
	}
	return err
}

func (h *HAProxyClient) schema() string {
	if strings.HasPrefix(h.Addr, socketSchema) {
		return "socket"
	}
	if strings.HasPrefix(h.Addr, tcpSchema) {
		return "tcp"
	}
	return ""
}

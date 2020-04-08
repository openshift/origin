package ssh

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/armon/go-socks5"
	. "gopkg.in/check.v1"
)

type ProxySuite struct {
	UploadPackSuite
}

var _ = Suite(&ProxySuite{})

func (s *ProxySuite) SetUpSuite(c *C) {
	s.UploadPackSuite.SetUpSuite(c)

	l, err := net.Listen("tcp", "localhost:0")
	c.Assert(err, IsNil)

	server, err := socks5.New(&socks5.Config{})
	c.Assert(err, IsNil)

	port := l.Addr().(*net.TCPAddr).Port

	err = os.Setenv("ALL_PROXY", fmt.Sprintf("socks5://localhost:%d", port))
	c.Assert(err, IsNil)

	go func() {
		log.Fatal(server.Serve(l))
	}()
}

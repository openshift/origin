package transport

import (
	"fmt"
	"net/url"
	"testing"

	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp/capability"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type SuiteCommon struct{}

var _ = Suite(&SuiteCommon{})

func (s *SuiteCommon) TestNewEndpointHTTP(c *C) {
	e, err := NewEndpoint("http://git:pass@github.com/user/repository.git?foo#bar")
	c.Assert(err, IsNil)
	c.Assert(e.Protocol, Equals, "http")
	c.Assert(e.User, Equals, "git")
	c.Assert(e.Password, Equals, "pass")
	c.Assert(e.Host, Equals, "github.com")
	c.Assert(e.Port, Equals, 0)
	c.Assert(e.Path, Equals, "/user/repository.git?foo#bar")
	c.Assert(e.String(), Equals, "http://git:pass@github.com/user/repository.git?foo#bar")
}

func (s *SuiteCommon) TestNewEndpointPorts(c *C) {
	e, err := NewEndpoint("http://git:pass@github.com:8080/user/repository.git?foo#bar")
	c.Assert(err, IsNil)
	c.Assert(e.String(), Equals, "http://git:pass@github.com:8080/user/repository.git?foo#bar")

	e, err = NewEndpoint("https://git:pass@github.com:443/user/repository.git?foo#bar")
	c.Assert(err, IsNil)
	c.Assert(e.String(), Equals, "https://git:pass@github.com/user/repository.git?foo#bar")

	e, err = NewEndpoint("ssh://git:pass@github.com:22/user/repository.git?foo#bar")
	c.Assert(err, IsNil)
	c.Assert(e.String(), Equals, "ssh://git:pass@github.com/user/repository.git?foo#bar")

	e, err = NewEndpoint("git://github.com:9418/user/repository.git?foo#bar")
	c.Assert(err, IsNil)
	c.Assert(e.String(), Equals, "git://github.com/user/repository.git?foo#bar")

}

func (s *SuiteCommon) TestNewEndpointSSH(c *C) {
	e, err := NewEndpoint("ssh://git@github.com/user/repository.git")
	c.Assert(err, IsNil)
	c.Assert(e.Protocol, Equals, "ssh")
	c.Assert(e.User, Equals, "git")
	c.Assert(e.Password, Equals, "")
	c.Assert(e.Host, Equals, "github.com")
	c.Assert(e.Port, Equals, 0)
	c.Assert(e.Path, Equals, "/user/repository.git")
	c.Assert(e.String(), Equals, "ssh://git@github.com/user/repository.git")
}

func (s *SuiteCommon) TestNewEndpointSSHNoUser(c *C) {
	e, err := NewEndpoint("ssh://github.com/user/repository.git")
	c.Assert(err, IsNil)
	c.Assert(e.Protocol, Equals, "ssh")
	c.Assert(e.User, Equals, "")
	c.Assert(e.Password, Equals, "")
	c.Assert(e.Host, Equals, "github.com")
	c.Assert(e.Port, Equals, 0)
	c.Assert(e.Path, Equals, "/user/repository.git")
	c.Assert(e.String(), Equals, "ssh://github.com/user/repository.git")
}

func (s *SuiteCommon) TestNewEndpointSSHWithPort(c *C) {
	e, err := NewEndpoint("ssh://git@github.com:777/user/repository.git")
	c.Assert(err, IsNil)
	c.Assert(e.Protocol, Equals, "ssh")
	c.Assert(e.User, Equals, "git")
	c.Assert(e.Password, Equals, "")
	c.Assert(e.Host, Equals, "github.com")
	c.Assert(e.Port, Equals, 777)
	c.Assert(e.Path, Equals, "/user/repository.git")
	c.Assert(e.String(), Equals, "ssh://git@github.com:777/user/repository.git")
}

func (s *SuiteCommon) TestNewEndpointSCPLike(c *C) {
	e, err := NewEndpoint("git@github.com:user/repository.git")
	c.Assert(err, IsNil)
	c.Assert(e.Protocol, Equals, "ssh")
	c.Assert(e.User, Equals, "git")
	c.Assert(e.Password, Equals, "")
	c.Assert(e.Host, Equals, "github.com")
	c.Assert(e.Port, Equals, 22)
	c.Assert(e.Path, Equals, "user/repository.git")
	c.Assert(e.String(), Equals, "ssh://git@github.com/user/repository.git")
}

func (s *SuiteCommon) TestNewEndpointSCPLikeWithPort(c *C) {
	e, err := NewEndpoint("git@github.com:9999/user/repository.git")
	c.Assert(err, IsNil)
	c.Assert(e.Protocol, Equals, "ssh")
	c.Assert(e.User, Equals, "git")
	c.Assert(e.Password, Equals, "")
	c.Assert(e.Host, Equals, "github.com")
	c.Assert(e.Port, Equals, 9999)
	c.Assert(e.Path, Equals, "user/repository.git")
	c.Assert(e.String(), Equals, "ssh://git@github.com:9999/user/repository.git")
}

func (s *SuiteCommon) TestNewEndpointFileAbs(c *C) {
	e, err := NewEndpoint("/foo.git")
	c.Assert(err, IsNil)
	c.Assert(e.Protocol, Equals, "file")
	c.Assert(e.User, Equals, "")
	c.Assert(e.Password, Equals, "")
	c.Assert(e.Host, Equals, "")
	c.Assert(e.Port, Equals, 0)
	c.Assert(e.Path, Equals, "/foo.git")
	c.Assert(e.String(), Equals, "file:///foo.git")
}

func (s *SuiteCommon) TestNewEndpointFileRel(c *C) {
	e, err := NewEndpoint("foo.git")
	c.Assert(err, IsNil)
	c.Assert(e.Protocol, Equals, "file")
	c.Assert(e.User, Equals, "")
	c.Assert(e.Password, Equals, "")
	c.Assert(e.Host, Equals, "")
	c.Assert(e.Port, Equals, 0)
	c.Assert(e.Path, Equals, "foo.git")
	c.Assert(e.String(), Equals, "file://foo.git")
}

func (s *SuiteCommon) TestNewEndpointFileWindows(c *C) {
	e, err := NewEndpoint("C:\\foo.git")
	c.Assert(err, IsNil)
	c.Assert(e.Protocol, Equals, "file")
	c.Assert(e.User, Equals, "")
	c.Assert(e.Password, Equals, "")
	c.Assert(e.Host, Equals, "")
	c.Assert(e.Port, Equals, 0)
	c.Assert(e.Path, Equals, "C:\\foo.git")
	c.Assert(e.String(), Equals, "file://C:\\foo.git")
}

func (s *SuiteCommon) TestNewEndpointFileURL(c *C) {
	e, err := NewEndpoint("file:///foo.git")
	c.Assert(err, IsNil)
	c.Assert(e.Protocol, Equals, "file")
	c.Assert(e.User, Equals, "")
	c.Assert(e.Password, Equals, "")
	c.Assert(e.Host, Equals, "")
	c.Assert(e.Port, Equals, 0)
	c.Assert(e.Path, Equals, "/foo.git")
	c.Assert(e.String(), Equals, "file:///foo.git")
}

func (s *SuiteCommon) TestValidEndpoint(c *C) {
	user := "person@mail.com"
	pass := " !\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"
	e, err := NewEndpoint(fmt.Sprintf(
		"http://%s:%s@github.com/user/repository.git",
		url.PathEscape(user),
		url.PathEscape(pass),
	))
	c.Assert(err, IsNil)
	c.Assert(e, NotNil)
	c.Assert(e.User, Equals, user)
	c.Assert(e.Password, Equals, pass)
	c.Assert(e.Host, Equals, "github.com")
	c.Assert(e.Path, Equals, "/user/repository.git")

	c.Assert(e.String(), Equals, "http://person@mail.com:%20%21%22%23$%25&%27%28%29%2A+%2C-.%2F:%3B%3C=%3E%3F@%5B%5C%5D%5E_%60%7B%7C%7D~@github.com/user/repository.git")
}

func (s *SuiteCommon) TestNewEndpointInvalidURL(c *C) {
	e, err := NewEndpoint("http://\\")
	c.Assert(err, NotNil)
	c.Assert(e, IsNil)
}

func (s *SuiteCommon) TestFilterUnsupportedCapabilities(c *C) {
	l := capability.NewList()
	l.Set(capability.MultiACK)

	FilterUnsupportedCapabilities(l)
	c.Assert(l.Supports(capability.MultiACK), Equals, false)
}

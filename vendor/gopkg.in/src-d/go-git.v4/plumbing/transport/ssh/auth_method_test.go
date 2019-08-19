package ssh

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/testdata"

	. "gopkg.in/check.v1"
)

type (
	SuiteCommon struct{}

	mockKnownHosts struct{}
)

func (mockKnownHosts) host() string { return "github.com" }
func (mockKnownHosts) knownHosts() []byte {
	return []byte(`github.com ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAq2A7hRGmdnm9tUDbO9IDSwBK6TbQa+PXYPCPy6rbTrTtw7PHkccKrpp0yVhp5HdEIcKr6pLlVDBfOLX9QUsyCOV0wzfjIJNlGEYsdlLJizHhbn2mUjvSAHQqZETYP81eFzLQNnPHt4EVVUh7VfDESU84KezmD5QlWpXLmvU31/yMf+Se8xhHTvKSCZIFImWwoG6mbUoWf9nzpIoaSjB+weqqUUmpaaasXVal72J+UX2B+2RPW3RcT0eOzQgqlJL3RKrTJvdsjE3JEAvGq3lGHSZXy28G3skua2SmVi/w4yCE6gbODqnTWlg7+wC604ydGXA8VJiS5ap43JXiUFFAaQ==`)
}
func (mockKnownHosts) Network() string { return "tcp" }
func (mockKnownHosts) String() string  { return "github.com:22" }

var _ = Suite(&SuiteCommon{})

func (s *SuiteCommon) TestKeyboardInteractiveName(c *C) {
	a := &KeyboardInteractive{
		User:      "test",
		Challenge: nil,
	}
	c.Assert(a.Name(), Equals, KeyboardInteractiveName)
}

func (s *SuiteCommon) TestKeyboardInteractiveString(c *C) {
	a := &KeyboardInteractive{
		User:      "test",
		Challenge: nil,
	}
	c.Assert(a.String(), Equals, fmt.Sprintf("user: test, name: %s", KeyboardInteractiveName))
}

func (s *SuiteCommon) TestPasswordName(c *C) {
	a := &Password{
		User:     "test",
		Password: "",
	}
	c.Assert(a.Name(), Equals, PasswordName)
}

func (s *SuiteCommon) TestPasswordString(c *C) {
	a := &Password{
		User:     "test",
		Password: "",
	}
	c.Assert(a.String(), Equals, fmt.Sprintf("user: test, name: %s", PasswordName))
}

func (s *SuiteCommon) TestPasswordCallbackName(c *C) {
	a := &PasswordCallback{
		User:     "test",
		Callback: nil,
	}
	c.Assert(a.Name(), Equals, PasswordCallbackName)
}

func (s *SuiteCommon) TestPasswordCallbackString(c *C) {
	a := &PasswordCallback{
		User:     "test",
		Callback: nil,
	}
	c.Assert(a.String(), Equals, fmt.Sprintf("user: test, name: %s", PasswordCallbackName))
}

func (s *SuiteCommon) TestPublicKeysName(c *C) {
	a := &PublicKeys{
		User:   "test",
		Signer: nil,
	}
	c.Assert(a.Name(), Equals, PublicKeysName)
}

func (s *SuiteCommon) TestPublicKeysString(c *C) {
	a := &PublicKeys{
		User:   "test",
		Signer: nil,
	}
	c.Assert(a.String(), Equals, fmt.Sprintf("user: test, name: %s", PublicKeysName))
}

func (s *SuiteCommon) TestPublicKeysCallbackName(c *C) {
	a := &PublicKeysCallback{
		User:     "test",
		Callback: nil,
	}
	c.Assert(a.Name(), Equals, PublicKeysCallbackName)
}

func (s *SuiteCommon) TestPublicKeysCallbackString(c *C) {
	a := &PublicKeysCallback{
		User:     "test",
		Callback: nil,
	}
	c.Assert(a.String(), Equals, fmt.Sprintf("user: test, name: %s", PublicKeysCallbackName))
}
func (s *SuiteCommon) TestNewSSHAgentAuth(c *C) {
	if os.Getenv("SSH_AUTH_SOCK") == "" {
		c.Skip("SSH_AUTH_SOCK or SSH_TEST_PRIVATE_KEY are required")
	}

	auth, err := NewSSHAgentAuth("foo")
	c.Assert(err, IsNil)
	c.Assert(auth, NotNil)
}

func (s *SuiteCommon) TestNewSSHAgentAuthNoAgent(c *C) {
	addr := os.Getenv("SSH_AUTH_SOCK")
	err := os.Unsetenv("SSH_AUTH_SOCK")
	c.Assert(err, IsNil)

	defer func() {
		err := os.Setenv("SSH_AUTH_SOCK", addr)
		c.Assert(err, IsNil)
	}()

	k, err := NewSSHAgentAuth("foo")
	c.Assert(k, IsNil)
	c.Assert(err, ErrorMatches, ".*SSH_AUTH_SOCK.*|.*SSH agent .* not running.*")
}

func (*SuiteCommon) TestNewPublicKeys(c *C) {
	auth, err := NewPublicKeys("foo", testdata.PEMBytes["rsa"], "")
	c.Assert(err, IsNil)
	c.Assert(auth, NotNil)
}

func (*SuiteCommon) TestNewPublicKeysWithEncryptedPEM(c *C) {
	f := testdata.PEMEncryptedKeys[0]
	auth, err := NewPublicKeys("foo", f.PEMBytes, f.EncryptionKey)
	c.Assert(err, IsNil)
	c.Assert(auth, NotNil)
}

func (*SuiteCommon) TestNewPublicKeysFromFile(c *C) {
	f, err := ioutil.TempFile("", "ssh-test")
	c.Assert(err, IsNil)
	_, err = f.Write(testdata.PEMBytes["rsa"])
	c.Assert(err, IsNil)
	c.Assert(f.Close(), IsNil)
	defer os.RemoveAll(f.Name())

	auth, err := NewPublicKeysFromFile("foo", f.Name(), "")
	c.Assert(err, IsNil)
	c.Assert(auth, NotNil)
}

func (*SuiteCommon) TestNewPublicKeysWithInvalidPEM(c *C) {
	auth, err := NewPublicKeys("foo", []byte("bar"), "")
	c.Assert(err, NotNil)
	c.Assert(auth, IsNil)
}

func (*SuiteCommon) TestNewKnownHostsCallback(c *C) {
	var mock = mockKnownHosts{}

	f, err := ioutil.TempFile("", "known-hosts")
	c.Assert(err, IsNil)

	_, err = f.Write(mock.knownHosts())
	c.Assert(err, IsNil)

	err = f.Close()
	c.Assert(err, IsNil)

	defer os.RemoveAll(f.Name())

	f, err = os.Open(f.Name())
	c.Assert(err, IsNil)

	defer f.Close()

	var hostKey ssh.PublicKey
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), " ")
		if len(fields) != 3 {
			continue
		}
		if strings.Contains(fields[0], mock.host()) {
			var err error
			hostKey, _, _, _, err = ssh.ParseAuthorizedKey(scanner.Bytes())
			if err != nil {
				c.Fatalf("error parsing %q: %v", fields[2], err)
			}
			break
		}
	}
	if hostKey == nil {
		c.Fatalf("no hostkey for %s", mock.host())
	}

	clb, err := NewKnownHostsCallback(f.Name())
	c.Assert(err, IsNil)

	err = clb(mock.String(), mock, hostKey)
	c.Assert(err, IsNil)
}

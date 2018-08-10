package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/mndrix/tap-go"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"github.com/syndtr/gocapability/capability"
	"github.com/urfave/cli"

	"github.com/opencontainers/runtime-tools/cmd/runtimetest/mount"
	rfc2119 "github.com/opencontainers/runtime-tools/error"
	"github.com/opencontainers/runtime-tools/specerror"

	"golang.org/x/sys/unix"
)

// gitCommit will be the hash that the binary was built from
// and will be populated by the Makefile
var gitCommit = ""

// version will be populated by the Makefile, read from
// VERSION file of the source code.
var version = ""

// errAccess will be used for defining either a read error or a write error
// from any type of files.
var errAccess error

// PrGetNoNewPrivs isn't exposed in Golang so we define it ourselves copying the value from
// the kernel
const PrGetNoNewPrivs = 39

const specConfig = "config.json"

var (
	defaultFS = map[string]string{
		"/proc":    "proc",
		"/sys":     "sysfs",
		"/dev/pts": "devpts",
		"/dev/shm": "tmpfs",
	}

	// NOTE: check if /dev/ptmx is a symlink to /dev/pts/ptmx.
	// os.Readlink() returns only a relative path "pts/ptmx" instead of
	// an absolute path /dev/pts/ptmx.
	defaultSymlinks = map[string]string{
		"/dev/fd":     "/proc/self/fd",
		"/dev/ptmx":   "pts/ptmx",
		"/dev/stdin":  "/proc/self/fd/0",
		"/dev/stdout": "/proc/self/fd/1",
		"/dev/stderr": "/proc/self/fd/2",
	}

	defaultDevices = []rspec.LinuxDevice{
		{
			Path:  "/dev/null",
			Type:  "c",
			Major: 1,
			Minor: 3,
		},
		{
			Path:  "/dev/zero",
			Type:  "c",
			Major: 1,
			Minor: 5,
		},
		{
			Path:  "/dev/full",
			Type:  "c",
			Major: 1,
			Minor: 7,
		},
		{
			Path:  "/dev/random",
			Type:  "c",
			Major: 1,
			Minor: 8,
		},
		{
			Path:  "/dev/urandom",
			Type:  "c",
			Major: 1,
			Minor: 9,
		},
		{
			Path:  "/dev/tty",
			Type:  "c",
			Major: 5,
			Minor: 0,
		},
		{
			Path:  "/dev/ptmx",
			Type:  "c",
			Major: 5,
			Minor: 2,
		},
	}
)

type complianceTester struct {
	harness         *tap.T
	complianceLevel rfc2119.Level
}

func (c *complianceTester) Ok(test bool, condition specerror.Code, version string, description string) (rfcError *rfc2119.Error, err error) {
	rfcError, err = specerror.NewRFCError(condition, errors.New(description), version)
	if err != nil {
		return nil, err
	}
	if test {
		c.harness.Pass(description)
	} else if rfcError.Level < c.complianceLevel {
		c.harness.Skip(1, description)
	} else {
		c.harness.Fail(description)
	}
	return rfcError, nil
}

type validator func(config *rspec.Spec) (err error)

func loadSpecConfig(path string) (spec *rspec.Spec, err error) {
	configPath := filepath.Join(path, specConfig)
	cf, err := os.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, specerror.NewError(specerror.ConfigInRootBundleDir, err, rspec.Version)
		}

		return nil, err
	}
	defer cf.Close()

	if err = json.NewDecoder(cf).Decode(&spec); err != nil {
		return
	}
	return spec, nil
}

func (c *complianceTester) validatePosixUser(spec *rspec.Spec) error {
	if spec.Process == nil {
		return nil
	}

	uid := uint32(os.Getuid())
	c.harness.Ok(uid == spec.Process.User.UID, "has expected user ID")
	c.harness.YAML(map[string]uint32{
		"expected": spec.Process.User.UID,
		"actual":   uid,
	})

	gid := uint32(os.Getgid())
	c.harness.Ok(gid == spec.Process.User.GID, "has expected group ID")
	c.harness.YAML(map[string]uint32{
		"expected": spec.Process.User.GID,
		"actual":   gid,
	})

	groups, err := os.Getgroups()
	if err != nil {
		return err
	}

	groupsMap := make(map[int]bool)
	for _, g := range groups {
		groupsMap[g] = true
	}

	for _, g := range spec.Process.User.AdditionalGids {
		c.harness.Ok(groupsMap[int(g)], fmt.Sprintf("has expected additional group ID %v", g))
	}

	return nil
}

func (c *complianceTester) validateProcess(spec *rspec.Spec) error {
	if spec.Process == nil {
		c.harness.Skip(1, "process not set")
		return nil
	}

	if spec.Process.Cwd == "" {
		c.harness.Skip(1, "process.cwd not set")
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		c.harness.Ok(cwd == spec.Process.Cwd, "has expected working directory")
		c.harness.YAML(map[string]string{
			"expected": spec.Process.Cwd,
			"actual":   cwd,
		})
	}

	for _, env := range spec.Process.Env {
		parts := strings.Split(env, "=")
		key := parts[0]
		expectedValue := parts[1]
		actualValue := os.Getenv(key)
		c.harness.Ok(expectedValue == actualValue, fmt.Sprintf("has expected environment variable %v", key))
		c.harness.YAML(map[string]string{
			"variable": key,
			"expected": expectedValue,
			"actual":   actualValue,
		})
	}

	return nil
}

func (c *complianceTester) validateLinuxProcess(spec *rspec.Spec) error {
	if spec.Process == nil {
		c.harness.Skip(1, "process not set")
		return nil
	}

	cmdlineBytes, err := ioutil.ReadFile("/proc/self/cmdline")
	if err != nil {
		return err
	}

	args := bytes.Split(bytes.Trim(cmdlineBytes, "\x00"), []byte("\x00"))
	c.harness.Ok(len(args) == len(spec.Process.Args), "has expected number of process arguments")
	c.harness.YAML(map[string]interface{}{
		"expected": spec.Process.Args,
		"actual":   args,
	})
	for i, a := range args {
		c.harness.Ok(string(a) == spec.Process.Args[i], fmt.Sprintf("has expected process argument %d", i))
		c.harness.YAML(map[string]interface{}{
			"index":    i,
			"expected": spec.Process.Args[i],
			"actual":   string(a),
		})
	}

	ret, _, errno := syscall.Syscall6(syscall.SYS_PRCTL, PrGetNoNewPrivs, 0, 0, 0, 0, 0)
	if errno != 0 {
		return errno
	}
	noNewPrivileges := ret == 1
	c.harness.Ok(spec.Process.NoNewPrivileges == noNewPrivileges, "has expected noNewPrivileges")

	return nil
}

func (c *complianceTester) validateCapabilities(spec *rspec.Spec) error {
	if spec.Process == nil || spec.Process.Capabilities == nil {
		c.harness.Skip(1, "process.capabilities not set")
		return nil
	}

	last := capability.CAP_LAST_CAP
	// workaround for RHEL6 which has no /proc/sys/kernel/cap_last_cap
	if last == capability.Cap(63) {
		last = capability.CAP_BLOCK_SUSPEND
	}

	processCaps, err := capability.NewPid(0)
	if err != nil {
		return err
	}

	for _, capType := range []struct {
		capType capability.CapType
		config  []string
	}{
		{
			capType: capability.BOUNDING,
			config:  spec.Process.Capabilities.Bounding,
		},
		{
			capType: capability.EFFECTIVE,
			config:  spec.Process.Capabilities.Effective,
		},
		{
			capType: capability.INHERITABLE,
			config:  spec.Process.Capabilities.Inheritable,
		},
		{
			capType: capability.PERMITTED,
			config:  spec.Process.Capabilities.Permitted,
		},
		{
			capType: capability.AMBIENT,
			config:  spec.Process.Capabilities.Ambient,
		},
	} {
		expectedCaps := make(map[string]bool)
		for _, ec := range capType.config {
			expectedCaps[ec] = true
		}

		for _, cap := range capability.List() {
			if cap > last {
				continue
			}

			capKey := fmt.Sprintf("CAP_%s", strings.ToUpper(cap.String()))
			expectedSet := expectedCaps[capKey]
			actuallySet := processCaps.Get(capType.capType, cap)
			if expectedSet {
				c.harness.Ok(actuallySet, fmt.Sprintf("expected %s capability %v set", capType.capType, capKey))
			} else {
				c.harness.Ok(!actuallySet, fmt.Sprintf("unexpected %s capability %v not set", capType.capType, capKey))
			}
		}
	}

	return nil
}

func (c *complianceTester) validateHostname(spec *rspec.Spec) error {
	if spec.Hostname == "" {
		c.harness.Skip(1, "hostname not set")
		return nil
	}

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	c.harness.Ok(spec.Hostname == hostname, "has expected hostname")
	c.harness.YAML(map[string]string{
		"expected": spec.Hostname,
		"actual":   hostname,
	})
	return nil
}

func (c *complianceTester) validateRlimits(spec *rspec.Spec) error {
	if spec.Process == nil {
		c.harness.Skip(1, "process.rlimits not set")
		return nil
	}

	for _, r := range spec.Process.Rlimits {
		rl, err := strToRlimit(r.Type)
		if err != nil {
			return err
		}

		var rlimit syscall.Rlimit
		if err := syscall.Getrlimit(rl, &rlimit); err != nil {
			return err
		}

		rfcError, err := c.Ok(rlimit.Cur == r.Soft, specerror.PosixProcRlimitsSoftMatchCur, spec.Version, fmt.Sprintf("has expected soft %v", r.Type))
		if err != nil {
			return err
		}
		c.harness.YAML(map[string]interface{}{
			"level":     rfcError.Level.String(),
			"reference": rfcError.Reference,
			"type":      r.Type,
			"expected":  r.Soft,
			"actual":    rlimit.Cur,
		})

		rfcError, err = c.Ok(rlimit.Max == r.Hard, specerror.PosixProcRlimitsHardMatchMax, spec.Version, fmt.Sprintf("has expected hard %v", r.Type))
		if err != nil {
			return err
		}
		c.harness.YAML(map[string]interface{}{
			"level":     rfcError.Level.String(),
			"reference": rfcError.Reference,
			"type":      r.Type,
			"expected":  r.Hard,
			"actual":    rlimit.Max,
		})
	}
	return nil
}

func (c *complianceTester) validateSysctls(spec *rspec.Spec) error {
	if spec.Linux == nil || spec.Linux.Sysctl == nil {
		c.harness.Skip(1, "linux.sysctl not set")
		return nil
	}

	for k, v := range spec.Linux.Sysctl {
		keyPath := filepath.Join("/proc/sys", strings.Replace(k, ".", "/", -1))
		vBytes, err := ioutil.ReadFile(keyPath)
		if err != nil {
			return err
		}
		value := strings.TrimSpace(string(bytes.Trim(vBytes, "\x00")))
		c.harness.Ok(value == v, fmt.Sprintf("has expected sysctl %v", k))
		c.harness.YAML(map[string]string{
			"sysctl":   k,
			"expected": v,
			"actual":   value,
		})
	}
	return nil
}

func testReadAccess(path string) (readable bool, err error) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	// Check for readability in case of regular files, character device, or
	// directory. Although the runtime spec does not mandate the type of
	// masked files, we should check its Mode explicitly. A masked file
	// could be represented as a character file (/dev/null), which is the
	// case for runtimes like runc.
	switch fi.Mode() & os.ModeType {
	case 0, os.ModeDevice | os.ModeCharDevice:
		return testFileReadAccess(path)
	case os.ModeDir:
		return testDirectoryReadAccess(path)
	}

	errAccess = fmt.Errorf("cannot test read access for %q (mode %d)", path, fi.Mode())
	return false, errAccess
}

func testDirectoryReadAccess(path string) (readable bool, err error) {
	files, err := ioutil.ReadDir(path)
	if err == io.EOF || len(files) == 0 {
		// Our validation/ tests only use non-empty directories for read-access
		// tests. So if we get an EOF on the first read, the runtime did
		// successfully block readability. So it should not be considered as test
		// failure, it just means that the test program successfully assessed
		// that the directory is not readable.
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func testFileReadAccess(path string) (readable bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return false, nil
	}
	defer f.Close()
	b := make([]byte, 1)
	_, err = f.Read(b)
	if err == nil {
		return true, nil
	} else if err == io.EOF {
		// Our validation/ tests only use non-empty files for read-access
		// tests. So if we get an EOF on the first read, the runtime did
		// successfully block readability.
		return false, nil
	}
	return false, err
}

func testWriteAccess(path string) (writable bool, err error) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	// Check for writability in case of regular files, character device, or
	// directory. Although the runtime spec does not mandate the type of
	// masked files, we should check its Mode explicitly. A masked file
	// could be represented as a character file (/dev/null), which is the
	// case for runtimes like runc.
	switch fi.Mode() & os.ModeType {
	case 0, os.ModeDevice | os.ModeCharDevice:
		return testFileWriteAccess(path)
	case os.ModeDir:
		return testDirectoryWriteAccess(path)
	}
	errAccess = fmt.Errorf("cannot test write access for %q (mode %d)", path, fi.Mode())
	return false, errAccess
}

func testDirectoryWriteAccess(path string) (writable bool, err error) {
	tmpfile, err := ioutil.TempFile(path, "Test")
	if err != nil {
		return false, nil
	}
	tmpfile.Close()
	return true, os.RemoveAll(filepath.Join(path, tmpfile.Name()))
}

func testFileWriteAccess(path string) (readable bool, err error) {
	err = ioutil.WriteFile(path, []byte("a"), 0644)
	if err == nil {
		return true, nil
	}
	return false, nil
}

func (c *complianceTester) validateRootFS(spec *rspec.Spec) error {
	if spec.Root == nil {
		c.harness.Skip(1, "root not set")
		return nil
	}

	writable, err := testDirectoryWriteAccess("/")
	if err != nil {
		return err
	}

	if spec.Root.Readonly {
		rfcError, err := c.Ok(!writable, specerror.RootReadonlyImplement, spec.Version, "root filesystem is readonly")
		if err != nil {
			return err
		}
		c.harness.YAML(map[string]string{
			"level":     rfcError.Level.String(),
			"reference": rfcError.Reference,
		})
	} else if !writable {
		c.harness.Skip(1, "root.readonly is false but the root filesystem is still not writable")
	}

	return nil
}

func (c *complianceTester) validateRootfsPropagation(spec *rspec.Spec) error {
	if spec.Linux == nil || spec.Linux.RootfsPropagation == "" {
		c.harness.Skip(1, "linux.rootfsPropagation not set")
		return nil
	}

	targetDir, err := ioutil.TempDir("/", "target")
	if err != nil {
		return err
	}
	defer os.RemoveAll(targetDir)

	switch spec.Linux.RootfsPropagation {
	case "shared", "slave", "private":
		mountDir, err := ioutil.TempDir("/", "mount")
		if err != nil {
			return err
		}
		defer os.RemoveAll(mountDir)

		testDir, err := ioutil.TempDir("/", "test")
		if err != nil {
			return err
		}
		defer os.RemoveAll(testDir)

		tmpfile, err := ioutil.TempFile(testDir, "example")
		if err != nil {
			return err
		}
		defer os.Remove(tmpfile.Name())

		if err := unix.Mount("/", targetDir, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
			return err
		}
		defer unix.Unmount(targetDir, unix.MNT_DETACH)
		if err := unix.Mount(testDir, mountDir, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
			return err
		}
		defer unix.Unmount(mountDir, unix.MNT_DETACH)
		targetFile := filepath.Join(targetDir, filepath.Join(mountDir, filepath.Base(tmpfile.Name())))
		var exposed bool
		_, err = os.Stat(targetFile)
		if os.IsNotExist(err) {
			exposed = false
		} else if err != nil {
			return err
		} else {
			exposed = true
		}
		if spec.Linux.RootfsPropagation == "shared" {
			c.harness.Ok(exposed, fmt.Sprintf("shared root propagation exposes %q", targetFile))
		} else {
			c.harness.Ok(
				!exposed,
				fmt.Sprintf("%s root propagation does not expose %q", spec.Linux.RootfsPropagation, targetFile),
			)
		}
	case "unbindable":
		err = unix.Mount("/", targetDir, "", unix.MS_BIND|unix.MS_REC, "")
		if err == syscall.EINVAL {
			c.harness.Pass("root propagation is unbindable")
			return nil
		} else if err != nil {
			return err
		}
		defer unix.Unmount(targetDir, unix.MNT_DETACH)
		c.harness.Fail("root propagation is unbindable")
		return nil
	default:
		c.harness.Skip(1, fmt.Sprintf("unrecognized linux.rootfsPropagation %s", spec.Linux.RootfsPropagation))
	}

	return nil
}

func (c *complianceTester) validateDefaultFS(spec *rspec.Spec) error {
	mountInfos, err := mount.GetMounts()
	if err != nil {
		return nil
	}

	mountsMap := make(map[string]string)
	for _, mountInfo := range mountInfos {
		mountsMap[mountInfo.Mountpoint] = mountInfo.Fstype
	}

	for fs, fstype := range defaultFS {
		rfcError, err := c.Ok(mountsMap[fs] == fstype, specerror.DefaultFilesystems, spec.Version, fmt.Sprintf("mount %v has expected type", fs))
		if err != nil {
			return err
		}
		c.harness.YAML(map[string]string{
			"level":     rfcError.Level.String(),
			"reference": rfcError.Reference,
			"mount":     fs,
			"expected":  fstype,
			"actual":    mountsMap[fs],
		})
	}

	return nil
}

func (c *complianceTester) validateLinuxDevices(spec *rspec.Spec) error {
	if spec.Linux == nil || spec.Linux.Devices == nil {
		c.harness.Skip(1, "linux.devices is not set")
		return nil
	}

	for i, device := range spec.Linux.Devices {
		err := c.validateDevice(
			&device,
			specerror.DevicesAvailable,
			spec.Version,
			fmt.Sprintf("%q (linux.devices[%d])", device.Path, i))
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *complianceTester) validateDevice(device *rspec.LinuxDevice, condition specerror.Code, version string, description string) (err error) {
	var exists bool
	fi, err := os.Stat(device.Path)
	if os.IsNotExist(err) {
		exists = false
	} else if err != nil {
		return err
	} else {
		exists = true
	}
	rfcError, err := c.Ok(exists, condition, version, fmt.Sprintf("has a file at %s", description))
	if err != nil {
		return err
	}
	c.harness.YAML(map[string]string{
		"level":     rfcError.Level.String(),
		"reference": rfcError.Reference,
		"path":      device.Path,
	})
	if !exists {
		return nil
	}

	fStat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("could not convert to syscall.Stat_t: %v", fi.Sys())
	}
	expectedType := device.Type
	if expectedType == "u" {
		expectedType = "c"
	}
	var devType string
	switch fStat.Mode & syscall.S_IFMT {
	case syscall.S_IFCHR:
		devType = "c"
	case syscall.S_IFBLK:
		devType = "b"
	case syscall.S_IFIFO:
		devType = "p"
	default:
		devType = "unmatched"
	}
	rfcError, err = c.Ok(devType == expectedType, condition, version, fmt.Sprintf("%s has the expected type", description))
	if err != nil {
		return err
	}
	c.harness.YAML(map[string]string{
		"level":     rfcError.Level.String(),
		"reference": rfcError.Reference,
		"path":      device.Path,
		"expected":  expectedType,
		"actual":    devType,
	})
	if devType != expectedType {
		return nil
	}

	if devType != "p" {
		dev := fStat.Rdev
		major := (dev >> 8) & 0xfff
		minor := (dev & 0xff) | ((dev >> 12) & 0xfff00)
		rfcError, err = c.Ok(int64(major) == device.Major, condition, version, fmt.Sprintf("%s has the expected major ID", description))
		if err != nil {
			return err
		}
		c.harness.YAML(map[string]interface{}{
			"level":     rfcError.Level.String(),
			"reference": rfcError.Reference,
			"path":      device.Path,
			"expected":  device.Major,
			"actual":    major,
		})
		rfcError, err = c.Ok(int64(minor) == device.Minor, condition, version, fmt.Sprintf("%s has the expected minor ID", description))
		if err != nil {
			return err
		}
		c.harness.YAML(map[string]interface{}{
			"level":     rfcError.Level.String(),
			"reference": rfcError.Reference,
			"path":      device.Path,
			"expected":  device.Minor,
			"actual":    minor,
		})
	}

	if device.FileMode == nil {
		c.harness.Skip(1, fmt.Sprintf("%s has unconfigured permissions", description))
	} else {
		expectedPerm := *device.FileMode & os.ModePerm
		actualPerm := fi.Mode() & os.ModePerm
		rfcError, err = c.Ok(actualPerm == expectedPerm, condition, version, fmt.Sprintf("%s has the expected permissions", description))
		if err != nil {
			return err
		}
		c.harness.YAML(map[string]interface{}{
			"level":     rfcError.Level.String(),
			"reference": rfcError.Reference,
			"path":      device.Path,
			"expected":  expectedPerm,
			"actual":    actualPerm,
		})
	}

	if description == "/dev/console (default device)" {
		c.harness.Todo().Fail("we need the major/minor from the controlling TTY")
		return nil
	}

	if device.UID == nil {
		c.harness.Skip(1, fmt.Sprintf("%s has an unconfigured user ID", description))
	} else {
		rfcError, err = c.Ok(fStat.Uid == *device.UID, condition, version, fmt.Sprintf("%s has the expected user ID", description))
		if err != nil {
			return err
		}
		c.harness.YAML(map[string]interface{}{
			"level":     rfcError.Level.String(),
			"reference": rfcError.Reference,
			"path":      device.Path,
			"expected":  *device.UID,
			"actual":    fStat.Uid,
		})
	}

	if device.GID == nil {
		c.harness.Skip(1, fmt.Sprintf("%s has an unconfigured group ID", description))
	} else {
		rfcError, err = c.Ok(fStat.Gid == *device.GID, condition, version, fmt.Sprintf("%s has the expected group ID", description))
		if err != nil {
			return err
		}
		c.harness.YAML(map[string]interface{}{
			"level":     rfcError.Level.String(),
			"reference": rfcError.Reference,
			"path":      device.Path,
			"expected":  *device.GID,
			"actual":    fStat.Gid,
		})
	}

	return nil
}

func (c *complianceTester) validateDefaultSymlinks(spec *rspec.Spec) error {
	for symlink, dest := range defaultSymlinks {
		var exists bool
		fi, err := os.Lstat(symlink)
		if os.IsNotExist(err) {
			exists = false
		} else if err != nil {
			return err
		} else {
			exists = true
		}
		rfcError, err := c.Ok(exists, specerror.DefaultRuntimeLinuxSymlinks, spec.Version, fmt.Sprintf("has a file at default symlink path %q", symlink))
		if err != nil {
			return err
		}
		c.harness.YAML(map[string]string{
			"level":     rfcError.Level.String(),
			"reference": rfcError.Reference,
			"path":      symlink,
		})
		if !exists {
			continue
		}

		isSymlink := fi.Mode()&os.ModeType == os.ModeSymlink
		rfcError, err = c.Ok(
			isSymlink,
			specerror.DefaultRuntimeLinuxSymlinks,
			spec.Version,
			fmt.Sprintf("file at default symlink path %q is a symlink", symlink))
		if err != nil {
			return err
		}
		c.harness.YAML(map[string]interface{}{
			"level":     rfcError.Level.String(),
			"reference": rfcError.Reference,
			"path":      symlink,
			"mode":      fi.Mode(),
		})
		if !isSymlink {
			continue
		}

		realDest, err := os.Readlink(symlink)
		if err != nil {
			return err
		}
		rfcError, err = c.Ok(
			realDest == dest,
			specerror.DefaultRuntimeLinuxSymlinks,
			spec.Version,
			fmt.Sprintf("symlink at default symlink path %q has the expected target", symlink))
		if err != nil {
			return err
		}
		c.harness.YAML(map[string]string{
			"level":     rfcError.Level.String(),
			"reference": rfcError.Reference,
			"path":      symlink,
			"expected":  dest,
			"actual":    realDest,
		})
	}

	return nil
}

func (c *complianceTester) validateDefaultDevices(spec *rspec.Spec) error {
	if spec.Process != nil && spec.Process.Terminal {
		defaultDevices = append(defaultDevices, rspec.LinuxDevice{
			Path: "/dev/console",
			Type: "c",
			// FIXME: get the major/minor from the controlling TTY
		})
	}

	for _, device := range defaultDevices {
		err := c.validateDevice(
			&device,
			specerror.DefaultDevices,
			spec.Version,
			fmt.Sprintf("%s (default device)", device.Path))
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *complianceTester) validateMaskedPaths(spec *rspec.Spec) error {
	if spec.Linux == nil || spec.Linux.MaskedPaths == nil {
		c.harness.Skip(1, "linux.maskedPaths not set")
		return nil
	}

	for _, maskedPath := range spec.Linux.MaskedPaths {
		readable, err := testReadAccess(maskedPath)
		if err != nil && !os.IsNotExist(err) && err != errAccess {
			return err
		}
		c.harness.Ok(!readable, fmt.Sprintf("cannot read masked path %q", maskedPath))
	}

	return nil
}

func (c *complianceTester) validateSeccomp(spec *rspec.Spec) error {
	if spec.Linux == nil || spec.Linux.Seccomp == nil {
		c.harness.Skip(1, "linux.seccomp not set")
		return nil
	}

	for _, sys := range spec.Linux.Seccomp.Syscalls {
		if sys.Action == "SCMP_ACT_ERRNO" {
			for _, name := range sys.Names {
				if name == "getcwd" {
					_, err := os.Getwd()
					if err == nil {
						c.harness.Skip(1, "getcwd did not return an error")
					}
				} else {
					c.harness.Skip(1, fmt.Sprintf("%s syscall returns errno", name))
				}
			}
		} else {
			c.harness.Skip(1, fmt.Sprintf("syscall action %s", sys.Action))
		}
	}

	return nil
}

func (c *complianceTester) validateROPaths(spec *rspec.Spec) error {
	if spec.Linux == nil || spec.Linux.ReadonlyPaths == nil {
		c.harness.Skip(1, "linux.readonlyPaths not set")
		return nil
	}

	for i, path := range spec.Linux.ReadonlyPaths {
		readable, err := testReadAccess(path)
		if err != nil && err != errAccess {
			return err
		}
		if !readable {
			c.harness.Skip(1, fmt.Sprintf("%q (linux.readonlyPaths[%d]) is not readable", path, i))
		}

		writable, err := testWriteAccess(path)
		if err != nil && !os.IsNotExist(err) && err != errAccess {
			return err
		}
		c.harness.Ok(!writable, fmt.Sprintf("%q (linux.readonlyPaths[%d]) is not writable", path, i))
	}

	return nil
}

func (c *complianceTester) validateOOMScoreAdj(spec *rspec.Spec) error {
	if spec.Process == nil || spec.Process.OOMScoreAdj == nil {
		c.harness.Skip(1, "process.oomScoreAdj not set")
		return nil
	}

	expected := *spec.Process.OOMScoreAdj
	f, err := os.Open("/proc/self/oom_score_adj")
	if err != nil {
		return err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		err := s.Err()
		if err != nil {
			return err
		}
		text := strings.TrimSpace(s.Text())
		actual, err := strconv.Atoi(text)
		if err != nil {
			return err
		}
		rfcError, err := c.Ok(actual == expected, specerror.LinuxProcOomScoreAdjSet, spec.Version, fmt.Sprintf("has expected OOM score adjustment"))
		if err != nil {
			return err
		}
		c.harness.YAML(map[string]interface{}{
			"level":     rfcError.Level.String(),
			"reference": rfcError.Reference,
			"expected":  expected,
			"actual":    actual,
		})
	}

	return nil
}

func getIDMappings(path string) ([]rspec.LinuxIDMapping, error) {
	var idMaps []rspec.LinuxIDMapping
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		if err := s.Err(); err != nil {
			return nil, err
		}

		idMap := strings.Fields(strings.TrimSpace(s.Text()))
		if len(idMap) == 3 {
			// "man 7 user_namespaces" explains the format of uid_map and gid_map:
			// <containerID> <hostID> <mapSize>
			containerID, err := strconv.ParseUint(idMap[0], 0, 32)
			if err != nil {
				return nil, err
			}
			hostID, err := strconv.ParseUint(idMap[1], 0, 32)
			if err != nil {
				return nil, err
			}
			mapSize, err := strconv.ParseUint(idMap[2], 0, 32)
			if err != nil {
				return nil, err
			}
			idMaps = append(idMaps, rspec.LinuxIDMapping{HostID: uint32(hostID), ContainerID: uint32(containerID), Size: uint32(mapSize)})
		} else {
			return nil, fmt.Errorf("invalid format in %v", path)
		}
	}

	return idMaps, nil
}

func (c *complianceTester) validateIDMappings(mappings []rspec.LinuxIDMapping, path string, property string) error {
	if len(mappings) == 0 {
		c.harness.Skip(1, fmt.Sprintf("%s not set", property))
		return nil
	}

	idMaps, err := getIDMappings(path)
	if err != nil {
		return err
	}
	c.harness.Ok(len(idMaps) == len(mappings), fmt.Sprintf("%s has expected number of mappings", path))
	c.harness.YAML(map[string]interface{}{
		"expected": mappings,
		"actual":   idMaps,
	})
	for _, v := range mappings {
		exist := false
		for _, cv := range idMaps {
			if v.HostID == cv.HostID && v.ContainerID == cv.ContainerID && v.Size == cv.Size {
				exist = true
				break
			}
		}
		c.harness.Ok(exist, fmt.Sprintf("%s has expected mapping %v", path, v))
	}

	return nil
}

func (c *complianceTester) validateUIDMappings(spec *rspec.Spec) error {
	if spec.Linux == nil || spec.Linux.UIDMappings == nil {
		c.harness.Skip(1, "linux.uidMappings not set")
		return nil
	}
	return c.validateIDMappings(spec.Linux.UIDMappings, "/proc/self/uid_map", "linux.uidMappings")
}

func (c *complianceTester) validateGIDMappings(spec *rspec.Spec) error {
	if spec.Linux == nil || spec.Linux.GIDMappings == nil {
		c.harness.Skip(1, "linux.gidMappings not set")
		return nil
	}
	return c.validateIDMappings(spec.Linux.GIDMappings, "/proc/self/gid_map", "linux.gidMappings")
}

func mountMatch(configMount rspec.Mount, sysMount *mount.Info) error {
	sys := rspec.Mount{
		Destination: sysMount.Mountpoint,
		Type:        sysMount.Fstype,
		Source:      sysMount.Source,
	}

	if filepath.Clean(configMount.Destination) != sys.Destination {
		return fmt.Errorf("mount destination expected: %v, actual: %v", configMount.Destination, sys.Destination)
	}

	isBind := false
	for _, opt := range configMount.Options {
		if opt == "bind" || opt == "rbind" {
			isBind = true
			break
		}
	}
	// Type is an optional field in the spec: only check if it is set
	if configMount.Type != "" && configMount.Type != sys.Type {
		return fmt.Errorf("mount %v type expected: %v, actual: %v", configMount.Destination, configMount.Type, sys.Type)
	}

	// For bind mounts, the source is not the block device but the path on the host that is being bind mounted.
	// sysMount.Root is that path.
	if isBind {
		// Source is an optional field in the spec: only check if it is set
		// We only test the base name here, in case the tests are being run in a chroot environment
		if configMount.Source != "" && filepath.Base(configMount.Source) != filepath.Base(sysMount.Root) {
			return fmt.Errorf("mount %v source expected: %v, actual: %v", configMount.Destination, filepath.Base(configMount.Source), filepath.Base(sysMount.Root))
		}
	} else {
		// Source is an optional field in the spec: only check if it is set
		if configMount.Source != "" && filepath.Clean(configMount.Source) != sys.Source {
			return fmt.Errorf("mount %v source expected: %v, actual: %v", configMount.Destination, configMount.Source, sys.Source)
		}
	}
	return nil
}

func (c *complianceTester) validatePosixMounts(spec *rspec.Spec) error {
	if spec.Mounts == nil {
		c.harness.Skip(1, "mounts not set")
		return nil
	}

	mountInfos, err := mount.GetMounts()
	if err != nil {
		return err
	}

	var mountErrs error
	var configSys = make(map[int]int)
	var consumedSys = make(map[int]bool)
	highestMatchedConfig := -1
	var j = 0
	for i, configMount := range spec.Mounts {
		if configMount.Type == "bind" || configMount.Type == "rbind" {
			c.harness.Todo().Fail("we need an (r)bind spec to test against")
			continue
		}

		foundInOrder := false
		foundOutOfOrder := false
		for k, sysMount := range mountInfos[j:] {
			if err := mountMatch(configMount, sysMount); err == nil {
				foundInOrder = true
				j += k + 1
				configSys[i] = j - 1
				consumedSys[j-1] = true
				if j > configSys[highestMatchedConfig] {
					highestMatchedConfig = i
				}
				break
			}
		}
		if err != nil {
			return err
		}
		if !foundInOrder {
			if j > 0 {
				for k, sysMount := range mountInfos[:j-1] {
					if _, ok := consumedSys[k]; ok {
						continue
					}
					if err := mountMatch(configMount, sysMount); err == nil {
						foundOutOfOrder = true
						break
					}
				}
			}
		}

		var rfcError *rfc2119.Error
		if !foundInOrder && !foundOutOfOrder {
			rfcError, err = c.Ok(false, specerror.MountsInOrder, spec.Version, fmt.Sprintf("mounts[%d] (%s) found", i, configMount.Destination))
		} else {
			rfcError, err = c.Ok(foundInOrder, specerror.MountsInOrder, spec.Version, fmt.Sprintf("mounts[%d] (%s) found in order", i, configMount.Destination))
			c.harness.YAML(map[string]interface{}{
				"level":       rfcError.Level.String(),
				"reference":   rfcError.Reference,
				"config":      configMount,
				"indexConfig": i,
				"indexSystem": configSys[i],
				"earlier": map[string]interface{}{
					"config":      spec.Mounts[highestMatchedConfig],
					"indexConfig": highestMatchedConfig,
					"indexSystem": configSys[highestMatchedConfig],
				},
			})
		}
	}

	return mountErrs
}

func run(context *cli.Context) error {
	logLevelString := context.String("log-level")
	logLevel, err := logrus.ParseLevel(logLevelString)
	if err != nil {
		return err
	}
	logrus.SetLevel(logLevel)

	platform := runtime.GOOS
	if platform != "linux" && platform != "solaris" && platform != "windows" {
		return fmt.Errorf("runtime-tools has not implemented testing for your platform %q, because the spec has nothing to say about it", platform)
	}

	inputPath := context.String("path")
	spec, err := loadSpecConfig(inputPath)
	if err != nil {
		return err
	}

	complianceLevelString := context.String("compliance-level")
	complianceLevel, err := rfc2119.ParseLevel(complianceLevelString)
	if err != nil {
		complianceLevel = rfc2119.Must
		logrus.Warningf("%s, using 'MUST' by default.", err.Error())
	}

	c := &complianceTester{
		harness:         tap.New(),
		complianceLevel: complianceLevel,
	}

	c.harness.Header(0)

	defaultValidations := []validator{
		c.validateRootFS,
		c.validateHostname,
		c.validateProcess,
	}

	posixValidations := []validator{
		c.validatePosixMounts,
		c.validatePosixUser,
		c.validateRlimits,
	}

	linuxValidations := []validator{
		c.validateCapabilities,
		c.validateDefaultSymlinks,
		c.validateDefaultFS,
		c.validateDefaultDevices,
		c.validateLinuxDevices,
		c.validateLinuxProcess,
		c.validateMaskedPaths,
		c.validateOOMScoreAdj,
		c.validateSeccomp,
		c.validateROPaths,
		c.validateRootfsPropagation,
		c.validateSysctls,
		c.validateUIDMappings,
		c.validateGIDMappings,
	}

	validations := defaultValidations
	if platform == "linux" {
		validations = append(validations, posixValidations...)
		validations = append(validations, linuxValidations...)
	} else if platform == "solaris" {
		validations = append(validations, posixValidations...)
	}

	for _, validation := range validations {
		err := validation(spec)
		if err != nil {
			return err
		}
	}
	c.harness.AutoPlan()

	return nil
}

func main() {
	app := cli.NewApp()
	app.Name = "runtimetest"
	if gitCommit != "" {
		app.Version = fmt.Sprintf("%s, commit: %s", version, gitCommit)
	} else {
		app.Version = version
	}
	app.Usage = "Compare the environment with an OCI configuration"
	app.Description = "runtimetest compares its current environment with an OCI runtime configuration read from config.json in its current working directory.  The tests are fairly generic and cover most configurations used by the runtime validation suite, but there are corner cases where a container launched by a valid runtime would not satisfy runtimetest."
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "log-level",
			Value: "error",
			Usage: "Log level (panic, fatal, error, warn, info, or debug)",
		},
		cli.StringFlag{
			Name:  "path",
			Value: ".",
			Usage: "Path to the configuration",
		},
		cli.StringFlag{
			Name:  "compliance-level",
			Value: "must",
			Usage: "Compliance level (may, should or must)",
		},
	}

	app.Action = run
	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

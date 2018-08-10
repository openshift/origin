package validate

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hashicorp/go-multierror"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"

	"github.com/opencontainers/runtime-tools/specerror"
)

func TestNewValidator(t *testing.T) {
	testSpec := &rspec.Spec{}
	testBundle := ""
	testPlatform := "not" + runtime.GOOS
	cases := []struct {
		val      Validator
		expected Validator
	}{
		{Validator{testSpec, testBundle, true, runtime.GOOS}, Validator{testSpec, testBundle, true, runtime.GOOS}},
		{Validator{testSpec, testBundle, false, testPlatform}, Validator{testSpec, testBundle, false, testPlatform}},
	}

	for _, c := range cases {
		v, err := NewValidator(c.val.spec, c.val.bundlePath, c.val.HostSpecific, c.val.platform)
		if err != nil {
			t.Errorf("unexpected NewValidator error: %+v", err)
		}
		assert.Equal(t, c.expected, v)
	}
}

func TestJSONSchema(t *testing.T) {
	for _, tt := range []struct {
		config *rspec.Spec
		error  string
	}{
		{
			config: &rspec.Spec{},
			error:  "1 error occurred:\n\n* Version string empty\nRefer to: https://github.com/opencontainers/runtime-spec/blob/v1.0.0/config.md#specification-version",
		},
		{
			config: &rspec.Spec{
				Version: "1.0.1-rc1",
			},
			error: "Could not read schema from HTTP, response status is 404 Not Found",
		},
		{
			config: &rspec.Spec{
				Version: "1.0.0",
			},
			error: "",
		},
		{
			config: &rspec.Spec{
				Version: "1.0.0",
				Process: &rspec.Process{},
			},
			error: "process.args: Invalid type. Expected: array, given: null",
		},
		{
			config: &rspec.Spec{
				Version: "1.0.0",
				Linux:   &rspec.Linux{},
			},
			error: "",
		},
		{
			config: &rspec.Spec{
				Version: "1.0.0",
				Linux: &rspec.Linux{
					RootfsPropagation: "",
				},
			},
			error: "",
		},
		{
			config: &rspec.Spec{
				Version: "1.0.0",
				Linux: &rspec.Linux{
					RootfsPropagation: "shared",
				},
			},
			error: "",
		},
		{
			config: &rspec.Spec{
				Version: "1.0.0",
				Linux: &rspec.Linux{
					RootfsPropagation: "rshared",
				},
			},
			error: "linux.rootfsPropagation: linux.rootfsPropagation must be one of the following: \"private\", \"shared\", \"slave\", \"unbindable\"",
		},
		{
			config: &rspec.Spec{
				Version: "1.0.0-rc5",
			},
			error: "process: process is required",
		},
		{
			config: &rspec.Spec{
				Version: "1.0.0",
				Linux: &rspec.Linux{
					Namespaces: []rspec.LinuxNamespace{
						{
							Type: "pid",
						},
					},
				},
			},
			error: "",
		},
		{
			config: &rspec.Spec{
				Version: "1.0.0",
				Linux: &rspec.Linux{
					Namespaces: []rspec.LinuxNamespace{
						{
							Type: "test",
						},
					},
				},
			},
			error: "2 errors occurred:\n\n* linux.namespaces.0: Must validate at least one schema (anyOf)\n* linux.namespaces.0.type: linux.namespaces.0.type must be one of the following: \"mount\", \"pid\", \"network\", \"uts\", \"ipc\", \"user\", \"cgroup\"",
		},
		{
			config: &rspec.Spec{
				Version: "1.0.0",
				Linux: &rspec.Linux{
					Seccomp: &rspec.LinuxSeccomp{
						DefaultAction: "SCMP_ACT_ALLOW",
						Architectures: []rspec.Arch{
							"SCMP_ARCH_X86",
							"SCMP_ARCH_X32",
						},
					},
				},
			},
			error: "",
		},
		{
			config: &rspec.Spec{
				Version: "1.0.0",
				Linux: &rspec.Linux{
					Seccomp: &rspec.LinuxSeccomp{
						DefaultAction: "SCMP_ACT_ALLOW",
						Architectures: []rspec.Arch{
							"SCMP_ARCH_X86",
							"SCMP_ARCH",
						},
					},
				},
			},
			error: "linux.seccomp.architectures.1: linux.seccomp.architectures.1 must be one of the following: \"SCMP_ARCH_X86\", \"SCMP_ARCH_X86_64\", \"SCMP_ARCH_X32\", \"SCMP_ARCH_ARM\", \"SCMP_ARCH_AARCH64\", \"SCMP_ARCH_MIPS\", \"SCMP_ARCH_MIPS64\", \"SCMP_ARCH_MIPS64N32\", \"SCMP_ARCH_MIPSEL\", \"SCMP_ARCH_MIPSEL64\", \"SCMP_ARCH_MIPSEL64N32\", \"SCMP_ARCH_PPC\", \"SCMP_ARCH_PPC64\", \"SCMP_ARCH_PPC64LE\", \"SCMP_ARCH_S390\", \"SCMP_ARCH_S390X\", \"SCMP_ARCH_PARISC\", \"SCMP_ARCH_PARISC64\"",
		},
		{
			config: &rspec.Spec{
				Version: "1.0.0",
				Linux: &rspec.Linux{
					Seccomp: &rspec.LinuxSeccomp{
						DefaultAction: "SCMP_ACT_ALLOW",
						Syscalls: []rspec.LinuxSyscall{
							{
								Names:  []string{"getcwd"},
								Action: "SCMP_ACT_KILL",
							},
						},
					},
				},
			},
			error: "",
		},
		{
			config: &rspec.Spec{
				Version: "1.0.0",
				Linux: &rspec.Linux{
					Seccomp: &rspec.LinuxSeccomp{
						DefaultAction: "SCMP_ACT_ALLOW",
						Syscalls: []rspec.LinuxSyscall{
							{
								Names:  []string{"getcwd"},
								Action: "SCMP_ACT",
							},
						},
					},
				},
			},
			error: "linux.seccomp.syscalls.0.action: linux.seccomp.syscalls.0.action must be one of the following: \"SCMP_ACT_KILL\", \"SCMP_ACT_TRAP\", \"SCMP_ACT_ERRNO\", \"SCMP_ACT_TRACE\", \"SCMP_ACT_ALLOW\"",
		},
		{
			config: &rspec.Spec{
				Version: "1.0.0",
				Linux: &rspec.Linux{
					Seccomp: &rspec.LinuxSeccomp{
						DefaultAction: "SCMP_ACT_ALLOW",
						Syscalls: []rspec.LinuxSyscall{
							{
								Names:  []string{"getcwd"},
								Action: "SCMP_ACT_KILL",
								Args: []rspec.LinuxSeccompArg{
									{
										Index: 0,
										Value: 2080,
										Op:    "SCMP_CMP_NE",
									},
								},
							},
						},
					},
				},
			},
			error: "",
		},
		{
			config: &rspec.Spec{
				Version: "1.0.0",
				Linux: &rspec.Linux{
					Seccomp: &rspec.LinuxSeccomp{
						DefaultAction: "SCMP_ACT_ALLOW",
						Syscalls: []rspec.LinuxSyscall{
							{
								Names:  []string{"getcwd"},
								Action: "SCMP_ACT_KILL",
								Args: []rspec.LinuxSeccompArg{
									{
										Index: 0,
										Value: 2080,
										Op:    "SCMP_CMP",
									},
								},
							},
						},
					},
				},
			},
			error: "linux.seccomp.syscalls.0.args.0.op: linux.seccomp.syscalls.0.args.0.op must be one of the following: \"SCMP_CMP_NE\", \"SCMP_CMP_LT\", \"SCMP_CMP_LE\", \"SCMP_CMP_EQ\", \"SCMP_CMP_GE\", \"SCMP_CMP_GT\", \"SCMP_CMP_MASKED_EQ\"",
		},
	} {
		t.Run(tt.error, func(t *testing.T) {
			v := &Validator{spec: tt.config}
			errs := v.CheckJSONSchema()
			if tt.error == "" {
				if errs == nil {
					return
				}
				t.Fatalf("expected no error, but got: %s", errs.Error())
			}
			if errs == nil {
				t.Fatal("failed to raise the expected error")
			}
			merr, ok := errs.(*multierror.Error)
			if !ok {
				t.Fatalf("non-multierror returned by CheckJSONSchema: %s", errs.Error())
			}
			for _, err := range merr.Errors {
				if err.Error() == tt.error {
					return
				}
			}
			assert.Equal(t, tt.error, errs.Error())
		})
	}
}

func TestCheckRoot(t *testing.T) {
	tmpBundle, err := ioutil.TempDir("", "oci-check-rootfspath")
	if err != nil {
		t.Fatalf("Failed to create a TempDir in 'CheckRoot'")
	}
	defer os.RemoveAll(tmpBundle)

	rootfsDir := "rootfs/rootfs"
	rootfsNonDir := "rootfsfile"
	rootfsNonExists := "rootfsnil"
	if err := os.MkdirAll(filepath.Join(tmpBundle, rootfsDir), 0700); err != nil {
		t.Fatalf("Failed to create a rootfs directory in 'CheckRoot'")
	}
	if _, err := os.Create(filepath.Join(tmpBundle, rootfsNonDir)); err != nil {
		t.Fatalf("Failed to create a non-directory rootfs in 'CheckRoot'")
	}

	// Note: Abs error is not tested
	cases := []struct {
		val      rspec.Spec
		platform string
		expected specerror.Code
	}{
		{rspec.Spec{Windows: &rspec.Windows{HyperV: &rspec.WindowsHyperV{}}, Root: &rspec.Root{}}, "windows", specerror.RootOnHyperVNotSet},
		{rspec.Spec{Windows: &rspec.Windows{HyperV: &rspec.WindowsHyperV{}}, Root: nil}, "windows", specerror.NonError},
		{rspec.Spec{Windows: &rspec.Windows{}, Root: &rspec.Root{Path: filepath.Join(tmpBundle, "rootfs")}}, "windows", specerror.RootPathOnWindowsGUID},
		{rspec.Spec{Windows: &rspec.Windows{}, Root: &rspec.Root{Path: "\\\\?\\Volume{ec84d99e-3f02-11e7-ac6c-00155d7682cf}\\"}}, "windows", specerror.NonError},
		{rspec.Spec{Windows: &rspec.Windows{}, Root: nil}, "windows", specerror.RootOnWindowsRequired},
		{rspec.Spec{Root: nil}, "linux", specerror.RootOnNonWindowsRequired},
		{rspec.Spec{Root: &rspec.Root{Path: "maverick-rootfs"}}, "linux", specerror.RootPathOnPosixConvention},
		{rspec.Spec{Root: &rspec.Root{Path: "rootfs"}}, "linux", specerror.NonError},
		{rspec.Spec{Root: &rspec.Root{Path: filepath.Join(tmpBundle, rootfsNonExists)}}, "linux", specerror.RootPathExist},
		{rspec.Spec{Root: &rspec.Root{Path: filepath.Join(tmpBundle, rootfsNonDir)}}, "linux", specerror.RootPathExist},
		{rspec.Spec{Root: &rspec.Root{Path: filepath.Join(tmpBundle, "rootfs")}}, "linux", specerror.NonError},
		{rspec.Spec{Root: &rspec.Root{Path: "rootfs/rootfs"}}, "linux", specerror.ArtifactsInSingleDir},
		{rspec.Spec{Root: &rspec.Root{Readonly: true}}, "windows", specerror.RootReadonlyOnWindowsFalse},
	}
	for _, c := range cases {
		v, err := NewValidator(&c.val, tmpBundle, false, c.platform)
		if err != nil {
			t.Errorf("unexpected NewValidator error: %+v", err)
		}
		err = v.CheckRoot()
		assert.Equal(t, c.expected, specerror.FindError(err, c.expected), fmt.Sprintf("Fail to check Root: %v %d", err, c.expected))
	}
}

func TestCheckSemVer(t *testing.T) {
	cases := []struct {
		val      string
		expected specerror.Code
	}{
		{rspec.Version, specerror.NonError},
		//FIXME: validate currently only handles rpsec.Version
		{"0.0.1", specerror.NonRFCError},
		{"invalid", specerror.SpecVersionInSemVer},
	}

	for _, c := range cases {
		v, err := NewValidator(&rspec.Spec{Version: c.val}, "", false, "linux")
		if err != nil {
			t.Errorf("unexpected NewValidator error: %+v", err)
		}
		err = v.CheckSemVer()
		assert.Equal(t, c.expected, specerror.FindError(err, c.expected), "Fail to check SemVer "+c.val)
	}
}

func TestCheckProcess(t *testing.T) {
	cases := []struct {
		val      rspec.Spec
		platform string
		expected specerror.Code
	}{
		{
			val: rspec.Spec{
				Version: "1.0.0",
				Process: &rspec.Process{
					Args: []string{"sh"},
					Cwd:  "/",
				},
			},
			platform: "linux",
			expected: specerror.NonError,
		},
		{
			val: rspec.Spec{
				Version: "1.0.0",
				Process: &rspec.Process{
					Args: []string{"sh"},
					Cwd:  "/",
				},
			},
			platform: "windows",
			expected: specerror.ProcCwdAbs,
		},
		{
			val: rspec.Spec{
				Version: "1.0.0",
				Process: &rspec.Process{
					Args: []string{"sh"},
					Cwd:  "c:\\foo",
				},
			},
			platform: "linux",
			expected: specerror.ProcCwdAbs,
		},
		{
			val: rspec.Spec{
				Version: "1.0.0",
				Process: &rspec.Process{
					Args: []string{"sh"},
					Cwd:  "c:\\foo",
				},
			},
			platform: "windows",
			expected: specerror.NonError,
		},
		{
			val: rspec.Spec{
				Version: "1.0.0",
				Process: &rspec.Process{
					Cwd: "/",
				},
			},
			platform: "linux",
			expected: specerror.ProcArgsOneEntryRequired,
		},
		{
			val: rspec.Spec{
				Version: "1.0.0",
				Process: &rspec.Process{
					Args: []string{"sh"},
					Cwd:  "/",
					Rlimits: []rspec.POSIXRlimit{
						{
							Type: "RLIMIT_NOFILE",
							Hard: 1024,
							Soft: 1024,
						},
						{
							Type: "RLIMIT_NPROC",
							Hard: 512,
							Soft: 512,
						},
					},
				},
			},
			platform: "linux",
			expected: specerror.NonError,
		},
		{
			val: rspec.Spec{
				Version: "1.0.0",
				Process: &rspec.Process{
					Args: []string{"sh"},
					Cwd:  "/",
					Rlimits: []rspec.POSIXRlimit{
						{
							Type: "RLIMIT_NOFILE",
							Hard: 1024,
							Soft: 1024,
						},
					},
				},
			},
			platform: "solaris",
			expected: specerror.NonError,
		},
		{
			val: rspec.Spec{
				Version: "1.0.0",
				Process: &rspec.Process{
					Args: []string{"sh"},
					Cwd:  "/",
					Rlimits: []rspec.POSIXRlimit{
						{
							Type: "RLIMIT_DOES_NOT_EXIST",
							Hard: 512,
							Soft: 512,
						},
						{
							Type: "RLIMIT_DOES_NOT_EXIST",
							Hard: 512,
							Soft: 512,
						},
					},
				},
			},
			platform: "linux",
			expected: specerror.PosixProcRlimitsErrorOnDup,
		},
		{
			val: rspec.Spec{
				Version: "1.0.0",
				Process: &rspec.Process{
					Args: []string{"sh"},
					Cwd:  "/",
					Rlimits: []rspec.POSIXRlimit{
						{
							Type: "RLIMIT_DOES_NOT_EXIST",
							Hard: 512,
							Soft: 512,
						},
					},
				},
			},
			platform: "linux",
			expected: specerror.PosixProcRlimitsTypeValueError,
		},
		{
			val: rspec.Spec{
				Version: "1.0.0",
				Process: &rspec.Process{
					Args: []string{"sh"},
					Cwd:  "/",
					Rlimits: []rspec.POSIXRlimit{
						{
							Type: "RLIMIT_NPROC",
							Hard: 512,
							Soft: 512,
						},
					},
				},
			},
			platform: "solaris",
			expected: specerror.PosixProcRlimitsTypeValueError,
		},
	}
	for _, c := range cases {
		v, err := NewValidator(&c.val, ".", false, c.platform)
		if err != nil {
			t.Errorf("unexpected NewValidator error: %+v", err)
		}
		err = v.CheckProcess()
		assert.Equal(t, c.expected, specerror.FindError(err, c.expected), fmt.Sprintf("failed CheckProcess: %v %d", err, c.expected))
	}
}

func TestCheckLinux(t *testing.T) {
	weightDevices := []rspec.LinuxWeightDevice{
		rspec.LinuxWeightDevice{},
	}
	weightDevices[0].Major = 5
	weightDevices[0].Minor = 0

	cases := []struct {
		val      rspec.Spec
		expected specerror.Code
	}{
		{
			val: rspec.Spec{
				Version: "1.0.0",
				Linux: &rspec.Linux{
					Namespaces: []rspec.LinuxNamespace{
						{
							Type: "pid",
							Path: "/proc/test",
						},
						{
							Type: "network",
						},
					},
				},
			},
			expected: specerror.NonError,
		},
		{
			val: rspec.Spec{
				Version: "1.0.0",
				Linux: &rspec.Linux{
					Namespaces: []rspec.LinuxNamespace{
						{
							Type: "pid",
							Path: "proc",
						},
					},
				},
			},
			expected: specerror.NSPathAbs,
		},
		{
			val: rspec.Spec{
				Version: "1.0.0",
				Linux: &rspec.Linux{
					Namespaces: []rspec.LinuxNamespace{
						{
							Type: "pid",
						},
						{
							Type: "pid",
						},
					},
				},
			},
			expected: specerror.NSErrorOnDup,
		},
		{
			val: rspec.Spec{
				Version: "1.0.0",
				Linux: &rspec.Linux{
					MaskedPaths: []string{"/proc/kcore"},
				},
			},
			expected: specerror.NonError,
		},
		{
			val: rspec.Spec{
				Version: "1.0.0",
				Linux: &rspec.Linux{
					MaskedPaths: []string{"proc"},
				},
			},
			expected: specerror.MaskedPathsAbs,
		},
		{
			val: rspec.Spec{
				Version: "1.0.0",
				Linux: &rspec.Linux{
					ReadonlyPaths: []string{"/proc/sys"},
				},
			},
			expected: specerror.NonError,
		},
		{
			val: rspec.Spec{
				Version: "1.0.0",
				Linux: &rspec.Linux{
					ReadonlyPaths: []string{"proc"},
				},
			},
			expected: specerror.ReadonlyPathsAbs,
		},
		{
			val: rspec.Spec{
				Version: "1.0.0",
				Linux: &rspec.Linux{
					Resources: &rspec.LinuxResources{
						BlockIO: &rspec.LinuxBlockIO{
							WeightDevice: weightDevices,
						},
					},
				},
			},
			expected: specerror.BlkIOWeightOrLeafWeightExist,
		},
	}
	for _, c := range cases {
		v, err := NewValidator(&c.val, ".", false, "linux")
		if err != nil {
			t.Errorf("unexpected NewValidator error: %+v", err)
		}
		err = v.CheckLinux()
		assert.Equal(t, c.expected, specerror.FindError(err, c.expected), fmt.Sprintf("failed CheckLinux: %v %d", err, c.expected))
	}
}

func TestCheckPlatform(t *testing.T) {
	cases := []struct {
		val      rspec.Spec
		platform string
		expected specerror.Code
	}{
		{
			val: rspec.Spec{
				Version: "1.0.0",
			},
			platform: "linux",
			expected: specerror.NonError,
		},
		{
			val: rspec.Spec{
				Version: "1.0.0",
			},
			platform: "windows",
			expected: specerror.PlatformSpecConfOnWindowsSet,
		},
		{
			val: rspec.Spec{
				Version: "1.0.0",
				Windows: &rspec.Windows{
					LayerFolders: []string{"C:\\Layers\\layer1"},
				},
			},
			platform: "windows",
			expected: specerror.NonError,
		},
	}
	for _, c := range cases {
		v, err := NewValidator(&c.val, ".", false, c.platform)
		if err != nil {
			t.Errorf("unexpected NewValidator error: %+v", err)
		}
		err = v.CheckPlatform()
		assert.Equal(t, c.expected, specerror.FindError(err, c.expected), fmt.Sprintf("failed CheckPlatform: %v %d", err, c.expected))
	}
}

func TestCheckHooks(t *testing.T) {
	cases := []struct {
		val      rspec.Spec
		expected specerror.Code
	}{
		{
			val: rspec.Spec{
				Version: "1.0.0",
				Hooks: &rspec.Hooks{
					Prestart: []rspec.Hook{
						{
							Path: "/usr/bin/fix-mounts",
						},
					},
				},
			},
			expected: specerror.NonError,
		},
		{
			val: rspec.Spec{
				Version: "1.0.0",
				Hooks: &rspec.Hooks{
					Prestart: []rspec.Hook{
						{
							Path: "usr",
						},
					},
				},
			},
			expected: specerror.PosixHooksPathAbs,
		},
	}
	for _, c := range cases {
		v, err := NewValidator(&c.val, ".", false, "linux")
		if err != nil {
			t.Errorf("unexpected NewValidator error: %+v", err)
		}
		err = v.CheckHooks()
		assert.Equal(t, c.expected, specerror.FindError(err, c.expected), fmt.Sprintf("failed CheckHooks: %v %d", err, c.expected))
	}
}

func TestCheckMandatoryFields(t *testing.T) {
	for _, tt := range []struct {
		config *rspec.Spec
		error  string
	}{
		{
			config: &rspec.Spec{},
			error:  "1 error occurred:\n\n* 'Spec.Version' should not be empty",
		},
		{
			config: nil,
			error:  "1 error occurred:\n\n* Spec can't be nil",
		},
		{
			config: &rspec.Spec{
				Version: "1.0.0",
			},
			error: "",
		},
		{
			config: &rspec.Spec{
				Version: "1.0.0",
				Root:    &rspec.Root{},
			},
			error: "1 error occurred:\n\n* 'Root.Path' should not be empty",
		},
	} {
		t.Run(tt.error, func(t *testing.T) {
			var errs *multierror.Error
			v := &Validator{spec: tt.config}
			errs = multierror.Append(errs, v.CheckMandatoryFields())
			if tt.error == "" {
				if errs.ErrorOrNil() == nil {
					return
				}
				t.Fatalf("expected no error, but got: %s", errs.Error())
			}
			if errs.ErrorOrNil() == nil {
				t.Fatal("failed to raise the expected error")
			}

			for _, err := range errs.Errors {
				if err.Error() == tt.error {
					return
				}
			}
			assert.Equal(t, tt.error, errs.Error())
		})
	}
}

func TestCheckAnnotations(t *testing.T) {
	cases := []struct {
		val      rspec.Spec
		expected specerror.Code
	}{
		{
			val:      rspec.Spec{},
			expected: specerror.NonError,
		},
		{
			val: rspec.Spec{
				Annotations: map[string]string{},
			},
			expected: specerror.NonError,
		},
		{
			val: rspec.Spec{
				Annotations: map[string]string{"invalid": ""},
			},
			expected: specerror.AnnotationsKeyReversedDomain,
		},
		{
			val: rspec.Spec{
				Annotations: map[string]string{"org.opencontainers.oci": ""},
			},
			expected: specerror.AnnotationsKeyReservedNS,
		},
		{
			val: rspec.Spec{
				Annotations: map[string]string{"com.example": ""},
			},
			expected: specerror.NonError,
		},
	}
	for _, c := range cases {
		v, err := NewValidator(&c.val, ".", false, "")
		if err != nil {
			t.Errorf("unexpected NewValidator error: %+v", err)
		}
		err = v.CheckAnnotations()
		assert.Equal(t, c.expected, specerror.FindError(err, c.expected), fmt.Sprintf("failed CheckAnnotations: %v %d", err, c.expected))
	}
}

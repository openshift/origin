package runner

import (
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var timeout = flag.Duration("sub.timeout", 0, "Specify the timeout for each sub test")

func TestIntegration(t *testing.T) {
	executeTests(t, "..", "github.com/openshift/origin/test/integration", 1)
}

func testsForPackage(t *testing.T, dir, packageName string) []string {
	c := build.Default
	p, err := c.ImportDir(dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	fset := token.NewFileSet()
	for _, testFile := range p.TestGoFiles {
		p, err := parser.ParseFile(fset, filepath.Join("..", testFile), nil, parser.DeclarationErrors|parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range p.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name == nil || !strings.HasPrefix(d.Name.Name, "Test") || len(d.Name.Name) <= 4 {
					continue
				}
				if len(d.Type.Params.List) != 1 || len(d.Type.Params.List[0].Names) != 1 {
					continue
				}
				switch expr := d.Type.Params.List[0].Type.(type) {
				case *ast.StarExpr:
					sexpr, ok := expr.X.(*ast.SelectorExpr)
					if !ok {
						continue
					}
					if sexpr.Sel.Name != "T" || sexpr.X.(*ast.Ident).Name != "testing" {
						continue
					}
					names = append(names, d.Name.Name)
				default:
				}
			default:
			}
		}
	}
	sort.Strings(names)
	return names
}

func executeTests(t *testing.T, dir, packageName string, maxRetries int) {
	binaryName := path.Base(packageName) + ".test"

	names := testsForPackage(t, dir, packageName)

	var binaryPath string
	if path, err := exec.LookPath(binaryName); err == nil {
		// use the compiled binary on the test
		if testing.Verbose() {
			t.Logf("using existing binary")
		}
		binaryPath = path
	} else {
		// compile the test
		if testing.Verbose() {
			t.Logf("compiling %s", packageName)
		}
		cmd := exec.Command("go", "test", packageName, "-i", "-c", binaryName)
		if testing.Verbose() {
			cmd.Args = append(cmd.Args, "-test.v")
		}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatal(string(out))
		}
		binaryPath = "." + string(filepath.Separator) + binaryName
	}

	// run all the nested tests
	for _, s := range names {
		name := s
		t.Run(name, func(t *testing.T) {
			if t.Skipped() {
				return
			}
			t.Parallel()

			retry := maxRetries
			for {
				err := runSingleTest(t, dir, binaryPath, name)
				if err == nil {
					if retry != maxRetries {
						// signal that the test was abnormal if we got at least one flake
						t.Skipf("FAILED %s %d times, skipping:\n%v", name, maxRetries+1, err)
					}
					break
				}
				if retry == 0 {
					t.Error(err)
					break
				}
				retry--
				t.Logf("FAILED %s, retrying:\n%v", name, err)
			}
		})
	}
}

func runSingleTest(t *testing.T, dir, binaryPath, name string) error {
	env := os.Environ()

	// create a base temporary directory for config and temporary output that will be cleaned up
	// after the test
	testDir, err := ioutil.TempDir("", "tmp-"+strings.ToLower(name))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { os.RemoveAll(testDir) }()
	env = append(without(env, "BASETMPDIR="), fmt.Sprintf("BASETMPDIR=%s", testDir))
	env = append(without(env, "TMPDIR="), fmt.Sprintf("TMPDIR=%s", testDir))

	// ETCD_TEST_DIR allows tests put etcd on fast storage, like a ramdisk.
	if etcdDir := os.Getenv("ETCD_TEST_DIR"); len(etcdDir) > 0 {
		etcdTestDir, err := ioutil.TempDir(etcdDir, "tmp-"+strings.ToLower(name))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { os.RemoveAll(etcdTestDir) }()
		env = append(without(env, "ETCD_TEST_DIR="), fmt.Sprintf("ETCD_TEST_DIR=%s", etcdTestDir))
	}

	cmd := exec.Command(
		binaryPath,
		"-test.run", "^"+regexp.QuoteMeta(name)+"$",
		"-test.v",
	)
	cmd.Dir = dir
	cmd.Env = env
	if testing.Short() {
		cmd.Args = append(cmd.Args, "-test.short")
	}
	cmd.Args = append(cmd.Args, "-test.timeout", (*timeout).String())

	out, err := cmd.CombinedOutput()
	if err != nil {
		if len(out) != 0 {
			return fmt.Errorf(string(out))
		}
		return err
	}

	if testing.Verbose() {
		t.Log(string(out))
	}
	return nil
}

func without(all []string, value string) []string {
	var result []string
	for i := 0; i < len(all); i++ {
		if !strings.HasPrefix(all[i], value) {
			result = append(result, all[i])
		}
	}
	return result
}

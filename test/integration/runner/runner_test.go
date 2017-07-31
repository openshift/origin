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
	dir := ".."
	packageName := "github.com/openshift/origin/test/integration"
	binaryName := path.Base(packageName) + ".test"

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
		dir, err := ioutil.TempDir("", "openshift-integration")
		if err != nil {
			t.Fatal(err)
		}
		name := s
		cmd := exec.Command(
			binaryPath,
			"-test.run", regexp.QuoteMeta(name),
			"-test.v",
		)
		cmd.Env = append(os.Environ(), fmt.Sprintf("BASETMPDIR=%s", dir))
		if testing.Short() {
			cmd.Args = append(cmd.Args, "-test.short")
		}
		if *timeout != 0 {
			cmd.Args = append(cmd.Args, "-test.timeout", (*timeout).String())
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				os.RemoveAll(dir)
			}()
			out, err := cmd.CombinedOutput()
			if err != nil {
				if len(out) != 0 {
					t.Error(string(out))
				} else {
					t.Error(err)
				}
				return
			}
			if testing.Verbose() {
				t.Log(string(out))
			}
		})
	}
}

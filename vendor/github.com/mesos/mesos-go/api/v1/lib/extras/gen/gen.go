// +build ignore

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"text/template"
)

type (
	Type struct {
		Notation  string
		Spec      string
		Prototype string
	}

	TypeMap map[string]Type

	Imports []string

	Config struct {
		Package string
		Imports Imports
		Args    string // arguments that we were invoked with; TODO(jdef) rename this to Flags?
		Types   TypeMap
	}
)

func (i *Imports) String() string {
	if i == nil {
		return ""
	}
	return fmt.Sprintf("%#v", ([]string)(*i))
}

func (i *Imports) Set(s string) error {
	*i = append(*i, s)
	return nil
}

func (tm *TypeMap) Set(s string) error {
	tok := strings.SplitN(s, ":", 3)

	if len(tok) < 2 {
		return errors.New("expected {notation}:{type-spec} syntax, instead of " + s)
	}

	if *tm == nil {
		*tm = make(TypeMap)
	}

	t := (*tm)[tok[0]]
	t.Notation, t.Spec = tok[0], tok[1]

	if t.Notation == "" {
		return fmt.Errorf("type notation in %q may not be an empty string", s)
	}

	if t.Spec == "" {
		return fmt.Errorf("type specification in %q may not be an empty string", s)
	}

	if len(tok) == 3 {
		t.Prototype = tok[2]

		if t.Prototype == "" {
			return fmt.Errorf("prototype specification in %q may not be an empty string", s)
		}
	}

	(*tm)[tok[0]] = t
	return nil
}

func (tm *TypeMap) String() string {
	if tm == nil {
		return ""
	}
	return fmt.Sprintf("%#v", *tm)
}

func (c *Config) Var(notation string, names ...string) string {
	t := c.Type(notation)
	if t == "" || len(names) == 0 {
		return ""
	}
	return "var " + strings.Join(names, ",") + " " + t
}

func (c *Config) Arg(notation, name string) string {
	t := c.Type(notation)
	if t == "" {
		return ""
	}
	if name == "" {
		return t
	}
	if strings.HasSuffix(name, ",") {
		return strings.TrimSpace(name[:len(name)-1]+" "+t) + ", "
	}
	return name + " " + t
}

func (c *Config) Ref(notation, name string) (string, error) {
	t := c.Type(notation)
	if t == "" || name == "" {
		return "", nil
	}
	if strings.HasSuffix(name, ",") {
		if len(name) < 2 {
			return "", errors.New("expected ref name before comma")
		}
		return name[:len(name)-1] + ", ", nil
	}
	return name, nil
}

func (c *Config) RequireType(notation string) (string, error) {
	_, ok := c.Types[notation]
	if !ok {
		return "", fmt.Errorf("type %q is required but not specified", notation)
	}
	return "", nil
}

func (c *Config) RequirePrototype(notation string) (string, error) {
	t, ok := c.Types[notation]
	if !ok {
		// needed for optional types: don't require the prototype if the optional type is not defined
		return "", nil
	}
	if t.Prototype == "" {
		return "", fmt.Errorf("prototype for type %q is required but not specified", notation)
	}
	return "", nil
}

func (c *Config) Type(notation string) string {
	t, ok := c.Types[notation]
	if !ok {
		return ""
	}
	return t.Spec
}

func (c *Config) Prototype(notation string) string {
	t, ok := c.Types[notation]
	if !ok {
		return ""
	}
	return t.Prototype
}

func (c *Config) AddFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.Package, "package", c.Package, "destination package")
	fs.Var(&c.Imports, "import", "packages to import")
	fs.Var(&c.Types, "type", "auxilliary type mappings in {notation}:{type-spec}:{prototype-expr} format")
}

func NewConfig() *Config {
	var (
		c = Config{
			Package: os.Getenv("GOPACKAGE"),
		}
	)
	return &c
}

func Run(src, test *template.Template, args ...string) {
	if len(args) < 1 {
		panic(errors.New("expected at least one arg"))
	}
	var (
		c             = NewConfig()
		defaultOutput = "foo.go"
		output        string
	)
	if c.Package != "" {
		defaultOutput = c.Package + "_generated.go"
	}

	fs := flag.NewFlagSet(args[0], flag.PanicOnError)
	fs.StringVar(&output, "output", output, "path of the to-be-generated file")
	c.AddFlags(fs)

	if err := fs.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			fs.PrintDefaults()
		}
		panic(err)
	}

	c.Args = strings.Join(args[1:], " ")

	if c.Package == "" {
		c.Package = "foo"
	}

	if output == "" {
		output = defaultOutput
	}

	genmap := make(map[string]*template.Template)
	if src != nil {
		genmap[output] = src
	}
	if test != nil {
		testOutput := output + "_test"
		if strings.HasSuffix(output, ".go") {
			testOutput = output[:len(output)-3] + "_test.go"
		}
		genmap[testOutput] = test
	}
	if len(genmap) == 0 {
		panic(errors.New("neither src or test templates were provided"))
	}

	Generate(genmap, c, func(err error) { panic(err) })
}

func Generate(items map[string]*template.Template, data interface{}, eh func(error)) {
	for filename, t := range items {
		func() {
			f, err := os.Create(filename)
			if err != nil {
				eh(err)
				return
			}
			closer := safeClose(f)
			defer closer()

			log.Println("generating file", filename)
			err = t.Execute(f, data)
			if err != nil {
				eh(err)
			}
			err = f.Sync()
			if err != nil {
				eh(err)
			}
			err = closer()
			if err != nil {
				eh(err)
			}
		}()
	}
}

func safeClose(c io.Closer) func() error {
	var s bool
	return func() error {
		if s {
			return nil
		}
		s = true
		return c.Close()
	}
}
